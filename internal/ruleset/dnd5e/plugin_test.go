package dnd5e

import (
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	"github.com/hbaldwin98/dungeon/internal/ruleset"
)

func TestOpenWithoutBooksReturnsNil(t *testing.T) {
	plugin, err := Open(pluginTestFetcher(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plugin != nil {
		t.Fatal("empty book list must not attach the 5e plugin")
	}
}

func TestOpenDoesNotAttachCoreBooksUntilImported(t *testing.T) {
	plugin, err := Open(pluginTestFetcher(), Options{Books: []string{"ADV"}})
	if err != nil {
		t.Fatal(err)
	}
	if plugin == nil {
		t.Fatal("adventure book should still open a scoped plugin")
	}
	if _, ok := plugin.LookupName("cave rascal"); ok {
		t.Fatal("MM creature must not resolve from an adventure-only import")
	}
	if _, ok := plugin.LookupName("Spark Bolt"); ok {
		t.Fatal("PHB spell must not resolve from an adventure-only import")
	}
	if _, ok := plugin.LookupName("Cave Rascal|MM"); ok {
		t.Fatal("explicit MM source must still be gated")
	}
	wight, ok := plugin.LookupName("Marsh Wight")
	if !ok || wight.Source != "ADV" || wight.Kind != ruleset.Creature {
		t.Fatalf("adventure creature=%#v ok=%v", wight, ok)
	}
}

func TestOpenLooksUpImportedCoreBooks(t *testing.T) {
	plugin, err := Open(pluginTestFetcher(), Options{Books: []string{"MM", "PHB"}})
	if err != nil {
		t.Fatal(err)
	}
	ent, ok := plugin.LookupName("cave rascal")
	if !ok {
		t.Fatal("imported MM should resolve Cave Rascal")
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
	spell, ok := plugin.LookupName("Spark Bolt")
	if !ok || spell.Kind != ruleset.Spell || spell.Source != "PHB" {
		t.Fatalf("spell=%#v ok=%v", spell, ok)
	}
	if _, ok := plugin.LookupName("Marsh Wight"); ok {
		t.Fatal("ADV creature must not resolve when only MM/PHB are enabled")
	}
}

func pluginTestFetcher() fivetools.MapFetcher {
	return fivetools.MapFetcher{
		"data/bestiary/index.json":    []byte(`{"ADV":"bestiary-adv.json","MM":"bestiary-mm.json"}`),
		"data/spells/index.json":      []byte(`{"PHB":"spells-phb.json"}`),
		"data/spells/spells-phb.json": []byte(`{"spell":[{"name":"Spark Bolt","source":"PHB","level":0,"school":"V","entries":["A spark leaps to a target."]}]}`),
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
