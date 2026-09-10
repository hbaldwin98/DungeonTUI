package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

type failStore struct{}

func (failStore) Load() (domain.Workspace, error) { return domain.Workspace{}, errors.New("no") }
func (failStore) Save(domain.Workspace) error     { return errors.New("disk full") }

func testWS() domain.Workspace {
	scope := domain.Scope{WorldID: "w", CampaignID: "c"}
	return domain.Workspace{
		Scope: scope,
		Records: []domain.Record{
			{ID: "npc-vale", Type: domain.NPC, Title: "Vale", Authority: domain.Canon, Scope: scope, Body: "Captain."},
		},
	}
}

func TestCommitRollsBackWhenSaveFails(t *testing.T) {
	svc := New(failStore{})
	ws := testWS()
	next, err := svc.Commit(ws, func(current domain.Workspace) (domain.Workspace, error) {
		current.Records[0].Body = "mutated"
		return current, nil
	})
	if err == nil {
		t.Fatal("expected save error")
	}
	if next.Records[0].Body != "Captain." {
		t.Fatalf("caller workspace should stay original, got %q", next.Records[0].Body)
	}
}

func TestEndSessionAndApplyRecon(t *testing.T) {
	ws := testWS()
	ended := time.Now().UTC()
	session := domain.SessionRecord{
		ID: "sit-1", Title: "Sit 1", Scope: ws.Scope, EndedAt: &ended,
		Entries: []domain.TranscriptEntry{{ID: "e1", Text: "They hid the token."}},
	}
	ws, recon, err := EndSession(ws, session)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Sessions) != 1 || len(recon.Items) == 0 {
		t.Fatalf("session=%#v recon=%#v", ws.Sessions, recon)
	}
	noteIdx := -1
	for i, item := range ws.Reconciliations[0].Items {
		if item.Kind == domain.ReconTranscriptNote {
			noteIdx = i
			break
		}
	}
	if noteIdx < 0 {
		t.Fatal("expected transcript note")
	}
	ws, err = ApplyRecon(ws, 0, noteIdx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range ws.Records {
		if record.ID == "note-e1" && record.Authority == domain.Canon && strings.Contains(record.Body, "token") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sourced note, got %#v", ws.Records)
	}
	if ws.Sessions[0].Entries[0].Text != "They hid the token." {
		t.Fatal("transcript mutated")
	}
}

func TestEndSessionPreservesExistingReviewProgress(t *testing.T) {
	ws := testWS()
	ended := time.Now().UTC()
	session := domain.SessionRecord{ID: "sit-1", Title: "Sit 1", Scope: ws.Scope, EndedAt: &ended,
		Entries: []domain.TranscriptEntry{{ID: "e1", Text: "Keep this note."}}}
	ws, recon, err := EndSession(ws, session)
	if err != nil {
		t.Fatal(err)
	}
	ws, err = DeferRecon(ws, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	ws.Reconciliations[0].Items[0].Mutation.Text = "Edited owner text"
	ws, repeated, err := EndSession(ws, session)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.ID != recon.ID || repeated.Items[0].Status != domain.ReconDeferred || repeated.Items[0].Mutation.Text != "Edited owner text" {
		t.Fatalf("repeated end reset review progress: %#v", repeated)
	}
}

func TestDeferReconIsIdempotentAndCanLaterApply(t *testing.T) {
	ws := testWS()
	ws.Sessions = []domain.SessionRecord{{ID: "sit-1", Title: "Sit", Scope: ws.Scope}}
	ws.Reconciliations = []domain.ReconciliationRecord{{ID: "recon", SessionID: "sit-1", Items: []domain.ReconciliationItem{{ID: "item", Status: domain.ReconPending}}}}
	var err error
	ws, err = DeferRecon(ws, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	ws, err = DeferRecon(ws, 0, 0)
	if err != nil || ws.Reconciliations[0].Items[0].Status != domain.ReconDeferred {
		t.Fatalf("repeat defer = %q, %v", ws.Reconciliations[0].Items[0].Status, err)
	}
	ws, err = ApplyRecon(ws, 0, 0)
	if err != nil || ws.Reconciliations[0].Items[0].Status != domain.ReconApproved {
		t.Fatalf("apply deferred = %q, %v", ws.Reconciliations[0].Items[0].Status, err)
	}
}

func TestRejectReconIsIdempotentAndPreservesTranscript(t *testing.T) {
	ws := testWS()
	ws.Sessions = []domain.SessionRecord{{ID: "sit-1", Entries: []domain.TranscriptEntry{{ID: "entry-1", Text: "Immutable evidence"}}}}
	ws.Reconciliations = []domain.ReconciliationRecord{{ID: "recon", SessionID: "sit-1", Items: []domain.ReconciliationItem{{ID: "item", Status: domain.ReconDeferred}}}}

	var err error
	ws, err = RejectRecon(ws, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	ws, err = RejectRecon(ws, 0, 0)
	if err != nil {
		t.Fatalf("repeat reject: %v", err)
	}
	if got := ws.Reconciliations[0].Items[0].Status; got != domain.ReconRejected {
		t.Fatalf("status=%q", got)
	}
	if got := ws.Sessions[0].Entries[0].Text; got != "Immutable evidence" {
		t.Fatalf("reject changed transcript to %q", got)
	}
}

func TestEditReconMutationSupportsDeferredItemsAndValidatesInput(t *testing.T) {
	ws := testWS()
	ws.Reconciliations = []domain.ReconciliationRecord{{ID: "recon", Items: []domain.ReconciliationItem{{
		ID: "item", Status: domain.ReconDeferred, Mutation: domain.Mutation{Text: "Original"},
	}}}}

	updated, err := EditReconMutation(ws, 0, 0, "Owner wording")
	if err != nil {
		t.Fatal(err)
	}
	if got := updated.Reconciliations[0].Items[0].Mutation.Text; got != "Owner wording" {
		t.Fatalf("mutation text=%q", got)
	}
	if _, err := EditReconMutation(updated, 0, 0, ""); err == nil {
		t.Fatal("expected empty mutation text to be rejected")
	}
	updated.Reconciliations[0].Items[0].Status = domain.ReconApproved
	if _, err := EditReconMutation(updated, 0, 0, "Too late"); err == nil {
		t.Fatal("expected decided item edit to be rejected")
	}
}

func TestDeleteAndSupersedeRecord(t *testing.T) {
	ws := testWS()
	ws, err := SupersedeRecord(ws, "npc-vale")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Records[0].Authority != domain.Superseded {
		t.Fatalf("authority=%q", ws.Records[0].Authority)
	}
	ws, err = DeleteRecord(ws, "npc-vale")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Records) != 0 {
		t.Fatalf("records=%#v", ws.Records)
	}
}

func TestDeleteSessionDropsRecon(t *testing.T) {
	ws := testWS()
	ws.Sessions = []domain.SessionRecord{{ID: "sit-1", Title: "Sit"}}
	ws.Reconciliations = []domain.ReconciliationRecord{{ID: "r", SessionID: "sit-1"}}
	ws, err := DeleteSession(ws, "sit-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Sessions) != 0 || len(ws.Reconciliations) != 0 {
		t.Fatalf("sessions=%#v recons=%#v", ws.Sessions, ws.Reconciliations)
	}
}
