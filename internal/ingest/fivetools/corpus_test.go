package fivetools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pluginFetcher() MapFetcher {
	m := testFetcher()
	m["data/bestiary/index.json"] = []byte(`{"ADV":"bestiary-adv.json","BST":"bestiary-bst.json","MM":"bestiary-mm.json"}`)
	m["data/spells/index.json"] = []byte(`{"PHB":"spells-phb.json"}`)
	m["data/spells/spells-phb.json"] = []byte(`{"spell":[{"name":"Spark Bolt","source":"PHB","level":0,"school":"V","entries":["A spark leaps to a target."]}]}`)
	m["data/conditionsdiseases.json"] = []byte(`{"condition":[{"name":"Muddy","source":"PHB","entries":["A muddy creature has trouble keeping its footing."]}]}`)
	m["data/actions.json"] = []byte(`{"action":[{"name":"Dash","source":"PHB","entries":["You gain extra movement this turn."]}]}`)
	m["data/bestiary/bestiary-mm.json"] = []byte(`{"monster":[{
		"name":"Cave Rascal","source":"MM","size":["S"],"type":"humanoid",
		"ac":[12],"hp":{"average":9,"formula":"2d6"},
		"str":9,"dex":13,"con":11,"int":8,"wis":10,"cha":7,
		"speed":{"walk":30},"cr":"1/8",
		"action":[{"name":"Shortblade","entries":["{@atk mw} {@hit 3} to hit. {@h}{@damage 1d4 + 1} piercing."]}]
	},{
		"name":"Townsfolk","source":"MM","size":["M"],"type":"humanoid",
		"ac":[10],"hp":{"average":4,"formula":"1d8"},
		"str":10,"dex":10,"con":10,"int":10,"wis":10,"cha":10,"cr":"0"
	},{
		"name":"Marsh Wyrm","source":"MM","size":["H"],"type":"dragon",
		"ac":[16],"hp":{"average":88},"cr":"6",
		"legendaryGroup":{"name":"Marsh Wyrm","source":"MM"}
	}]}`)
	m["data/bestiary/fluff-bestiary-mm.json"] = []byte(`{"monsterFluff":[{
		"name":"Cave Rascal","source":"MM","entries":["Cave rascals raid from hidden dens."]
	}]}`)
	m["data/bestiary/template.json"] = []byte(`{"monsterTemplate":[{
		"name":"Hill Folk","source":"TST",
		"apply":{
			"_root":{"speed":{"walk":25}},
			"_mod":{"trait":{"mode":"appendArr","items":[{"name":"Hill Endurance","entries":["<$title_short_name$> shrugs off exhaustion."]}]}}
		}
	}]}`)
	m["data/bestiary/legendarygroups.json"] = []byte(`{"legendaryGroup":[{
		"name":"Marsh Wyrm","source":"MM",
		"lairActions":["The marsh floods with silt."],
		"regionalEffects":["Water within 6 miles tastes of peat."]
	}]}`)
	m["data/items.json"] = []byte(`{"item":[{"name":"Pocket Satchel","source":"TST","rarity":"uncommon","wondrous":true,"entries":["A pocket satchel opens into a hidden pocket."]}]}`)
	m["data/items-base.json"] = []byte(`{"baseitem":[{"name":"Longblade","source":"TST","type":"M","rarity":"none","weight":3}]}`)
	m["data/magicvariants.json"] = []byte(`{"magicvariant":[{
		"name":"Blade of Warning","source":"TST","type":"GV",
		"inherits":{"nameSuffix":" of Warning","rarity":"uncommon","reqAttune":true,"entries":["The weapon warns you of danger."]}
	}]}`)
	m["data/bestiary/bestiary-adv.json"] = []byte(`{"monster":[{
		"name":"Marsh Wight","source":"ADV","size":["M"],"type":"undead",
		"ac":[11],"hp":{"average":13,"formula":"3d8"},
		"str":12,"dex":8,"con":13,"int":6,"wis":9,"cha":6,"cr":"1/4"
	},{
		"name":"Reed Holt","source":"ADV","isNpc":true,
		"_copy":{"name":"Townsfolk","source":"MM","_templates":[{"name":"Hill Folk","source":"TST"}]}
	}]}`)
	return m
}

func TestCatalogLooksUpCoreCreatureWithoutWikiRecords(t *testing.T) {
	c := NewCatalog(pluginFetcher())
	if err := c.loadIndexes(); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadCompose(); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadBestiary("MM"); err != nil {
		t.Fatal(err)
	}
	hit, ok := c.LookupName("cave rascal")
	if !ok {
		t.Fatal("expected cave rascal")
	}
	if hit.Source != "MM" || hit.Kind != KindCreature {
		t.Fatalf("hit=%#v", hit)
	}
	if !strings.Contains(hit.Body, "**AC** 12") || !strings.Contains(hit.Body, "Cave rascals raid") {
		t.Fatalf("body:\n%s", hit.Body)
	}
}

func TestCatalogAppliesCopyTemplateAndLegendaryGroup(t *testing.T) {
	c := NewCatalog(pluginFetcher())
	if err := c.loadIndexes(); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadCompose(); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadBestiary("ADV"); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadBestiary("MM"); err != nil {
		t.Fatal(err)
	}
	reed, ok := c.LookupName("Reed Holt")
	if !ok {
		t.Fatal("expected Reed Holt")
	}
	if !strings.Contains(reed.Body, "**HP** 4") {
		t.Fatalf("copy stats missing:\n%s", reed.Body)
	}
	if !strings.Contains(reed.Body, "Hill Endurance") {
		t.Fatalf("template missing:\n%s", reed.Body)
	}
	wyrm, ok := c.LookupName("Marsh Wyrm")
	if !ok {
		t.Fatal("expected marsh wyrm")
	}
	if !strings.Contains(wyrm.Body, "The marsh floods") {
		t.Fatalf("legendary group missing:\n%s", wyrm.Body)
	}
}

