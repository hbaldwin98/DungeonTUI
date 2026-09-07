package domain

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSessionReconciliationLeavesTranscriptIntact(t *testing.T) {
	ended := time.Now().UTC()
	session := SessionRecord{
		ID:      "session-1",
		Title:   "Session 1",
		EndedAt: &ended,
		Entries: []TranscriptEntry{
			{
				ID:    "entry-1",
				Text:  "@Captain Vale finds a clue",
				Links: []EntityLink{{Text: "Captain Vale", RecordID: "npc-captain-vale"}},
			},
			{ID: "entry-2", Text: "The party rests"},
		},
	}
	original := append([]TranscriptEntry(nil), session.Entries...)
	records := []Record{
		{ID: "npc-captain-vale", Type: NPC, Title: "Captain Vale", Authority: Canon},
		{ID: "draft-key", Type: Item, Title: "Iron Key", Authority: Draft, Source: "Session 1"},
	}

	recon := BuildSessionReconciliation(session, records)
	if recon.SessionID != session.ID {
		t.Fatalf("session id=%q", recon.SessionID)
	}
	if len(recon.Items) < 2 {
		t.Fatalf("expected review items, got %#v", recon.Items)
	}
	for index, entry := range session.Entries {
		if entry.Text != original[index].Text || entry.ID != original[index].ID {
			t.Fatal("building reconciliation must not mutate transcript entries")
		}
	}

	var promote *ReconciliationItem
	for index := range recon.Items {
		if recon.Items[index].Kind == ReconPromoteDraft && recon.Items[index].RecordID == "draft-key" {
			promote = &recon.Items[index]
			break
		}
	}
	if promote == nil {
		t.Fatal("expected draft promotion item for session-created draft")
	}
	updatedItem, updatedRecords, err := ApproveItem(*promote, records)
	if err != nil {
		t.Fatal(err)
	}
	if updatedItem.Status != ReconApproved {
		t.Fatalf("status=%q", updatedItem.Status)
	}
	found := false
	for _, record := range updatedRecords {
		if record.ID == "draft-key" {
			found = true
			if record.Authority != Canon {
				t.Fatalf("expected canon after approve, got %q", record.Authority)
			}
		}
	}
	if !found {
		t.Fatal("draft missing after approve")
	}
	for index, entry := range session.Entries {
		if entry.Text != original[index].Text {
			t.Fatal("approve must not mutate transcript")
		}
	}
}

func TestApproveTranscriptNoteCreatesSourcedCanonNote(t *testing.T) {
	ended := time.Now().UTC()
	session := SessionRecord{
		ID: "session-1", Title: "Session 1", Scope: testReconScope, EndedAt: &ended,
		Entries: []TranscriptEntry{{ID: "entry-2", Text: "The party rests in Greywatch."}},
	}
	records := []Record{{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Authority: Canon, Scope: testReconScope}}
	originalBody := records[0].Body
	originalEntries := append([]TranscriptEntry(nil), session.Entries...)

	recon := BuildSessionReconciliation(session, records)
	note := findItem(t, recon, ReconTranscriptNote)
	if strings.TrimSpace(note.Mutation.Text) == "" {
		t.Fatal("expected editable mutation text on the transcript note")
	}

	updated, records, err := ApplyReconItem(note, records, session)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != ReconApproved {
		t.Fatalf("status=%q", updated.Status)
	}
	created := findRecordByID(t, records, "note-entry-2")
	if created.Type != Note || created.Authority != Canon {
		t.Fatalf("expected canon note, got %#v", created)
	}
	if created.Body != "The party rests in Greywatch." {
		t.Fatalf("body=%q", created.Body)
	}
	if created.Source != "Session 1" {
		t.Fatalf("source=%q", created.Source)
	}
	if created.Scope != testReconScope {
		t.Fatalf("scope=%#v", created.Scope)
	}
	if records[0].Body != originalBody {
		t.Fatal("unrelated records must stay intact")
	}
	for i, entry := range session.Entries {
		if entry.Text != originalEntries[i].Text {
			t.Fatal("transcript mutated")
		}
	}
}

func TestApproveLinkReviewAppendsSourcedCitation(t *testing.T) {
	ended := time.Now().UTC()
	session := SessionRecord{
		ID: "session-1", Title: "Session 1", EndedAt: &ended,
		Entries: []TranscriptEntry{{
			ID: "entry-1", Text: "@Captain Vale finds a clue",
			Links: []EntityLink{{Text: "Captain Vale", RecordID: "npc-vale"}},
		}},
	}
	records := []Record{{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Authority: Canon, Body: "Captain of the guard."}}
	recon := BuildSessionReconciliation(session, records)
	item := findItem(t, recon, ReconLinkReview)
	item.Mutation.Text = "Saw Vale at the crypt."
	updated, records, err := ApplyReconItem(item, records, session)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != ReconApproved {
		t.Fatalf("status=%q", updated.Status)
	}
	vale := findRecordByID(t, records, "npc-vale")
	if !strings.Contains(vale.Body, "Saw Vale at the crypt.") {
		t.Fatalf("expected edited citation, got %q", vale.Body)
	}
	if !strings.Contains(vale.Body, "dungeon:session=session-1 entry=entry-1") {
		t.Fatalf("expected source marker, got %q", vale.Body)
	}

	_, _, err = ApplyReconItem(updated, records, session)
	if err == nil {
		t.Fatal("expected error re-applying a non-pending item")
	}
	vale = findRecordByID(t, records, "npc-vale")
	if strings.Count(vale.Body, "dungeon:session=session-1 entry=entry-1") != 1 {
		t.Fatalf("citation should stay unique, got %q", vale.Body)
	}
}

func TestApplyReconItemRejectsMissingTarget(t *testing.T) {
	item := ReconciliationItem{
		ID: "x", Kind: ReconLinkReview, Status: ReconPending, RecordID: "missing",
		Mutation: Mutation{Op: MutationCite, RecordID: "missing", Text: "hello"},
	}
	_, _, err := ApplyReconItem(item, nil, SessionRecord{ID: "s", Title: "S"})
	if err == nil {
		t.Fatal("expected missing-record error")
	}
}

func TestApplyNoteRequiresTextAndUsesUniqueIDs(t *testing.T) {
	session := SessionRecord{ID: "s1", Title: "Sit", Scope: testReconScope}
	item := ReconciliationItem{
		Kind: ReconTranscriptNote, Status: ReconPending, EntryID: "e1",
		Mutation: Mutation{Op: MutationNote, Text: "   "},
	}
	if _, _, err := ApplyReconItem(item, nil, session); err == nil {
		t.Fatal("expected empty mutation to fail")
	}
	item.Mutation.Text = "First note"
	_, records, err := ApplyReconItem(item, []Record{{ID: "note-e1", Type: Note, Title: "Existing", Authority: Canon}}, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findRecord(records, "note-e1-2"); !ok {
		t.Fatalf("expected unique note id, got %#v", records)
	}
}

var testReconScope = Scope{WorldID: "w", CampaignID: "c"}

func findItem(t *testing.T, recon ReconciliationRecord, kind ReconciliationKind) ReconciliationItem {
	t.Helper()
	for _, item := range recon.Items {
		if item.Kind == kind {
			return item
		}
	}
	t.Fatalf("missing %s item in %#v", kind, recon.Items)
	return ReconciliationItem{}
}

func findRecordByID(t *testing.T, records []Record, id string) Record {
	t.Helper()
	rec, ok := findRecord(records, id)
	if !ok {
		t.Fatalf("missing record %s in %#v", id, records)
	}
	return rec
}
