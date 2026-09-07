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

func TestJSONStoreRoundTripsSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	s := NewJSON(path)
	scope := domain.Scope{WorldID: "test-world", WorldName: "Test World", CampaignID: "test-campaign", Campaign: "Test Campaign"}
	ws, err := domain.NewWorkspace(scope, []domain.Record{{
		ID: "src-bst-rascals", Type: domain.Creature, Title: "Cave Rascals",
		Authority: domain.Canon, SourceID: "src-bst",
	}})
	if err != nil {
		t.Fatal(err)
	}
	ws.Library = []domain.WorldRef{{
		ID: "test-world", Name: "Test World",
		Campaigns: []domain.CampaignRef{{ID: "test-campaign", Name: "Test Campaign", EnabledSourceIDs: []string{"src-bst"}}},
	}}
	ws.Sources = []domain.SourceDocument{{
		ID: "src-bst", Title: "Test Bestiary (2025)", Edition: "2025", Kind: domain.SourceBestiary,
	}}
	if err := s.Save(ws); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 1 || got.Sources[0].ID != "src-bst" {
		t.Fatalf("sources=%#v", got.Sources)
	}
	if got.Records[0].SourceID != "src-bst" {
		t.Fatalf("record source id=%q", got.Records[0].SourceID)
	}
	if len(got.Library[0].Campaigns[0].EnabledSourceIDs) != 1 {
		t.Fatalf("enablement=%#v", got.Library[0].Campaigns[0].EnabledSourceIDs)
	}
}