func TestCatalogLooksUpItemsAndVariants(t *testing.T) {
	c := NewCatalog(pluginFetcher())
	if err := c.LoadItems(); err != nil {
		t.Fatal(err)
	}
	bag, ok := c.LookupName("Pocket Satchel")
	if !ok || bag.Kind != KindItem || !strings.Contains(bag.Body, "hidden pocket") {
		t.Fatalf("bag=%#v ok=%v", bag, ok)
	}
	sword, ok := c.LookupName("Longblade")
	if !ok || !strings.Contains(sword.Body, "Longblade") {
		t.Fatalf("sword=%#v ok=%v", sword, ok)
	}
	warn, ok := c.LookupName("Blade of Warning")
	if !ok || !strings.Contains(warn.Body, "warns you") {
		t.Fatalf("variant=%#v ok=%v", warn, ok)
	}
}

func TestCatalogLooksUpSpellsAndTerms(t *testing.T) {
	c := NewCatalog(pluginFetcher())
	if err := c.LoadSpells("PHB"); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadTerms(); err != nil {
		t.Fatal(err)
	}
	spell, ok := c.LookupName("Spark Bolt")
	if !ok || spell.Kind != KindSpell || !strings.Contains(spell.Body, "spark leaps") {
		t.Fatalf("spell=%#v ok=%v", spell, ok)
	}
	term, ok := c.LookupName("Muddy")
	if !ok || term.Kind != KindTerm || !strings.Contains(term.Body, "footing") {
		t.Fatalf("term=%#v ok=%v", term, ok)
	}
}

func TestCacheFetcherReadsAfterWrite(t *testing.T) {
	dir := t.TempDir()
	inner := pluginFetcher()
	cached := NewCacheFetcher(inner, dir)
	data, err := cached.Get("data/items.json")
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(dir, "data", "items.json")
	if _, err := os.Stat(disk); err != nil {
		t.Fatal(err)
	}
	only := CacheOnly(dir)
	again, err := only.Get("data/items.json")
	if err != nil || string(again) != string(data) {
		t.Fatalf("cache-only: %v %q", err, again)
	}
}

func TestCacheFetcherReportsUnavailableCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dir, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	cached := NewCacheFetcher(MapFetcher{"data/items.json": []byte(`{"item":[]}`)}, dir)
	if _, err := cached.Get("data/items.json"); err == nil {
		t.Fatalf("Get error = %v", err)
	}
}

func TestCacheOnlyRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "items.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"item":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CacheOnly(dir).Get("data/items.json"); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("Get error = %v", err)
	}
}

func TestCacheFetcherRejectsMalformedCachedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data", "items.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"item":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCacheFetcher(MapFetcher{}, dir).Get("data/items.json"); err == nil {
		t.Fatal("expected invalid cache error")
	}
}

func TestCatalogAllowedSourcesHideCoreLookups(t *testing.T) {
	c := NewCatalog(pluginFetcher())
	if err := c.LoadCompose(); err != nil {
		t.Fatal(err)
	}
	if err := c.LoadBestiary("ADV"); err != nil {
		t.Fatal(err)
	}
	c.SetAllowed([]string{"ADV"})
	c.SetPreferred([]string{"ADV"})
	if _, ok := c.LookupName("Cave Rascal"); ok {
		t.Fatal("MM creature must not resolve until MM is allowed")
	}
	reed, ok := c.LookupName("Reed Holt")
	if !ok {
		t.Fatal("adventure NPC should still compose from a copied MM parent")
	}
	if !strings.Contains(reed.Body, "**HP** 4") {
		t.Fatalf("copy stats missing:\n%s", reed.Body)
	}
	if _, ok := c.LookupName("Cave Rascal"); ok {
		t.Fatal("composing an adventure copy must not expose MM lookups")
	}
}

func TestPrimeDoesNotFetchCoreBooksForAdventure(t *testing.T) {
	rec := &recordingFetcher{inner: pluginFetcher()}
	if err := Prime(rec, Entry{ID: "ADV", Name: "The Hollow Crown", Kind: "adventure"}); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, path := range rec.paths {
		got[path] = true
	}
	for _, path := range []string{
		"data/bestiary/bestiary-mm.json",
		"data/spells/spells-phb.json",
		"data/items.json",
		"data/conditionsdiseases.json",
	} {
		if got[path] {
			t.Fatalf("adventure prime fetched %s", path)
		}
	}
	if !got["data/bestiary/bestiary-adv.json"] {
		t.Fatal("adventure prime should fetch the adventure bestiary")
	}
}

func TestPluginRecordIDIsStable(t *testing.T) {
	hit := Hit{Kind: KindCreature, Name: "Cave Rascal", Source: "MM", Body: "stats"}
	rec := hit.Record()
	if rec.ID != "ruleset:dnd5e:creature:mm:cave-rascal" {
		t.Fatalf("id=%q", rec.ID)
	}
	if !IsPluginID(rec.ID) {
		t.Fatal("expected plugin id")
	}
}
