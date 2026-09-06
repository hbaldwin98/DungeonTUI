package domain

import (
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
