package storage

import (
	"path/filepath"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func TestJSONStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "workspace.json")
	s := NewJSON(path)
	scope := domain.Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
	want, err := domain.NewWorkspace(scope, []domain.Record{{
		ID: "npc-1", Type: domain.NPC, Title: "Mira", Authority: domain.Draft,
		Scope: scope, Body: "A draft NPC.", IsAIContent: false,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != want.Scope || len(got.Records) != 1 || got.Records[0].Title != "Mira" {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
