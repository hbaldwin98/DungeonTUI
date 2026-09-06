package domain

import (
	"fmt"
	"testing"
	"time"
)

func TestNormalizeFolder(t *testing.T) {
	if got := NormalizeFolder("  Greywatch / Crypt / "); got != "Greywatch/Crypt" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeFolder("../secret/./keep"); got != "secret/keep" {
		t.Fatalf("dot-dot stripped, got %q", got)
	}
	if got := NormalizeFolder(""); got != "" {
		t.Fatalf("empty should stay empty, got %q", got)
	}
}

func TestSessionFolderPathNamedWinsOverMonth(t *testing.T) {
	session := SessionRecord{
		Folder:    " Greywatch/Crypt ",
		StartedAt: time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC),
	}
	if got := SessionFolderPath(session); got != "Greywatch/Crypt" {
		t.Fatalf("got %q", got)
	}
	unfiled := SessionRecord{}
	if got := SessionFolderPath(unfiled); got != UnfiledFolder {
		t.Fatalf("zero time → Unfiled, got %q", got)
	}
	month := SessionRecord{StartedAt: time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)}
	if got := SessionFolderPath(month); got != "2026-03" {
		t.Fatalf("got %q", got)
	}
}

func TestFlattenSessionTreeNestsAndCollapses(t *testing.T) {
	ended := time.Date(2026, 3, 2, 21, 0, 0, 0, time.UTC)
	sessions := []SessionRecord{
		{ID: "old-month", Title: "January sit", StartedAt: time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)},
		{ID: "crypt-2", Title: "Crypt night 2", Folder: "Greywatch/Crypt", StartedAt: ended},
		{ID: "crypt-1", Title: "Crypt night 1", Folder: "Greywatch/Crypt", StartedAt: ended.Add(-24 * time.Hour)},
		{ID: "loose", Title: "Loose march sit", StartedAt: time.Date(2026, 3, 8, 18, 0, 0, 0, time.UTC)},
	}

	rows := FlattenSessionTree(sessions, nil)
	got := rowSummary(rows)
	want := []string{
		"folder:Greywatch:0",
		"folder:Greywatch/Crypt:1",
		"session:crypt-2:2",
		"session:crypt-1:2",
		"folder:2026-03:0",
		"session:loose:1",
		"folder:2026-01:0",
		"session:old-month:1",
	}
	if len(got) != len(want) {
		t.Fatalf("rows=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: got %s want %s\nall=%v", i, got[i], want[i], got)
		}
	}

	collapsed := map[string]bool{"Greywatch": true}
	hidden := rowSummary(FlattenSessionTree(sessions, collapsed))
	if len(hidden) != 5 {
		t.Fatalf("collapsed Greywatch should hide crypt sits, got %v", hidden)
	}
	if hidden[0] != "folder:Greywatch:0" {
		t.Fatalf("expected Greywatch header first, got %v", hidden)
	}
	for _, row := range hidden {
		if row == "session:crypt-2:2" || row == "folder:Greywatch/Crypt:1" {
			t.Fatalf("collapsed parent still showing child %s in %v", row, hidden)
		}
	}
}

func TestApplySessionFolderAndPrefix(t *testing.T) {
	sessions := []SessionRecord{
		{ID: "a", Title: "A", StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", Title: "B", Folder: "Greywatch"},
	}
	sessions = ApplySessionFolder(sessions, []string{"a", "missing"}, " Greywatch/Crypt ")
	if sessions[0].Folder != "Greywatch/Crypt" {
		t.Fatalf("a should be filed, got %q", sessions[0].Folder)
	}
	if !FolderPrefix(SessionFolderPath(sessions[0]), "Greywatch") {
		t.Fatal("Crypt should sit under Greywatch")
	}
	sessions = ApplySessionFolder(sessions, []string{"a"}, "")
	if sessions[0].Folder != "" {
		t.Fatalf("empty folder unfiles, got %q", sessions[0].Folder)
	}
}

func rowSummary(rows []SessionTreeRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		id := row.Label
		if row.Kind == SessionTreeSession {
			id = row.Session.ID
		} else {
			id = row.Path
		}
		out = append(out, fmt.Sprintf("%s:%s:%d", row.Kind, id, row.Depth))
	}
	return out
}
