package fivetools

import (
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func TestParseRefFromFiveEToolsURLs(t *testing.T) {
	cases := []struct {
		in   string
		id   string
		kind string
	}{
		{"https://5e.tools/adventure.html#adv,-1", "adv", "adventure"},
		{"https://5e.tools/book.html#bst,-1", "bst", "book"},
		{"5e:BST", "BST", ""},
		{"ADV", "ADV", ""},
	}
	for _, tc := range cases {
		ref, ok := ParseRef(tc.in)
		if !ok {
			t.Fatalf("ParseRef(%q) failed", tc.in)
		}
		if !strings.EqualFold(ref.ID, tc.id) {
			t.Fatalf("ParseRef(%q).ID=%q want %q", tc.in, ref.ID, tc.id)
		}
		if tc.kind != "" && ref.Kind != tc.kind {
			t.Fatalf("ParseRef(%q).Kind=%q want %q", tc.in, ref.Kind, tc.kind)
		}
	}
	if LooksLikeRef("The Hollow Crown.md") {
		t.Fatal("markdown path should not look like a 5e.tools ref")
	}
}

func TestRenderTagsBecomeMentions(t *testing.T) {
	got := renderTags(`hire {@creature Mira Holt|ADV} and cast {@spell Spark Bolt|TST}`)
	if !strings.Contains(got, "@Mira Holt") || !strings.Contains(got, "@Spark Bolt") {
		t.Fatalf("got %q", got)
	}
	atk := renderTags("{@atk mw} {@hit 3} to hit. {@h} {@damage 1d6 + 1}")
	if !strings.Contains(atk, "Melee Weapon Attack:") || !strings.Contains(atk, "+3") || !strings.Contains(atk, "Hit:") {
		t.Fatalf("attack tags: %q", atk)
	}
}

func TestConvertMonsterKeepsPrintedStats(t *testing.T) {
	item := map[string]any{
		"name": "Cave Rascal", "source": "BST", "size": []any{"S"},
		"type": map[string]any{"type": "humanoid", "tags": []any{"raider"}},
		"ac":   []any{float64(12)},
		"hp":   map[string]any{"average": float64(9), "formula": "2d6"},
		"str":  float64(9), "dex": float64(13), "con": float64(11),
		"int": float64(8), "wis": float64(10), "cha": float64(7),
		"speed": map[string]any{"walk": float64(30)},
		"cr":    "1/8",
		"action": []any{map[string]any{
			"name":    "Shortblade",
			"entries": []any{"{@atk mw} {@hit 3} to hit, reach 5 ft. {@h}{@damage 1d4 + 1} Piercing."},
		}},
	}
	drafts := convertMonsters([]map[string]any{item}, nil, "BST", nil, false)
	if len(drafts) != 1 {
		t.Fatalf("drafts=%d", len(drafts))
	}
	body := drafts[0].Body
	for _, want := range []string{"**AC** 12", "**HP** 9 (2d6)", "**CR** 1/8", "Shortblade", "Melee Weapon Attack:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestConvertMonsterDoesNotInventArmorClass(t *testing.T) {
	item := map[string]any{"name": "Mystery Beast", "source": "BST", "hp": map[string]any{"average": float64(4)}}
	body := convertMonsters([]map[string]any{item}, nil, "BST", nil, false)[0].Body
	if strings.Contains(body, "**AC**") {
		t.Fatalf("invented AC:\n%s", body)
	}
	if !strings.Contains(body, "**HP** 4") {
		t.Fatalf("expected printed HP:\n%s", body)
	}
}

func testFetcher() MapFetcher {
	return MapFetcher{
		"data/adventures.json":     []byte(`{"adventure":[{"id":"ADV","name":"The Hollow Crown","group":"supplement","published":"2020-01-01"}]}`),
		"data/books.json":          []byte(`{"book":[{"id":"BST","name":"Test Bestiary (2025)","group":"core","published":"2025-02-18"}]}`),
		"data/bestiary/index.json": []byte(`{"ADV":"bestiary-adv.json","BST":"bestiary-bst.json"}`),
		"data/spells/index.json":   []byte(`{}`),
		"data/bestiary/bestiary-bst.json": []byte(`{"monster":[{
			"name":"Cave Rascal","source":"BST","size":["S"],
			"type":{"type":"humanoid","tags":["raider"]},
			"ac":[12],"hp":{"average":9,"formula":"2d6"},
			"str":9,"dex":13,"con":11,"int":8,"wis":10,"cha":7,
			"speed":{"walk":30},"cr":"1/8",
			"action":[{"name":"Shortblade","entries":["{@atk mw} {@hit 3} to hit. {@h}{@damage 1d4 + 1} Piercing."]}]
		}]}`),
		"data/bestiary/bestiary-adv.json": []byte(`{"monster":[{
			"name":"Marsh Wight","source":"ADV","size":["M"],"type":"undead",
			"ac":[11],"hp":{"average":13,"formula":"3d8"},
			"str":12,"dex":8,"con":13,"int":6,"wis":9,"cha":6,"cr":"1/4"
		}]}`),
		"data/adventure/adventure-adv.json": []byte(`{"data":[{
			"type":"section","name":"Introduction",
			"entries":[
				"How to run this adventure.",
				{"type":"section","name":"The Hollow Crown","entries":[
					{"type":"entries","name":"Millhaven","entries":["A duplicate town copy."]},
					{"type":"entries","name":"1. The Mill Inn","entries":["A duplicate inn copy."]}
				]},
				{"type":"entries","name":"Running the Adventure","entries":["Advice for the DM."]}
			]
		},{
			"type":"section","name":"Millhaven",
			"entries":[
				"A frontier town.",
				{"type":"entries","name":"Important NPCs","entries":[
					{"type":"table","colLabels":["Name","Role"],"rows":[
						["{@creature Mira Holt|ADV}","Innkeeper."],
						["{@creature Captain Reed|ADV}","Town guard."]
					]}
				]},
				{"type":"entries","name":"1. The Mill Inn","entries":[
					"A stout inn.",
					{"type":"entries","name":"Mira Holt","entries":["She greets guests at the bar."]}
				]},
				{"type":"entries","name":"10. Mill Shrine","entries":["A small shrine."]}
			]
		},{
			"type":"section","name":"The Ambush",
			"entries":[
				"The wagon is ambushed by {@creature cave rascal|BST|cave rascals}.",
				{"type":"section","name":"The Hideout","entries":[
					{"type":"entries","name":"1. Cave Mouth","entries":["Two {@creature cave rascal|BST|cave rascals} watch the cave."]}
				]}
			]
		}]}`),
		"data/book/book-bst.json": []byte(`{"data":[{"type":"section","name":"How to Use a Monster","entries":["Read the stat block."]}]}`),
	}
}

func TestBuildBSTCreatesLibraryCreatureWithStats(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/book.html#bst,-1")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Doc.ID != "src-5e-bst" || bundle.Doc.Kind != domain.SourceBestiary {
		t.Fatalf("doc=%#v", bundle.Doc)
	}
	found := false
	for _, d := range bundle.Records {
		if d.Title == "Cave Rascal" && d.Type == domain.Creature && strings.Contains(d.Body, "**AC** 12") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing cave rascal stats: %#v", titles(bundle))
	}
}

func TestBuildADVCreatesAdventureMentionsAndPrep(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/adventure.html#adv,-1")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Doc.Kind != domain.SourceAdventure {
		t.Fatalf("kind=%s", bundle.Doc.Kind)
	}
	var cave string
	for _, d := range bundle.Records {
		if strings.Contains(d.Title, "Cave Mouth") {
			cave = d.Body
		}
	}
	if !strings.Contains(cave, "@cave rascal") {
		t.Fatalf("expected @ mention in room text: %q", cave)
	}
	if len(bundle.Plans) == 0 {
		t.Fatal("expected prep notes for adventure parts")
	}
}

