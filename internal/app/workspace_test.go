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
