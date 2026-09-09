package fivetools

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

func TestHTTPFetcherRejectsOversizedContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(MaxResponseBytes+1, 10))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, err := (HTTPFetcher{BaseURL: server.URL, Client: server.Client()}).Get("data/test.json")
	if err == nil || !strings.Contains(err.Error(), "exceeds 64 MiB") {
		t.Fatalf("Get error = %v", err)
	}
}

func TestHTTPFetcherRejectsOversizedStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "-1")
		_, _ = w.Write(make([]byte, MaxResponseBytes+1))
	}))
	defer server.Close()
	if _, err := (HTTPFetcher{BaseURL: server.URL, Client: server.Client()}).Get("data/test.json"); err == nil {
		t.Fatal("expected oversized response error")
	}
}

func TestDirFetcherReadsAndReportsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "small.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := (DirFetcher{Root: dir}).Get("small.json")
	if err != nil || string(data) != `{}` {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if _, err := (DirFetcher{Root: dir}).Get("missing.json"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing error = %v", err)
	}
}

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
	multi := renderTags(`two {@creature cave rascal|BST|cave rascals} watch`)
	if !strings.Contains(multi, "@Cave Rascal") {
		t.Fatalf("multi-word mention: %q", multi)
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

func TestBuildRejectsMechanicalBooks(t *testing.T) {
	_, err := Build(testFetcher(), "https://5e.tools/book.html#bst,-1")
	if err == nil || !strings.Contains(err.Error(), "adventures") {
		t.Fatalf("expected rules-book rejection, got %v", err)
	}
}

func TestBuildADVCachesAdventureWithoutWikiRows(t *testing.T) {
	bundle, err := Build(testFetcher(), "https://5e.tools/adventure.html#adv,-1")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Doc.Kind != domain.SourceAdventure || !bundle.CacheOnly {
		t.Fatalf("doc=%#v cache=%v", bundle.Doc, bundle.CacheOnly)
	}
	if len(bundle.Records) != 0 || len(bundle.Plans) != 0 {
		t.Fatalf("adventure must not become wiki rows: records=%d plans=%d", len(bundle.Records), len(bundle.Plans))
	}
	book, err := LoadAdventure(testFetcher(), "ADV", bundle.Doc.Title)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(book.Body, "Millhaven") || !strings.Contains(book.Body, "@Cave Rascal") {
		t.Fatalf("reader body missing reference text:\n%s", book.Body)
	}
	if len(book.TOC) == 0 {
		t.Fatal("expected table of contents")
	}
	hit, ok := book.Lookup("Mira Holt")
	if !ok || hit.Name != "Mira Holt" || hit.Chapter != "Millhaven" {
		t.Fatalf("lookup Mira Holt: %#v ok=%v", hit, ok)
	}
}

func TestParseAdventureIndexesHeadingsAndCreatures(t *testing.T) {
	data, err := testFetcher().Get("data/adventure/adventure-adv.json")
	if err != nil {
		t.Fatal(err)
	}
	book, err := ParseAdventure(data)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(book.TOC, "\n")
	if !strings.Contains(joined, "Millhaven") || !strings.Contains(joined, "The Ambush") {
		t.Fatalf("toc=%q", joined)
	}
	if _, ok := book.Lookup("1. Cave Mouth"); !ok {
		t.Fatal("expected cave mouth heading")
	}
	if _, ok := book.Lookup("Cave Rascal"); !ok {
		t.Fatal("expected creature mention")
	}
}

func TestParseAdventureMergesSharedBodies(t *testing.T) {
	data, err := testFetcher().Get("data/adventure/adventure-adv.json")
	if err != nil {
		t.Fatal(err)
	}
	book, err := ParseAdventure(data)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, hit := range book.Hits {
		key := documentKey(hit.Body)
		if key == "" {
			continue
		}
		if other, ok := seen[key]; ok {
			t.Fatalf("same document listed as %q and %q", other, hit.Name)
		}
		seen[key] = hit.Name
	}
	rascal, ok := book.Lookup("Cave Rascal")
	if !ok {
		t.Fatal("Cave Rascal should still resolve")
	}
	if rascal.Name == "Cave Rascal" {
		t.Fatalf("creature tag should alias onto a heading, got own row %#v", rascal)
	}
	mira, ok := book.Lookup("Mira Holt")
	if !ok || mira.Name != "Mira Holt" {
		t.Fatalf("Mira Holt has her own writeup, got %#v ok=%v", mira, ok)
	}
}

func TestConvertGenericAdventureIsReadableReference(t *testing.T) {
	fetcher := MapFetcher{
		"data/adventures.json": []byte(`{"adventure":[{"id":"Gate","name":"The Gatehouse Run","published":"2020-01-01"}]}`),
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
	if !bundle.CacheOnly || len(bundle.Records) != 0 {
		t.Fatalf("expected cache-only adventure, records=%d", len(bundle.Records))
	}
	book, err := LoadAdventure(fetcher, "Gate", "The Gatehouse Run")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := book.Lookup("Nilo Voss"); !ok {
		t.Fatalf("expected Nilo in index: %#v", names(book))
	}
	if _, ok := book.Lookup("1. Portcullis"); !ok {
		t.Fatal("expected portcullis heading")
	}
}

func names(b AdventureBook) []string {
	out := make([]string, 0, len(b.Hits))
	for _, h := range b.Hits {
		out = append(out, h.Name)
	}
	return out
}

type recordingFetcher struct {
	inner Fetcher
	paths []string
}

func (r *recordingFetcher) Get(path string) ([]byte, error) {
	r.paths = append(r.paths, path)
	return r.inner.Get(path)
}

func TestLoadCatalogListsAdventuresOnly(t *testing.T) {
	entries, err := LoadCatalog(testFetcher())
	if err != nil {
		t.Fatal(err)
	}
	if len(FilterCatalog(entries, "hollow")) != 1 {
		t.Fatalf("filter=%#v", entries)
	}
	if len(FilterCatalog(entries, "bestiary")) != 0 {
		t.Fatalf("catalog should not list rules books: %#v", entries)
	}
}