func TestConvertAdventureNestsRoomsAndDedupsNPCs(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/adventure.html#adv,-1")
	if err != nil {
		t.Fatal(err)
	}
	type hit struct {
		typ   domain.EntityType
		group string
		body  string
		count int
	}
	found := map[string]*hit{}
	for _, d := range bundle.Records {
		item := found[d.Title]
		if item == nil {
			item = &hit{}
			found[d.Title] = item
		}
		item.typ = d.Type
		item.group = d.Group
		item.body = d.Body
		item.count++
	}
	if found["Millhaven"] == nil || found["Millhaven"].typ != domain.Location {
		t.Fatalf("expected Millhaven location, got %#v", titles(bundle))
	}
	if strings.Contains(found["Millhaven"].body, "A stout inn") {
		t.Fatalf("parent should not copy child rooms: %q", found["Millhaven"].body)
	}
	if found["1. The Mill Inn"] == nil || found["1. The Mill Inn"].group != "Millhaven" {
		t.Fatalf("room should nest under Millhaven, got %#v", found["1. The Mill Inn"])
	}
	if found["10. Mill Shrine"] == nil || found["10. Mill Shrine"].group != "Millhaven" {
		t.Fatalf("shrine should nest under Millhaven")
	}
	if found["Millhaven — 1. The Mill Inn"] != nil || found["Millhaven - 1"] != nil {
		t.Fatalf("prefixed sibling titles still present: %#v", titles(bundle))
	}
	mira := found["Mira Holt"]
	if mira == nil || mira.typ != domain.NPC {
		t.Fatalf("expected one Mira NPC, got %#v", found["Mira Holt"])
	}
	if mira.count != 1 {
		t.Fatalf("Mira duplicated %d times", mira.count)
	}
	if strings.Contains(mira.body, "Captain Reed") {
		t.Fatalf("NPC should not copy the whole table: %q", mira.body)
	}
	if !strings.Contains(mira.body, "greets guests") && !strings.Contains(mira.body, "Innkeeper") {
		t.Fatalf("Mira should keep her own text: %q", mira.body)
	}
	if found["1. Cave Mouth"] == nil || found["1. Cave Mouth"].group != "The Ambush/The Hideout" {
		t.Fatalf("cave group=%#v", found["1. Cave Mouth"])
	}
	intro := found["Introduction"]
	if intro == nil || intro.typ != domain.Note {
		t.Fatalf("introduction should be a note, got %#v", found["Introduction"])
	}
	if found["The Hollow Crown"] != nil {
		t.Fatalf("intro should not clone the adventure as a location: %#v", titles(bundle))
	}
	if run := found["Running the Adventure"]; run != nil && run.typ == domain.Location {
		t.Fatalf("intro advice should stay a note, got %#v", run)
	}
	if found["1. The Mill Inn"] != nil && found["1. The Mill Inn"].count != 1 {
		t.Fatalf("The Mill Inn duplicated %d times", found["1. The Mill Inn"].count)
	}
}

