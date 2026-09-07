package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/search"
)

func TestSQLiteStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "workspace.sqlite")
	store := NewSQLite(path)
	defer store.Close()
	scope := domain.Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
	want, err := domain.NewWorkspace(scope, []domain.Record{{
		ID: "npc-1", Type: domain.NPC, Title: "Mira", Authority: domain.Draft,
		Scope: scope, Body: "A draft NPC.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != want.Scope || len(got.Records) != 1 || got.Records[0].Title != "Mira" {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}

func TestSQLiteStoreSearchUsesTheSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.sqlite")
	store := NewSQLite(path)
	defer store.Close()
	scope := domain.Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
	ws, err := domain.NewWorkspace(scope, []domain.Record{{
		ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale", Authority: domain.Canon,
		Scope: scope, Body: "Wary of the party.",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	docs := search.DocumentsFromWorkspace(ws)
	if err := store.Reindex(docs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "search.sqlite")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("search must live in workspace.sqlite, not a sidecar")
	}
	results := store.Search().Find(search.Filter{
		Query: "vale", Scope: search.CurrentCampaign,
		WorldID: scope.WorldID, CampaignID: scope.CampaignID,
	})
	if len(results) != 1 || results[0].Record.ID != "npc-vale" {
		t.Fatalf("expected vale from sqlite FTS, got %#v", results)
	}
}

func TestSQLiteStoreMigratesJSONWorkspace(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "workspace.json")
	sqlitePath := filepath.Join(dir, "workspace.sqlite")
	legacy := NewJSON(jsonPath)
	ws := domain.Workspace{Records: []domain.Record{{ID: "note-1", Type: domain.Note, Title: "Keep"}}}
	if err := legacy.Save(ws); err != nil {
		t.Fatal(err)
	}
	store := NewSQLite(sqlitePath)
	defer store.Close()
	if err := store.MigrateFromJSON(jsonPath); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 || got.Records[0].Title != "Keep" {
		t.Fatalf("migrated %#v", got)
	}
}

func TestSQLiteStoreRefusesToReplaceCorruptWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.sqlite")
	if err := os.WriteFile(path, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewSQLite(path)
	defer store.Close()
	if err := store.Save(domain.Workspace{}); err == nil || !strings.Contains(err.Error(), "refuse to replace") {
		t.Fatalf("Save error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "this is not a sqlite database" {
		t.Fatalf("corrupt primary was changed: %q", got)
	}
}

func TestSQLiteStoreExportAndRestoreOverCorrupt(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "workspace.sqlite")
	store := NewSQLite(primary)
	defer store.Close()
	ws := domain.Workspace{Records: []domain.Record{{ID: "note-1", Type: domain.Note, Title: "Keep"}}}
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(dir, "export.json")
	if err := store.ExportTo(exportPath); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err := os.WriteFile(primary, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreFrom(exportPath); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 || got.Records[0].Title != "Keep" {
		t.Fatalf("restored %#v", got)
	}
}

func TestSQLiteStoreRoundTripsAdventureBlobs(t *testing.T) {
	store := NewSQLite(filepath.Join(t.TempDir(), "workspace.sqlite"))
	defer store.Close()
	if err := store.Save(domain.Workspace{}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAdventures([]Blob{{ID: "src-5e-adv", Data: []byte(`{"ID":"src-5e-adv","Title":"Crown"}`)}}); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadAdventures()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "src-5e-adv" || !strings.Contains(string(got[0].Data), "Crown") {
		t.Fatalf("adventures %#v", got)
	}
}
