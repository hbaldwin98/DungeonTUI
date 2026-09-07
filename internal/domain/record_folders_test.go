package domain

import "testing"

func TestRecordFolderPathGroupsImportedUnderSource(t *testing.T) {
	rascal := Record{
		ID: "src-5e-bst-creature-rascal", Type: Creature, Title: "Cave Rascal",
		SourceID: "src-5e-bst", Source: "Test Bestiary (2025)", Tags: []string{"creature", "bestiary"},
	}
	vale := Record{ID: "npc-vale", Type: NPC, Title: "Captain Vale"}
	sources := []SourceDocument{{ID: "src-5e-bst", Title: "Test Bestiary (2025)", Kind: SourceBestiary}}
	if got := RecordFolderPath(rascal, sources); got != "Test Bestiary (2025)/Creatures" {
		t.Fatalf("imported folder=%q", got)
	}
	if got := RecordFolderPath(vale, sources); got != "" {
		t.Fatalf("campaign record should be loose, got %q", got)
	}
}

func TestFlattenRecordTreeCollapsesSourceFolders(t *testing.T) {
	records := []Record{
		{ID: "npc-vale", Type: NPC, Title: "Captain Vale"},
		{ID: "g1", Type: Creature, Title: "Cave Rascal", SourceID: "src-bst", Source: "Test Bestiary (2025)", Folder: "Test Bestiary (2025)/Creatures"},
		{ID: "g2", Type: Creature, Title: "Night Hunter", SourceID: "src-bst", Source: "Test Bestiary (2025)", Folder: "Test Bestiary (2025)/Creatures"},
	}
	collapsed := map[string]bool{"Test Bestiary (2025)": true}
	rows := FlattenRecordTree(records, nil, func(path string) bool { return collapsed[path] })
	if len(rows) != 2 {
		t.Fatalf("expected campaign leaf + collapsed book, got %#v", rows)
	}
	if rows[0].Kind != RecordTreeRecord || rows[0].Record.Title != "Captain Vale" {
		t.Fatalf("campaign first: %#v", rows[0])
	}
	if rows[1].Kind != RecordTreeFolder || rows[1].Label != "Test Bestiary (2025)" || rows[1].Count != 2 {
		t.Fatalf("source folder: %#v", rows[1])
	}
}

func TestFlattenRecordTreeExpandsTypeFolder(t *testing.T) {
	records := []Record{
		{ID: "g1", Type: Creature, Title: "Cave Rascal", Folder: "Test Bestiary (2025)/Creatures"},
	}
	rows := FlattenRecordTree(records, nil, func(string) bool { return false })
	var titles []string
	for _, row := range rows {
		titles = append(titles, string(row.Kind)+":"+row.Label)
	}
	want := []string{"folder:Test Bestiary (2025)", "folder:Creatures", "record:Cave Rascal"}
	if len(titles) != 3 || titles[0] != want[0] || titles[1] != want[1] || titles[2] != want[2] {
		t.Fatalf("got %v", titles)
	}
}

func TestFlattenRecordTreeNestsPrefixedRoomsAndSortsNaturally(t *testing.T) {
	records := []Record{
		{ID: "p", Type: Location, Title: "Millhaven", Folder: "ADV/Locations"},
		{ID: "p1", Type: Location, Title: "Millhaven — 1. The Mill Inn", Folder: "ADV/Locations"},
		{ID: "p10", Type: Location, Title: "Millhaven - 10. Mill Shrine", Folder: "ADV/Locations"},
		{ID: "p2", Type: Location, Title: "Millhaven — 2. Mill Store", Folder: "ADV/Locations"},
	}
	rows := FlattenRecordTree(records, nil, func(string) bool { return false })
	var labels []string
	for _, row := range rows {
		labels = append(labels, string(row.Kind)+":"+row.Label)
	}
	want := []string{
		"folder:ADV",
		"folder:Locations",
		"folder:Millhaven",
		"record:Millhaven",
		"record:1. The Mill Inn",
		"record:2. Mill Store",
		"record:10. Mill Shrine",
	}
	if len(labels) != len(want) {
		t.Fatalf("got %v", labels)
	}
	for i, item := range want {
		if labels[i] != item {
			t.Fatalf("row %d: got %v want %v", i, labels, want)
		}
	}
}

func TestNestOverviewFoldersMovesParentBesideRooms(t *testing.T) {
	records := []Record{
		{ID: "p", Title: "Millhaven", Folder: "ADV/Locations"},
		{ID: "p1", Title: "1. The Mill Inn", Folder: "ADV/Locations/Millhaven"},
	}
	NestOverviewFolders(records)
	if records[0].Folder != "ADV/Locations/Millhaven" {
		t.Fatalf("overview folder=%q", records[0].Folder)
	}
}
