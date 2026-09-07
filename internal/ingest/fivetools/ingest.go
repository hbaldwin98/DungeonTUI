package fivetools

import (
	"fmt"
	"strings"
	"time"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

// Bundle is one chosen 5e.tools adventure. CacheOnly books write no wiki rows;
// JSON stays in the local 5e.tools cache for the Sources reader and @ peeks.
type Bundle struct {
	Entry      Entry
	Doc        domain.SourceDocument
	Records    []draft
	Plans      []planDraft
	PluginOnly bool // unused: mechanical books are not imported
	CacheOnly  bool // adventure JSON is cached for the reader; no wiki rows
}

// Catalog reads a local 5e.tools corpus. The durable copy is JSON pulled at
// ingest onto the owner's machine (CacheFetcher). Adventure ingest caches that
// JSON for the reader; it does not copy published prose into Workspace.Records.
type Catalog struct {
	fetcher       Fetcher
	cache         map[string][]byte
	bestiaryIndex map[string]string
	spellIndex    map[string]string
	monsters      map[string]map[string]any
	templates     map[string]map[string]any
	legendary     map[string]map[string]any
	items         map[string]map[string]any
	variants      map[string]map[string]any
	spells        map[string]map[string]any
	terms         map[string]map[string]any
	fluff         map[string]string
	preferred     []string
	allowed       []string
}

func NewCatalog(fetcher Fetcher) *Catalog {
	return &Catalog{
		fetcher:   fetcher,
		cache:     map[string][]byte{},
		monsters:  map[string]map[string]any{},
		templates: map[string]map[string]any{},
		legendary: map[string]map[string]any{},
		items:     map[string]map[string]any{},
		variants:  map[string]map[string]any{},
		spells:    map[string]map[string]any{},
		terms:     map[string]map[string]any{},
		fluff:     map[string]string{},
	}
}

func (c *Catalog) get(path string) ([]byte, bool, error) {
	if data, ok := c.cache[path]; ok {
		return data, true, nil
	}
	data, ok, err := getOptional(c.fetcher, path)
	if err != nil || !ok {
		return nil, ok, err
	}
	c.cache[path] = data
	return data, true, nil
}

func (c *Catalog) loadIndexes() error {
	if data, ok, err := c.get("data/bestiary/index.json"); err != nil {
		return err
	} else if ok {
		idx, err := jsonIndex(data)
		if err != nil {
			return fmt.Errorf("bestiary index: %w", err)
		}
		c.bestiaryIndex = idx
	}
	if data, ok, err := c.get("data/spells/index.json"); err != nil {
		return err
	} else if ok {
		idx, err := jsonIndex(data)
		if err != nil {
			return fmt.Errorf("spell index: %w", err)
		}
		c.spellIndex = idx
	}
	return nil
}

func indexLookup(index map[string]string, code string) string {
	if file, ok := index[code]; ok {
		return file
	}
	want := strings.ToLower(code)
	for key, file := range index {
		if strings.ToLower(key) == want {
			return file
		}
	}
	return ""
}

// Build fetches every 5e.tools collection that publishes the chosen source id
// and converts matching entities into drafts. It does not write files.
func Build(fetcher Fetcher, raw string) (Bundle, error) {
	ref, ok := ParseRef(raw)
	if !ok {
		return Bundle{}, fmt.Errorf("not a 5e.tools book or adventure: %q", raw)
	}
	if fetcher == nil {
		return Bundle{}, fmt.Errorf("5e.tools fetcher is required")
	}
	catalog, err := LoadCatalog(fetcher)
	if err != nil {
		return Bundle{}, err
	}
	entry, ok := ResolveEntry(catalog, ref)
	if !ok {
		entry = Entry{ID: ref.ID, Name: ref.ID, Kind: ref.Kind}
		if entry.Kind == "" {
			entry.Kind = "book"
		}
	}
	kind := entry.SourceKind()
	doc := domain.SourceDocument{
		ID:      SourceID(entry.ID),
		Title:   firstNonEmpty(entry.Name, entry.ID),
		Edition: entry.Edition(),
		Kind:    kind,
		Path:    firstNonEmpty(ref.Raw, entry.PageURL()),
	}
	if entry.Kind != "adventure" {
		return Bundle{}, fmt.Errorf("Dungeon stores 5e.tools adventures for reference, not rules books (%s)", entry.ID)
	}
	s := NewCatalog(fetcher)
	path := AdventureJSONPath(entry.ID)
	data, ok, err := s.get(path)
	if err != nil {
		return Bundle{}, err
	}
	if !ok {
		return Bundle{}, fmt.Errorf("no 5e.tools adventure published as %s", entry.ID)
	}
	if _, err := ParseAdventure(data); err != nil {
		return Bundle{}, err
	}
	doc.Kind = domain.SourceAdventure
	return Bundle{Entry: entry, Doc: doc, CacheOnly: true}, nil
}

func wantsClasses(entry Entry) bool {
	if entry.Kind == "adventure" || entry.SourceKind() == domain.SourceBestiary {
		return false
	}
	return looksLikeCharacterRules(entry)
}

func wantsShared(entry Entry, spec sharedSpec) bool {
	if spec.skipAdventure && entry.Kind == "adventure" {
		return false
	}
	if entry.SourceKind() == domain.SourceBestiary {
		return false
	}
	if entry.Kind == "adventure" {
		return false
	}
	if looksLikeCharacterRules(entry) {
		return true
	}
	if looksLikeTreasureRules(entry) {
		return spec.kind == "item" || spec.tag == "item" || spec.tag == "deck" || spec.tag == "hazard" || spec.tag == "object" || spec.tag == "vehicle" || spec.tag == "bastion"
	}
	return false
}

func looksLikeCharacterRules(entry Entry) bool {
	id := strings.ToUpper(entry.ID)
	name := strings.ToLower(entry.Name)
	switch id {
	case "PHB", "XPHB", "TCE", "XGE", "SCAG", "AAG", "FTD", "BMT", "GGR", "EGW", "AI":
		return true
	}
	if strings.Contains(name, "player") {
		return true
	}
	return strings.Contains(name, "handbook") && !strings.Contains(name, "dungeon master") && !strings.Contains(name, "monster")
}

func looksLikeTreasureRules(entry Entry) bool {
	id := strings.ToUpper(entry.ID)
	name := strings.ToLower(entry.Name)
	switch id {
	case "DMG", "XDMG", "TCE", "XGE":
		return true
	}
	return strings.Contains(name, "dungeon master") || strings.Contains(name, "treasure")
}

type sharedSpec struct {
	path          string
	keys          []string
	kind          string
	tag           string
	prefix        string
	skipAdventure bool
	title         func(map[string]any) string
}

func identityTitle(item map[string]any) string { return asString(item["name"]) }

func raceTitle(item map[string]any) string {
	name := asString(item["name"])
	race := asString(item["raceName"])
	if race != "" && name != "" {
		return race + " (" + name + ")"
	}
	return name
}

var sharedFiles = []sharedSpec{
	{path: "data/items.json", keys: []string{"item", "itemGroup"}, kind: "item", tag: "item", prefix: "item"},
	{path: "data/items-base.json", keys: []string{"baseitem", "item"}, kind: "item", tag: "item", prefix: "item"},
	{path: "data/magicvariants.json", keys: []string{"magicvariant"}, kind: "item", tag: "item", prefix: "item"},
	{path: "data/races.json", keys: []string{"race", "subrace"}, kind: "rule", tag: "species", prefix: "species", skipAdventure: true, title: raceTitle},
	{path: "data/feats.json", keys: []string{"feat"}, kind: "rule", tag: "feat", prefix: "feat", skipAdventure: true, title: identityTitle},
	{path: "data/backgrounds.json", keys: []string{"background"}, kind: "rule", tag: "background", prefix: "background", skipAdventure: true, title: identityTitle},
	{path: "data/conditionsdiseases.json", keys: []string{"condition", "disease", "status"}, kind: "rule", tag: "condition", prefix: "condition", skipAdventure: true, title: identityTitle},
	{path: "data/optionalfeatures.json", keys: []string{"optionalfeature"}, kind: "rule", tag: "option", prefix: "option", skipAdventure: true, title: identityTitle},
	{path: "data/rewards.json", keys: []string{"reward"}, kind: "rule", tag: "reward", prefix: "reward", skipAdventure: true, title: identityTitle},
	{path: "data/objects.json", keys: []string{"object"}, kind: "rule", tag: "object", prefix: "object", skipAdventure: true, title: identityTitle},
	{path: "data/vehicles.json", keys: []string{"vehicle"}, kind: "rule", tag: "vehicle", prefix: "vehicle", skipAdventure: true, title: identityTitle},
	{path: "data/deities.json", keys: []string{"deity"}, kind: "rule", tag: "deity", prefix: "deity", skipAdventure: true, title: identityTitle},
	{path: "data/trapshazards.json", keys: []string{"trap", "hazard"}, kind: "rule", tag: "hazard", prefix: "hazard", skipAdventure: true, title: identityTitle},
	{path: "data/variantrules.json", keys: []string{"variantrule"}, kind: "rule", tag: "variantrule", prefix: "rule", skipAdventure: true, title: identityTitle},
	{path: "data/actions.json", keys: []string{"action"}, kind: "rule", tag: "action", prefix: "action", skipAdventure: true, title: identityTitle},
	{path: "data/languages.json", keys: []string{"language"}, kind: "rule", tag: "language", prefix: "language", skipAdventure: true, title: identityTitle},
	{path: "data/charcreationoptions.json", keys: []string{"charoption"}, kind: "rule", tag: "option", prefix: "option", skipAdventure: true, title: identityTitle},
	{path: "data/bastions.json", keys: []string{"bastion", "facility"}, kind: "rule", tag: "bastion", prefix: "bastion", skipAdventure: true, title: identityTitle},
	{path: "data/recipes.json", keys: []string{"recipe"}, kind: "rule", tag: "recipe", prefix: "recipe", skipAdventure: true, title: identityTitle},
	{path: "data/decks.json", keys: []string{"deck", "card"}, kind: "item", tag: "deck", prefix: "deck", skipAdventure: true},
	{path: "data/cultsboons.json", keys: []string{"cult", "boon"}, kind: "rule", tag: "cult", prefix: "cult", skipAdventure: true, title: identityTitle},
	{path: "data/psionics.json", keys: []string{"psionic"}, kind: "rule", tag: "psionic", prefix: "psionic", skipAdventure: true, title: identityTitle},
}

func (c *Catalog) convertBestiaryFile(file, code string, adventure bool) ([]draft, error) {
	data, ok, err := c.get("data/bestiary/" + file)
	if err != nil || !ok {
		return nil, err
	}
	items, err := jsonObjects(data, "monster")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	c.rememberMonsters(items)
	fluff := c.loadBestiaryFluff(file)
	return convertMonsters(items, fluff, code, c.resolveMonster, adventure), nil
}

func (c *Catalog) convertSpellFile(file, code string) ([]draft, error) {
	data, ok, err := c.get("data/spells/" + file)
	if err != nil || !ok {
		return nil, err
	}
	items, err := jsonObjects(data, "spell")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	fluff := map[string]string{}
	fluffName := "fluff-" + strings.TrimSuffix(file, ".json") + ".json"
	if raw, found, err := c.get("data/spells/" + fluffName); err != nil {
		return nil, err
	} else if found {
		fluff = fluffIndex(raw, "spellFluff")
		if len(fluff) == 0 {
			fluff = fluffIndex(raw, "spell")
		}
	}
	return convertSpells(filterSource(items, code), fluff, code), nil
}

func Materialize(bundle Bundle, scope domain.Scope, now time.Time) ([]domain.Record, []domain.PlannedNotes) {
	library := domain.Scope{}
	campaign := scope
	kind := bundle.Doc.Kind
	records := make([]domain.Record, 0, len(bundle.Records))
	seen := map[string]bool{}
	for _, d := range bundle.Records {
		recScope := library
		if kind == domain.SourceAdventure {
			switch d.Type {
			case domain.Creature:
				if hasTag(d.Tags, "bestiary") && !hasTag(d.Tags, "adventure") {
					recScope = library
				} else {
					recScope = campaign
				}
			case domain.Rule:
				recScope = library
			default:
				recScope = campaign
			}
			if d.Type == domain.NPC || d.Type == domain.Location || d.Type == domain.Note || d.Type == domain.Item {
				recScope = campaign
			}
			if d.Type == domain.Creature && hasTag(d.Tags, "adventure") {
				recScope = campaign
			}
		}
		rec := d.record(bundle.Doc, recScope)
		if seen[rec.ID] {
			continue
		}
		seen[rec.ID] = true
		records = append(records, rec)
	}
	plans := make([]domain.PlannedNotes, 0, len(bundle.Plans))
	if kind == domain.SourceAdventure {
		for _, plan := range bundle.Plans {
			body := truncate(plan.Body, 2500)
			if loc := strings.TrimSpace(plan.Title); loc != "" {
				body = "#location " + loc + "\n\n" + body
			}
			plans = append(plans, domain.PlannedNotes{
				ID:        bundle.Doc.ID + "-prep-" + slug(plan.Title),
				Title:     plan.Title,
				Scope:     campaign,
				Body:      body,
				SourceID:  bundle.Doc.ID,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}
	if kind == domain.SourceAdventure {
		domain.NestOverviewFolders(records)
	}
	return records, plans
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}
