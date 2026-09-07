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
		{"https://5e.tools/adventure.html#lmop,-1", "lmop", "adventure"},
		{"https://5e.tools/book.html#xdmg,-1", "xdmg", "book"},
		{"https://5e.tools/book.html#xphb,-1", "xphb", "book"},
		{"https://5e.tools/book.html#xmm,-1", "xmm", "book"},
		{"5e:XMM", "XMM", ""},
		{"LMoP", "LMoP", ""},
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
	if LooksLikeRef("Lost Mine of Phandelver.md") {
		t.Fatal("markdown path should not look like a 5e.tools ref")
	}
}

func TestRenderTagsBecomeMentions(t *testing.T) {
	got := renderTags(`hire {@creature Gundren Rockseeker|LMoP} and cast {@spell Acid Splash|XPHB}`)
	if !strings.Contains(got, "@Gundren Rockseeker") || !strings.Contains(got, "@Acid Splash") {
		t.Fatalf("got %q", got)
	}
	atk := renderTags("{@atk mw} {@hit 3} to hit. {@h} {@damage 1d6 + 1}")
	if !strings.Contains(atk, "Melee Weapon Attack:") || !strings.Contains(atk, "+3") || !strings.Contains(atk, "Hit:") {
		t.Fatalf("attack tags: %q", atk)
	}
}

func TestConvertMonsterKeepsPrintedStats(t *testing.T) {
	item := map[string]any{
		"name": "Goblin Warrior", "source": "XMM", "size": []any{"S"},
		"type": map[string]any{"type": "fey", "tags": []any{"goblinoid"}},
		"ac":   []any{float64(15)},
		"hp":   map[string]any{"average": float64(10), "formula": "3d6"},
		"str":  float64(8), "dex": float64(15), "con": float64(10),
		"int": float64(10), "wis": float64(8), "cha": float64(8),
		"speed": map[string]any{"walk": float64(30)},
		"cr":    "1/4",
		"action": []any{map[string]any{
			"name":    "Scimitar",
			"entries": []any{"{@atk mw} {@hit 4} to hit, reach 5 ft. {@h}{@damage 1d6 + 2} Slashing."},
		}},
	}
	drafts := convertMonsters([]map[string]any{item}, nil, "XMM", nil, false)
	if len(drafts) != 1 {
		t.Fatalf("drafts=%d", len(drafts))
	}
	body := drafts[0].Body
	for _, want := range []string{"**AC** 15", "**HP** 10 (3d6)", "**CR** 1/4", "Scimitar", "Melee Weapon Attack:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestConvertMonsterDoesNotInventArmorClass(t *testing.T) {
	item := map[string]any{"name": "Mystery Beast", "source": "XMM", "hp": map[string]any{"average": float64(4)}}
	body := convertMonsters([]map[string]any{item}, nil, "XMM", nil, false)[0].Body
	if strings.Contains(body, "**AC**") {
		t.Fatalf("invented AC:\n%s", body)
	}
	if !strings.Contains(body, "**HP** 4") {
		t.Fatalf("expected printed HP:\n%s", body)
	}
}

func testFetcher() MapFetcher {
	return MapFetcher{
		"data/adventures.json":     []byte(`{"adventure":[{"id":"LMoP","name":"Lost Mine of Phandelver","group":"supplement","published":"2014-07-15"}]}`),
		"data/books.json":          []byte(`{"book":[{"id":"XMM","name":"Monster Manual (2025)","group":"core","published":"2025-02-18"}]}`),
		"data/bestiary/index.json": []byte(`{"LMoP":"bestiary-lmop.json","XMM":"bestiary-xmm.json"}`),
		"data/spells/index.json":   []byte(`{}`),
		"data/bestiary/bestiary-xmm.json": []byte(`{"monster":[{
			"name":"Goblin Warrior","source":"XMM","size":["S"],
			"type":{"type":"fey","tags":["goblinoid"]},
			"ac":[15],"hp":{"average":10,"formula":"3d6"},
			"str":8,"dex":15,"con":10,"int":10,"wis":8,"cha":8,
			"speed":{"walk":30},"cr":"1/4",
			"action":[{"name":"Scimitar","entries":["{@atk mw} {@hit 4} to hit. {@h}{@damage 1d6 + 2} Slashing."]}]
		}]}`),
		"data/bestiary/bestiary-lmop.json": []byte(`{"monster":[{
			"name":"Ash Zombie","source":"LMoP","size":["M"],"type":"undead",
			"ac":[8],"hp":{"average":22,"formula":"3d8 + 9"},
			"str":13,"dex":6,"con":16,"int":3,"wis":6,"cha":5,"cr":"1/4"
		}]}`),
		"data/adventure/adventure-lmop.json": []byte(`{"data":[{
			"type":"section","name":"Phandalin",
			"entries":[
				"A frontier town.",
				{"type":"entries","name":"Important NPCs","entries":[
					{"type":"table","colLabels":["Name","Role"],"rows":[
						["{@creature Toblen Stonehill|LMoP}","Innkeeper."],
						["{@creature Sildar Hallwinter|LMoP}","Knight."]
					]}
				]},
				{"type":"entries","name":"1. Stonehill Inn","entries":[
					"A stout inn.",
					{"type":"entries","name":"Toblen Stonehill","entries":["He greets guests at the bar."]}
				]},
				{"type":"entries","name":"10. Shrine of Luck","entries":["A small shrine."]}
			]
		},{
			"type":"section","name":"Goblin Arrows",
			"entries":[
				"The wagon is ambushed by {@creature goblin|MM|goblins}.",
				{"type":"entries","name":"1. Cave Mouth","entries":["Two {@creature goblin|MM|goblins} watch the cave."]}
			]
		}]}`),
		"data/book/book-xmm.json": []byte(`{"data":[{"type":"section","name":"How to Use a Monster","entries":["Read the stat block."]}]}`),
	}
}

func TestBuildXMMCreatesLibraryCreatureWithStats(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/book.html#xmm,-1")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Doc.ID != "src-5e-xmm" || bundle.Doc.Kind != domain.SourceBestiary {
		t.Fatalf("doc=%#v", bundle.Doc)
	}
	found := false
	for _, d := range bundle.Records {
		if d.Title == "Goblin Warrior" && d.Type == domain.Creature && strings.Contains(d.Body, "**AC** 15") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing goblin warrior stats: %#v", titles(bundle))
	}
}

