package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func TestJSONStoreLoadMissing(t *testing.T) {
	_, err := NewJSON(filepath.Join(t.TempDir(), "missing.json")).Load()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load error = %v", err)
	}
}

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

func TestJSONStoreRefusesToReplaceCorruptWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	corrupt := []byte(`{"Scope":`)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewJSON(path)
	if err := store.Save(domain.Workspace{}); err == nil || !strings.Contains(err.Error(), "refuse to replace") {
		t.Fatalf("Save error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(corrupt) {
		t.Fatalf("corrupt primary was changed: %q", got)
	}
}

func TestJSONStoreCreatesValidatedBackupBeforeReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	store := NewJSON(path)
	first := domain.Workspace{Records: []domain.Record{{ID: "note-1", Type: domain.Note, Title: "First"}}}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	primary, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second := domain.Workspace{Records: []domain.Record{{ID: "note-2", Type: domain.Note, Title: "Second"}}}
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(primary) {
		t.Fatal("backup does not contain the previous validated workspace")
	}
}

func TestJSONStoreRestoresValidatedBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	store := NewJSON(path)
	first := domain.Workspace{Records: []domain.Record{{ID: "note-1", Type: domain.Note, Title: "First"}}}
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	second := domain.Workspace{Records: []domain.Record{{ID: "note-2", Type: domain.Note, Title: "Second"}}}
	if err := store.Save(second); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreBackup(); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 || got.Records[0].ID != "note-1" {
		t.Fatalf("restored workspace = %#v", got)
	}
}

func TestJSONStoreRoundTripsEmptyPickerScopeAndSchema(t *testing.T) {
	store := NewJSON(filepath.Join(t.TempDir(), "workspace.json"))
	want := domain.Workspace{Library: []domain.WorldRef{{ID: "world", Name: "World"}}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != (domain.Scope{}) || got.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("workspace = %#v", got)
	}
}

func TestJSONStoreRejectsUnknownSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace.json")
	if err := os.WriteFile(path, []byte(`{"SchemaVersion":999}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewJSON(path).Load()
	if err == nil || errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "unsupported workspace schema") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestJSONStoreRejectsAggregateInvariantViolations(t *testing.T) {
	now := time.Now().UTC()
	validRecord := domain.Record{ID: "record", Type: domain.Note, Title: "Record"}
	validSource := domain.SourceDocument{ID: "source", Title: "Source", Kind: domain.SourceAdventure}
	validSession := domain.SessionRecord{ID: "session", Title: "Session", StartedAt: now}
	validPlan := domain.PlannedNotes{ID: "plan", Title: "Plan"}
	validCollection := domain.Collection{ID: "collection", Title: "Collection"}
	cases := []domain.Workspace{
		{Library: []domain.WorldRef{{ID: "world", Name: ""}}},
		{Library: []domain.WorldRef{{ID: "world", Name: "One"}, {ID: "world", Name: "Two"}}},
		{Library: []domain.WorldRef{{ID: "world", Name: "World", Campaigns: []domain.CampaignRef{{ID: "campaign", Name: "One"}, {ID: "campaign", Name: "Two"}}}}},
		{Scope: domain.Scope{WorldID: "missing", CampaignID: "missing"}, Library: []domain.WorldRef{{ID: "world", Name: "World"}}},
		{Records: []domain.Record{validRecord, validRecord}},
		{Sources: []domain.SourceDocument{validSource, validSource}},
		{Sessions: []domain.SessionRecord{validSession, validSession}},
		{PlannedNotes: []domain.PlannedNotes{validPlan, validPlan}},
		{Collections: []domain.Collection{validCollection, validCollection}},
	}
	for index, workspace := range cases {
		store := NewJSON(filepath.Join(t.TempDir(), "workspace.json"))
		if err := store.Save(workspace); err == nil {
			t.Fatalf("case %d unexpectedly saved", index)
		}
	}
}

func TestWorkspaceCloneDoesNotShareNestedState(t *testing.T) {
	original := domain.Workspace{
		Library:  []domain.WorldRef{{ID: "world", Name: "World", Campaigns: []domain.CampaignRef{{ID: "campaign", Name: "Campaign", EnabledSourceIDs: []string{"source"}}}}},
		Records:  []domain.Record{{ID: "record", Type: domain.Note, Title: "Record", Tags: []string{"tag"}}},
		Sessions: []domain.SessionRecord{{ID: "session", Title: "Session", StartedAt: time.Now().UTC(), Entries: []domain.TranscriptEntry{{ID: "entry", Text: "before"}}}},
	}
	clone := original.Clone()
	clone.Library[0].Campaigns[0].EnabledSourceIDs[0] = "changed"
	clone.Records[0].Tags[0] = "changed"
	clone.Sessions[0].Entries[0].Text = "changed"
	if original.Library[0].Campaigns[0].EnabledSourceIDs[0] != "source" || original.Records[0].Tags[0] != "tag" || original.Sessions[0].Entries[0].Text != "before" {
		t.Fatalf("clone mutated original: %#v", original)
	}
}

func TestAtomicWriteReportsMissingDirectory(t *testing.T) {
	err := atomicWrite(filepath.Join(t.TempDir(), "missing", "workspace.json"), []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "temporary file") {
		t.Fatalf("atomicWrite error = %v", err)
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

func TestJSONStoreExportAndRestoreOverCorrupt(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "workspace.json")
	store := NewJSON(primary)
	ws := domain.Workspace{Records: []domain.Record{{ID: "note-1", Type: domain.Note, Title: "Keep"}}}
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(dir, "export.json")
	if err := store.ExportTo(exportPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary, []byte(`{"Scope":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreFrom(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("expected missing restore source to fail")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"Scope":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreFrom(bad); err == nil {
		t.Fatal("expected invalid restore to fail")
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
