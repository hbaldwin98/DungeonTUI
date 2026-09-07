package domain

import "testing"

func TestRecordFolderPathGroupsImportedUnderSource(t *testing.T) {
	goblin := Record{
		ID: "src-5e-xmm-creature-goblin", Type: Creature, Title: "Goblin Warrior",
		SourceID: "src-5e-xmm", Source: "Monster Manual (2025)", Tags: []string{"creature", "bestiary"},
	}
	vale := Record{ID: "npc-vale", Type: NPC, Title: "Captain Vale"}
	sources := []SourceDocument{{ID: "src-5e-xmm", Title: "Monster Manual (2025)", Kind: SourceBestiary}}
	if got := RecordFolderPath(goblin, sources); got != "Monster Manual (2025)/Creatures" {
		t.Fatalf("imported folder=%q", got)
	}
	if got := RecordFolderPath(vale, sources); got != "" {
		t.Fatalf("campaign record should be loose, got %q", got)
	}
}

func TestFlattenRecordTreeCollapsesSourceFolders(t *testing.T) {
	records := []Record{
		{ID: "npc-vale", Type: NPC, Title: "Captain Vale"},
		{ID: "g1", Type: Creature, Title: "Goblin Warrior", SourceID: "src-xmm", Source: "Monster Manual (2025)", Folder: "Monster Manual (2025)/Creatures"},
		{ID: "g2", Type: Creature, Title: "Bugbear", SourceID: "src-xmm", Source: "Monster Manual (2025)", Folder: "Monster Manual (2025)/Creatures"},
	}
	collapsed := map[string]bool{"Monster Manual (2025)": true}
	rows := FlattenRecordTree(records, nil, func(path string) bool { return collapsed[path] })
	if len(rows) != 2 {
		t.Fatalf("expected campaign leaf + collapsed book, got %#v", rows)
	}
	if rows[0].Kind != RecordTreeRecord || rows[0].Record.Title != "Captain Vale" {
		t.Fatalf("campaign first: %#v", rows[0])
	}
	if rows[1].Kind != RecordTreeFolder || rows[1].Label != "Monster Manual (2025)" || rows[1].Count != 2 {
		t.Fatalf("source folder: %#v", rows[1])
	}
}

func TestFlattenRecordTreeExpandsTypeFolder(t *testing.T) {
	records := []Record{
		{ID: "g1", Type: Creature, Title: "Goblin Warrior", Folder: "Monster Manual (2025)/Creatures"},
	}
	rows := FlattenRecordTree(records, nil, func(string) bool { return false })
	var titles []string
	for _, row := range rows {
		titles = append(titles, string(row.Kind)+":"+row.Label)
	}
	want := []string{"folder:Monster Manual (2025)", "folder:Creatures", "record:Goblin Warrior"}
	if len(titles) != 3 || titles[0] != want[0] || titles[1] != want[1] || titles[2] != want[2] {
		t.Fatalf("got %v", titles)
	}
}

func TestFlattenRecordTreeNestsPrefixedRoomsAndSortsNaturally(t *testing.T) {
	records := []Record{
		{ID: "p", Type: Location, Title: "Phandalin", Folder: "LMoP/Locations"},
		{ID: "p1", Type: Location, Title: "Phandalin — 1. Stonehill Inn", Folder: "LMoP/Locations"},
		{ID: "p10", Type: Location, Title: "Phandalin - 10. Shrine of Luck", Folder: "LMoP/Locations"},
		{ID: "p2", Type: Location, Title: "Phandalin — 2. Barthen's Provisions", Folder: "LMoP/Locations"},
	}
	rows := FlattenRecordTree(records, nil, func(string) bool { return false })
	var labels []string
	for _, row := range rows {
		labels = append(labels, string(row.Kind)+":"+row.Label)
	}
	want := []string{
		"folder:LMoP",
		"folder:Locations",
		"folder:Phandalin",
		"record:Phandalin",
		"record:1. Stonehill Inn",
		"record:2. Barthen's Provisions",
		"record:10. Shrine of Luck",
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
		{ID: "p", Title: "Phandalin", Folder: "LMoP/Locations"},
		{ID: "p1", Title: "1. Stonehill Inn", Folder: "LMoP/Locations/Phandalin"},
	}
	NestOverviewFolders(records)
	if records[0].Folder != "LMoP/Locations/Phandalin" {
		t.Fatalf("overview folder=%q", records[0].Folder)
	}
}