func TestBuildLMoPCreatesAdventureMentionsAndPrep(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/adventure.html#lmop,-1")
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
	if !strings.Contains(cave, "@goblin") {
		t.Fatalf("expected @ mention in room text: %q", cave)
	}
	if len(bundle.Plans) == 0 {
		t.Fatal("expected prep notes for adventure parts")
	}
}

func TestConvertAdventureNestsRoomsAndDedupsNPCs(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/adventure.html#lmop,-1")
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
	if found["Phandalin"] == nil || found["Phandalin"].typ != domain.Location {
		t.Fatalf("expected Phandalin location, got %#v", titles(bundle))
	}
	if strings.Contains(found["Phandalin"].body, "A stout inn") {
		t.Fatalf("parent should not copy child rooms: %q", found["Phandalin"].body)
	}
	if found["1. Stonehill Inn"] == nil || found["1. Stonehill Inn"].group != "Phandalin" {
		t.Fatalf("room should nest under Phandalin, got %#v", found["1. Stonehill Inn"])
	}
	if found["10. Shrine of Luck"] == nil || found["10. Shrine of Luck"].group != "Phandalin" {
		t.Fatalf("shrine should nest under Phandalin")
	}
	if found["Phandalin — 1. Stonehill Inn"] != nil || found["Phandalin - 1"] != nil {
		t.Fatalf("prefixed sibling titles still present: %#v", titles(bundle))
	}
	toblen := found["Toblen Stonehill"]
	if toblen == nil || toblen.typ != domain.NPC {
		t.Fatalf("expected one Toblen NPC, got %#v", found["Toblen Stonehill"])
	}
	if toblen.count != 1 {
		t.Fatalf("Toblen duplicated %d times", toblen.count)
	}
	if strings.Contains(toblen.body, "Sildar Hallwinter") {
		t.Fatalf("NPC should not copy the whole table: %q", toblen.body)
	}
	if !strings.Contains(toblen.body, "greets guests") && !strings.Contains(toblen.body, "Innkeeper") {
		t.Fatalf("Toblen should keep his own text: %q", toblen.body)
	}
	if found["1. Cave Mouth"] == nil || found["1. Cave Mouth"].group != "Goblin Arrows" {
		t.Fatalf("cave group=%#v", found["1. Cave Mouth"])
	}
}

func titles(b Bundle) []string {
	out := make([]string, 0, len(b.Records))
	for _, d := range b.Records {
		out = append(out, d.Title)
	}
	return out
}

func TestBuildXMMSkipsUnrelatedSharedFiles(t *testing.T) {
	inner := testFetcher()
	rec := &recordingFetcher{inner: inner}
	if _, err := Build(rec, "https://5e.tools/book.html#xmm,-1"); err != nil {
		t.Fatal(err)
	}
	for _, path := range rec.paths {
		if path == "data/items.json" || strings.HasPrefix(path, "data/class/") {
			t.Fatalf("XMM ingest should not fetch %s; got %v", path, rec.paths)
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
	if len(FilterCatalog(entries, "phandelver")) != 1 {
		t.Fatalf("filter=%#v", entries)
	}
}
