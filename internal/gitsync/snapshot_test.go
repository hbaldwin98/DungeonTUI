package gitsync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

func sampleSnapshot() Snapshot {
	scope := domain.Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
	return Snapshot{
		Workspace: domain.Workspace{
			SchemaVersion: storage.CurrentSchemaVersion,
			Scope:         scope,
			Library: []domain.WorldRef{{
				ID:   scope.WorldID,
				Name: scope.WorldName,
				Campaigns: []domain.CampaignRef{
					{ID: scope.CampaignID, Name: scope.Campaign},
				},
			}},
			Records: []domain.Record{{
				ID: "npc-mira", Type: domain.NPC, Title: "Mira", Authority: domain.Canon,
				Scope: scope, Body: "Captain of the watch.",
			}},
			Sources: []domain.SourceDocument{{
				ID: "src-notes", Title: "Field Notes", Kind: domain.SourceAdventure,
			}},
		},
		Adventures: []storage.Blob{{
			ID:   "src-5e-crown",
			Data: []byte(`{"ID":"src-5e-crown","Title":"The Ashen Crown"}` + "\n"),
		}},
	}
}

func TestWriteThenReadRoundTripsEntities(t *testing.T) {
	root := t.TempDir()
	want := sampleSnapshot()
	if err := Write(root, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace.Scope != want.Workspace.Scope {
		t.Fatalf("scope: got %#v want %#v", got.Workspace.Scope, want.Workspace.Scope)
	}
	if len(got.Workspace.Records) != 1 || got.Workspace.Records[0].Title != "Mira" {
		t.Fatalf("records: %#v", got.Workspace.Records)
	}
	if len(got.Workspace.Sources) != 1 || got.Workspace.Sources[0].ID != "src-notes" {
		t.Fatalf("sources: %#v", got.Workspace.Sources)
	}
	if len(got.Adventures) != 1 || got.Adventures[0].ID != "src-5e-crown" {
		t.Fatalf("adventures: %#v", got.Adventures)
	}
}

func TestWriteRemovesDeletedEntityFiles(t *testing.T) {
	root := t.TempDir()
	snap := sampleSnapshot()
	if err := Write(root, snap); err != nil {
		t.Fatal(err)
	}
	snap.Workspace.Records = nil
	if err := Write(root, snap); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workspace.Records) != 0 {
		t.Fatalf("deleted record still present: %#v", got.Workspace.Records)
	}
	matches, err := filepath.Glob(filepath.Join(root, workspaceDir, dirRecords, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("stale record files remain: %v", matches)
	}
}

func TestWriteLeavesOwnerFilesOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("my campaign notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, sampleSnapshot()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "my campaign notes\n" {
		t.Fatalf("owner file was rewritten: %q", got)
	}
}

func TestReadMissingMetaMeansUnwritten(t *testing.T) {
	_, err := Read(t.TempDir())
	if err == nil {
		t.Fatal("expected an error for a repository Dungeon has never written")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing snapshot should be not-exist, got %v", err)
	}
}

func TestWriteKeepsUnsafeIDsInsideTheFile(t *testing.T) {
	root := t.TempDir()
	snap := sampleSnapshot()
	snap.Workspace.Records[0].ID = "npc/mira holt"
	if err := Write(root, snap); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace.Records[0].ID != "npc/mira holt" {
		t.Fatalf("id lost in filename sanitizing: %#v", got.Workspace.Records[0])
	}
}
