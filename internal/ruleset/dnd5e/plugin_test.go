package dnd5e

import (
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	"github.com/hbaldwin98/dungeon/internal/ruleset"
)

func TestOpenLooksUpCoreCreatureFromAdventureEnablement(t *testing.T) {
	plugin, err := Open(pluginTestFetcher(), Options{Books: []string{"ADV"}})
	if err != nil {
		t.Fatal(err)
	}
	ent, ok := plugin.LookupName("cave rascal")
	if !ok {
		t.Fatal("adventure enablement should load the core bestiary into the plugin")
	}
	if ent.Source != "MM" || ent.Kind != ruleset.Creature {
		t.Fatalf("ent=%#v", ent)
	}
	if !strings.Contains(ent.Body, "**AC** 12") {
		t.Fatalf("body:\n%s", ent.Body)
	}
	rec, ok := plugin.Record(ent.Record.ID)
	if !ok || rec.Title != "Cave Rascal" {
		t.Fatalf("record round-trip: ok=%v %#v", ok, rec)
	}
}

func pluginTestFetcher() fivetools.MapFetcher {
	return fivetools.MapFetcher{
		"data/bestiary/index.json": []byte(`{"ADV":"bestiary-adv.json","MM":"bestiary-mm.json"}`),
		"data/spells/index.json":   []byte(`{}`),
		"data/bestiary/bestiary-mm.json": []byte(`{"monster":[{
			"name":"Cave Rascal","source":"MM","size":["S"],"type":"humanoid",
			"ac":[12],"hp":{"average":9},"cr":"1/8"
		}]}`),
		"data/bestiary/fluff-bestiary-mm.json": []byte(`{"monsterFluff":[{"name":"Cave Rascal","source":"MM","entries":["A cave rascal."]}]}`),
		"data/bestiary/bestiary-adv.json":      []byte(`{"monster":[{"name":"Marsh Wight","source":"ADV","ac":[11],"hp":{"average":13}}]}`),
		"data/bestiary/template.json":          []byte(`{"monsterTemplate":[]}`),
		"data/bestiary/legendarygroups.json":   []byte(`{"legendaryGroup":[]}`),
		"data/items.json":                      []byte(`{"item":[{"name":"Pocket Satchel","source":"TST","entries":["A bag."]}]}`),
		"data/items-base.json":                 []byte(`{"baseitem":[]}`),
		"data/magicvariants.json":              []byte(`{"magicvariant":[]}`),
	}
}
