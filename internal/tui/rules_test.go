package tui

import (
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
)

func TestRulesetPeekResolvesPluginCreatureWithoutWikiRecord(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources, domain.SourceDocument{
		ID: "src-5e-mm", Title: "Monster Manual", Kind: domain.SourceBestiary, Edition: "2014",
	})
	ws.EnableSource(ws.Scope.WorldID, ws.Scope.CampaignID, "src-5e-mm")
	ws.Records = append(ws.Records, domain.Record{
		ID: "cave", Type: domain.Location, Title: "Cave Mouth", Authority: domain.Canon, Scope: ws.Scope,
		Body: "Two @Cave Rascal watch the cave.\n",
	})
	model := newModel(ws, nil, nil)
	model.toolsFetcher = rulesetTestFetcher()
	model.attachRules()

	line := "Two @Cave Rascal watch the cave."
	peek := model.resolveReferenceAtCursor(line, 8)
	if peek == nil || peek.Title != "Cave Rascal" || peek.Source != "MM" {
		t.Fatalf("peek=%#v", peek)
	}
	if !strings.Contains(peek.Body, "**AC** 12") {
		t.Fatalf("expected composed stats:\n%s", peek.Body)
	}

	mentions := model.resolveMentions(line)
	if len(mentions) != 1 || mentions[0].RecordID == "" {
		t.Fatalf("mentions=%#v", mentions)
	}
}

func TestRulesetPeekRequiresEnabledPluginSource(t *testing.T) {
	ws := demoWorkspace()
	ws.EnsureLibrary()
	ws.Sources = append(ws.Sources,
		domain.SourceDocument{ID: "src-5e-adv", Title: "The Hollow Crown", Kind: domain.SourceAdventure, Edition: "2014"},
		domain.SourceDocument{ID: "src-5e-mm", Title: "Monster Manual", Kind: domain.SourceBestiary, Edition: "2014"},
	)
	ws.EnableSource(ws.Scope.WorldID, ws.Scope.CampaignID, "src-5e-adv")
	ws.Records = append(ws.Records, domain.Record{
		ID: "cave", Type: domain.Location, Title: "Cave Mouth", Authority: domain.Canon, Scope: ws.Scope,
		Body: "Two @Cave Rascal watch the cave.\n",
	})
	model := newModel(ws, nil, nil)
	model.toolsFetcher = rulesetTestFetcher()
	model.attachRules()

	line := "Two @Cave Rascal watch the cave."
	if peek := model.resolveReferenceAtCursor(line, 8); peek != nil {
		t.Fatalf("adventure enablement must not attach MM lookups, peek=%#v", peek)
	}
}

func rulesetTestFetcher() fivetools.MapFetcher {
	return fivetools.MapFetcher{
		"data/bestiary/index.json": []byte(`{"ADV":"bestiary-adv.json","MM":"bestiary-mm.json"}`),
		"data/spells/index.json":   []byte(`{}`),
		"data/bestiary/bestiary-mm.json": []byte(`{"monster":[{
			"name":"Cave Rascal","source":"MM","size":["S"],"type":"humanoid",
			"ac":[12],"hp":{"average":9},"cr":"1/8"
		}]}`),
		"data/bestiary/fluff-bestiary-mm.json": []byte(`{"monsterFluff":[{"name":"Cave Rascal","source":"MM","entries":["A cave rascal."]}]}`),
		"data/bestiary/bestiary-adv.json":      []byte(`{"monster":[]}`),
		"data/bestiary/template.json":          []byte(`{"monsterTemplate":[]}`),
		"data/bestiary/legendarygroups.json":   []byte(`{"legendaryGroup":[]}`),
		"data/items.json":                      []byte(`{"item":[]}`),
		"data/items-base.json":                 []byte(`{"baseitem":[]}`),
		"data/magicvariants.json":              []byte(`{"magicvariant":[]}`),
	}
}