func TestConvertGenericAdventureUsesStructureNotTitleLists(t *testing.T) {
	fetcher := MapFetcher{
		"data/adventures.json":     []byte(`{"adventure":[{"id":"Gate","name":"The Gatehouse Run","published":"2020-01-01"}]}`),
		"data/books.json":          []byte(`{"book":[]}`),
		"data/bestiary/index.json": []byte(`{}`),
		"data/spells/index.json":   []byte(`{}`),
		"data/adventure/adventure-gate.json": []byte(`{"data":[{
			"type":"section","name":"Introduction",
			"entries":[
				"How to run this.",
				{"type":"section","name":"The Village","entries":["A reprint of the later village."]},
				{"type":"entries","name":"Using This Adventure","entries":["Advice unique to the intro."]}
			]
		},{
			"type":"section","name":"The Village",
			"entries":[
				{"type":"table","colLabels":["Name","Role"],"rows":[["Nilo Voss","Local guide."]]},
				{"type":"entries","name":"1. The Inn","entries":["A taproom."]}
			]
		},{
			"type":"section","name":"The Citadel",
			"entries":[
				{"type":"section","name":"The Gatehouse","entries":[
					{"type":"entries","name":"1. Portcullis","entries":["Two guards."]}
				]}
			]
		}]}`),
	}
	bundle, err := Build(fetcher, "5e:Gate")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.EntityType{}
	group := map[string]string{}
	for _, d := range bundle.Records {
		got[d.Title] = d.Type
		group[d.Title] = d.Group
	}
	if got["Introduction"] != domain.Note {
		t.Fatalf("intro=%s titles=%v", got["Introduction"], got)
	}
	if got["The Village"] != domain.Location {
		t.Fatalf("village=%s", got["The Village"])
	}
	if got["1. The Inn"] != domain.Location || group["1. The Inn"] != "The Village" {
		t.Fatalf("inn type=%s group=%s", got["1. The Inn"], group["1. The Inn"])
	}
	if got["Nilo Voss"] != domain.NPC {
		t.Fatalf("npc table should yield Nilo Voss, got %v", got)
	}
	if got["1. Portcullis"] != domain.Location || group["1. Portcullis"] != "The Citadel/The Gatehouse" {
		t.Fatalf("portcullis type=%s group=%s", got["1. Portcullis"], group["1. Portcullis"])
	}
	if got["The Village"] == domain.Location && bundleHasDuplicateLocation(bundle, "The Village") {
		t.Fatal("intro reprint of The Village should not be a second location")
	}
	if got["Using This Adventure"] == domain.Location {
		t.Fatal("intro-only advice must not become a location")
	}
}

func bundleHasDuplicateLocation(b Bundle, title string) bool {
	n := 0
	for _, d := range b.Records {
		if d.Title == title && d.Type == domain.Location {
			n++
		}
	}
	return n > 1
}

func titles(b Bundle) []string {
	out := make([]string, 0, len(b.Records))
	for _, d := range b.Records {
		out = append(out, d.Title)
	}
	return out
}

func TestBuildBSTSkipsUnrelatedSharedFiles(t *testing.T) {
	inner := testFetcher()
	rec := &recordingFetcher{inner: inner}
	if _, err := Build(rec, "https://5e.tools/book.html#bst,-1"); err != nil {
		t.Fatal(err)
	}
	for _, path := range rec.paths {
		if path == "data/items.json" || strings.HasPrefix(path, "data/class/") {
			t.Fatalf("BST ingest should not fetch %s; got %v", path, rec.paths)
		}
	}
}

type recordingFetcher struct {
	inner MapFetcher
	paths []string
}

func (r *recordingFetcher) Get(path string) ([]byte, error) {
	r.paths = append(r.paths, path)
	return r.inner.Get(path)
}

func TestLoadCatalog(t *testing.T) {
	entries, err := LoadCatalog(testFetcher())
	if err != nil {
		t.Fatal(err)
	}
	if len(FilterCatalog(entries, "hollow")) != 1 {
		t.Fatalf("filter=%#v", entries)
	}
}
