package classify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func TestAssignUsesStructureNotBookTitles(t *testing.T) {
	cases := []struct {
		title, group string
		tags         []string
		want         domain.EntityType
	}{
		{"Introduction", "", nil, domain.Note},
		{"Running the Adventure", "Introduction", nil, domain.Note},
		{"The Village", "", []string{"5e-section"}, domain.Location},
		{"1. The Inn", "The Village", nil, domain.Location},
		{"Treasure", "The Village/1. The Inn", nil, domain.Note},
		{"Preface", "", nil, domain.Note},
	}
	for _, tc := range cases {
		got := Assign(tc.title, tc.group, tc.tags)
		if got != tc.want {
			t.Fatalf("%s [%s]=%s want %s", tc.title, tc.group, got, tc.want)
		}
	}
}

func TestDumpReportsRetypeFindings(t *testing.T) {
	ws := domain.Workspace{
		Scope: domain.Scope{Campaign: "Test", WorldName: "World"},
		Records: []domain.Record{
			{ID: "intro", Type: domain.Location, Title: "Introduction", SourceID: "src-x"},
			{ID: "room", Type: domain.Note, Title: "1. Gate", Folder: "Book/Notes/The Dungeon", SourceID: "src-x"},
		},
	}
	snap := Dump(ws, "", false)
	if len(snap.Records) != 2 {
		t.Fatalf("records=%d", len(snap.Records))
	}
	kinds := map[Kind]int{}
	for _, f := range snap.Findings {
		kinds[f.Kind]++
	}
	if kinds[KindFrontMatter] == 0 || kinds[KindRoom] == 0 {
		t.Fatalf("findings=%#v", snap.Findings)
	}
}

func TestOrganizeRecordsRetypesFrontMatter(t *testing.T) {
	records := []domain.Record{
		{ID: "intro", Type: domain.Location, Title: "Introduction", Source: "Book"},
	}
	OrganizeRecords(records)
	if records[0].Type != domain.Note {
		t.Fatalf("type=%s", records[0].Type)
	}
	if !strings.Contains(records[0].Folder, "Notes") {
		t.Fatalf("folder=%q", records[0].Folder)
	}
}

func TestParseHarnessCyclesOffAndOn(t *testing.T) {
	h, err := ParseHarness("on")
	if err != nil || h != HarnessOn {
		t.Fatalf("on -> %q %v", h, err)
	}
	dump, err := ParseHarness("dump")
	if err != nil || dump != HarnessOn {
		t.Fatalf("legacy dump alias -> %q %v", dump, err)
	}
	if NextHarness(HarnessOff) != HarnessOn || NextHarness(HarnessOn) != HarnessOff {
		t.Fatal("cycle should be off → on → off")
	}
}

func TestWriteDumpInspectsWorkspace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inspect.json")
	ws := domain.Workspace{
		Scope:   domain.Scope{Campaign: "Test", WorldName: "World"},
		Records: []domain.Record{{ID: "intro", Type: domain.Location, Title: "Introduction", SourceID: "src-x"}},
	}
	if err := WriteDump(path, ws, "src-x", false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Introduction") || !strings.Contains(string(raw), "front-matter") {
		t.Fatalf("inspect=%s", raw)
	}
}
