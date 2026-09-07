package tui

import (
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
)

func TestAdventurePeekResolvesEnabledSourceWithoutWikiRecord(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources, domain.SourceDocument{
		ID: "src-5e-adv", Title: "The Hollow Crown", Kind: domain.SourceAdventure, Edition: "2014",
	})
	ws.EnableSource(ws.Scope.WorldID, ws.Scope.CampaignID, "src-5e-adv")
	ws.Records = append(ws.Records, domain.Record{
		ID: "inn", Type: domain.Location, Title: "The Mill Inn", Authority: domain.Canon, Scope: ws.Scope,
		Body: "Meet @Mira Holt at the bar.\n",
	})
	model := newModel(ws, nil, nil)
	model.toolsFetcher = adventureTestFetcher()
	model.attachReferences()

	line := "Meet @Mira Holt at the bar."
	peek := model.resolveReferenceAtCursor(line, 8)
	if peek == nil || peek.Title != "Mira Holt" || peek.Source != "The Hollow Crown" {
		t.Fatalf("peek=%#v", peek)
	}
	if !fivetools.IsReferenceID(peek.ID) {
		t.Fatalf("expected reference id, got %q", peek.ID)
	}
	if !strings.Contains(peek.Body, "bar") && !strings.Contains(peek.Summary, "Mira") {
		t.Fatalf("expected adventure text:\n%#v", peek)
	}

	mentions := model.resolveMentions(line)
	if len(mentions) != 1 || mentions[0].RecordID == "" {
		t.Fatalf("mentions=%#v", mentions)
	}
}

func TestAdventurePeekRequiresEnabledSource(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources, domain.SourceDocument{
		ID: "src-5e-adv", Title: "The Hollow Crown", Kind: domain.SourceAdventure, Edition: "2014",
	})
	ws.Records = append(ws.Records, domain.Record{
		ID: "inn", Type: domain.Location, Title: "The Mill Inn", Authority: domain.Canon, Scope: ws.Scope,
		Body: "Meet @Mira Holt at the bar.\n",
	})
	model := newModel(ws, nil, nil)
	model.toolsFetcher = adventureTestFetcher()
	model.attachReferences()

	line := "Meet @Mira Holt at the bar."
	if peek := model.resolveReferenceAtCursor(line, 8); peek != nil {
		t.Fatalf("disabled adventure must not peek, peek=%#v", peek)
	}
}

func TestWikiPeekWinsOverAdventureReference(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources, domain.SourceDocument{
		ID: "src-5e-adv", Title: "The Hollow Crown", Kind: domain.SourceAdventure, Edition: "2014",
	})
	ws.EnableSource(ws.Scope.WorldID, ws.Scope.CampaignID, "src-5e-adv")
	ws.Records = append(ws.Records, domain.Record{
		ID: "mira-wiki", Type: domain.NPC, Title: "Mira Holt", Authority: domain.Canon, Scope: ws.Scope,
		Summary: "The campaign's innkeeper.",
		Body:    "She is ours.",
	})
	model := newModel(ws, nil, nil)
	model.toolsFetcher = adventureTestFetcher()
	model.attachReferences()

	line := "Meet @Mira Holt at the bar."
	peek := model.resolveReferenceAtCursor(line, 8)
	if peek == nil || peek.ID != "mira-wiki" {
		t.Fatalf("wiki must win name collision, peek=%#v", peek)
	}
	if fivetools.IsReferenceID(peek.ID) {
		t.Fatal("wiki peek must not be a reference id")
	}
}

func TestSourcesReaderShowsCachedAdventure(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources, domain.SourceDocument{
		ID: "src-5e-adv", Title: "The Hollow Crown", Kind: domain.SourceAdventure, Edition: "2014",
	})
	ws.EnableSource(ws.Scope.WorldID, ws.Scope.CampaignID, "src-5e-adv")
	model := newModel(ws, nil, nil)
	model.width = 100
	model.height = 32
	model.toolsFetcher = adventureTestFetcher()
	model.attachReferences()
	model.setNavCursor(2) // Sources

	view := model.View().Content
	if !strings.Contains(view, "Sources") || !strings.Contains(view, "The Hollow Crown") {
		t.Fatalf("expected Sources title: %q", view)
	}
	if !strings.Contains(view, "SOURCE") || !strings.Contains(view, "reference") {
		t.Fatalf("expected adventure reader: %q", view)
	}
	if !strings.Contains(view, "Millhaven") || !strings.Contains(view, "Mira Holt") {
		t.Fatalf("expected named hits in the Sources list: %q", view)
	}
}

func TestSourcesReaderRendersSelectedHitNotWholeBook(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources, domain.SourceDocument{
		ID: "src-5e-adv", Title: "The Hollow Crown", Kind: domain.SourceAdventure, Edition: "2014",
	})
	ws.EnableSource(ws.Scope.WorldID, ws.Scope.CampaignID, "src-5e-adv")
	model := newModel(ws, nil, nil)
	model.width = 100
	model.height = 32
	model.toolsFetcher = adventureTestFetcher()
	model.attachReferences()
	model.setNavCursor(2)

	mira := -1
	ambush := -1
	for index, row := range model.sourceListRows() {
		if row.IsHit && row.Hit.Name == "Mira Holt" {
			mira = index
		}
		if row.IsHit && row.Hit.Name == "The Ambush" {
			ambush = index
		}
	}
	if mira < 0 || ambush < 0 {
		t.Fatalf("expected Mira Holt and The Ambush hits, got %#v", model.sourceListRows())
	}

	model.cursor = mira
	model.bindSourceListRow(model.sourceListRows()[mira])
	detail := model.renderTreeDetail()
	if !strings.Contains(detail, "Mira Holt") || !strings.Contains(detail, "bar") {
		t.Fatalf("expected Mira slice:\n%s", detail)
	}
	if strings.Contains(detail, "ZZZAMBUSHZZZ") {
		t.Fatalf("Mira slice must not Glamour the rest of the book:\n%s", detail)
	}

	model.cursor = ambush
	model.bindSourceListRow(model.sourceListRows()[ambush])
	detail = model.renderTreeDetail()
	if !strings.Contains(detail, "ZZZAMBUSHZZZ") {
		t.Fatalf("expected Ambush slice:\n%s", detail)
	}
}

func adventureTestFetcher() fivetools.MapFetcher {
	return fivetools.MapFetcher{
		"data/adventures.json": []byte(`{"adventure":[{"id":"ADV","name":"The Hollow Crown","published":"2020-01-01"}]}`),
		"data/adventure/adventure-adv.json": []byte(`{"data":[{
			"type":"section","name":"Millhaven",
			"entries":[
				"A frontier town.",
				{"type":"entries","name":"Mira Holt","entries":["She greets guests at the bar."]}
			]
		},{
			"type":"section","name":"The Ambush",
			"entries":["ZZZAMBUSHZZZ wagon raid."]
		}]}`),
	}
}
