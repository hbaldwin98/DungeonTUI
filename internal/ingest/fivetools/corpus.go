package fivetools

import (
	"fmt"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

const PluginIDPrefix = "ruleset:dnd5e:"

type Kind string

const (
	KindCreature Kind = "creature"
	KindItem     Kind = "item"
)

// Hit is one composed ruleset entity ready to display as markdown.
type Hit struct {
	Kind    Kind
	Name    string
	Source  string
	Summary string
	Body    string
}

func (h Hit) ID() string {
	return PluginIDPrefix + string(h.Kind) + ":" + slug(h.Source) + ":" + slug(h.Name)
}

func (h Hit) Record() domain.Record {
	typ := domain.Creature
	if h.Kind == KindItem {
		typ = domain.Item
	}
	return domain.Record{
		ID:        h.ID(),
		Type:      typ,
		Title:     h.Name,
		Summary:   h.Summary,
		Body:      h.Body,
		Authority: domain.Canon,
		Source:    h.Source,
		Aliases:   []string{h.Name},
		Tags:      []string{"plugin", "ruleset", "dnd5e", string(h.Kind)},
	}
}

func IsPluginID(id string) bool {
	return strings.HasPrefix(id, PluginIDPrefix)
}

func SourceCode(sourceID string) string {
	return strings.TrimPrefix(sourceID, "src-5e-")
}

// CoreBestiaryCode is the shared monster book an adventure cites (MM for 2014,
// XMM for 2024-style X-ids). Lookup can resolve a creature mention without
// ingesting that bestiary as a wiki tree.
func CoreBestiaryCode(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "XMM", "XPHB", "XDMG":
		return "XMM"
	case "MM", "PHB", "DMG":
		return "MM"
	}
	if strings.HasPrefix(strings.ToUpper(code), "X") {
		return "XMM"
	}
	return "MM"
}

func (c *Catalog) SetPreferred(codes []string) {
	c.preferred = append([]string(nil), codes...)
}

func (c *Catalog) LoadCompose() error {
	if err := c.loadTemplates(); err != nil {
		return err
	}
	return c.loadLegendary()
}

func (c *Catalog) LoadBestiary(code string) error {
	if code == "" {
		return nil
	}
	if err := c.loadIndexes(); err != nil {
		return err
	}
	file := indexLookup(c.bestiaryIndex, code)
	if file == "" {
		return nil
	}
	data, ok, err := c.get("data/bestiary/" + file)
	if err != nil || !ok {
		return err
	}
	items, err := jsonObjects(data, "monster")
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	c.rememberMonsters(items)
	c.loadBestiaryFluff(file)
	return nil
}

func (c *Catalog) LoadItems() error {
	if err := c.loadItemFile("data/items.json", "item"); err != nil {
		return err
	}
	if err := c.loadItemFile("data/items.json", "itemGroup"); err != nil {
		return err
	}
	if err := c.loadItemFile("data/items-base.json", "baseitem"); err != nil {
		return err
	}
	data, ok, err := c.get("data/magicvariants.json")
	if err != nil || !ok {
		return err
	}
	items, err := jsonObjects(data, "magicvariant")
	if err != nil {
		return err
	}
	c.rememberVariants(items)
	return nil
}

// Prime fetches compose files, the chosen book, its core bestiary, and item
// tables into the fetcher cache without creating wiki records.
func Prime(fetcher Fetcher, entry Entry) error {
	c := NewCatalog(fetcher)
	if err := c.loadIndexes(); err != nil {
		return err
	}
	_ = c.LoadCompose()
	_ = c.LoadBestiary(entry.ID)
	if core := CoreBestiaryCode(entry.ID); !strings.EqualFold(core, entry.ID) {
		_ = c.LoadBestiary(core)
	}
	_ = c.LoadItems()
	return nil
}

func (c *Catalog) Lookup(kind Kind, name, source string) (Hit, bool) {
	name = strings.TrimSpace(name)
	source = strings.TrimSpace(source)
	if name == "" {
		return Hit{}, false
	}
	if kind == "" || kind == KindCreature {
		if item, src, ok := c.findMonster(name, source); ok {
			return c.renderMonster(item, src), true
		}
		if kind == KindCreature {
			return Hit{}, false
		}
	}
	if kind == "" || kind == KindItem {
		if item, src, ok := c.findItem(name, source); ok {
			return c.renderItem(item, src), true
		}
		if item, src, ok := c.findVariant(name, source); ok {
			return c.renderVariant(item, src), true
		}
	}
	return Hit{}, false
}

func (c *Catalog) LookupName(raw string) (Hit, bool) {
	name, source := splitNameSource(raw)
	return c.Lookup("", name, source)
}

func (c *Catalog) Search(query string, limit int) []Hit {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit < 1 {
		return nil
	}
	out := make([]Hit, 0, limit)
	add := func(kind Kind, store map[string]map[string]any) {
		for key, item := range store {
			if len(out) >= limit {
				return
			}
			name := asString(item["name"])
			if name == "" {
				_, name, _ = strings.Cut(key, "|")
			}
			if !strings.Contains(strings.ToLower(name), query) {
				continue
			}
			src := itemSource(item)
			switch kind {
			case KindCreature:
				out = append(out, c.renderMonster(item, src))
			case KindItem:
				if _, ok := item["inherits"]; ok {
					out = append(out, c.renderVariant(item, src))
				} else {
					out = append(out, c.renderItem(item, src))
				}
			}
		}
	}
	add(KindCreature, c.monsters)
	if len(out) < limit {
		add(KindItem, c.items)
	}
	if len(out) < limit {
		add(KindItem, c.variants)
	}
	return out
}

func splitNameSource(raw string) (name, source string) {
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, "|"); i >= 0 {
		return strings.TrimSpace(raw[:i]), strings.TrimSpace(raw[i+1:])
	}
	return raw, ""
}

func (c *Catalog) loadTemplates() error {
	data, ok, err := c.get("data/bestiary/template.json")
	if err != nil || !ok {
		return err
	}
	items, err := jsonObjects(data, "monsterTemplate")
	if err != nil {
		return err
	}
	for _, item := range items {
		name := asString(item["name"])
		src := itemSource(item)
		if name == "" {
			continue
		}
		c.templates[strings.ToLower(name+"|"+src)] = item
		c.templates[strings.ToLower(name)] = item
	}
	return nil
}

func (c *Catalog) loadLegendary() error {
	data, ok, err := c.get("data/bestiary/legendarygroups.json")
	if err != nil || !ok {
		return err
	}
	items, err := jsonObjects(data, "legendaryGroup")
	if err != nil {
		return err
	}
	for _, item := range items {
		name := asString(item["name"])
		src := itemSource(item)
		if name == "" {
			continue
		}
		c.legendary[strings.ToLower(name+"|"+src)] = item
		c.legendary[strings.ToLower(name)] = item
	}
	return nil
}

func (c *Catalog) loadItemFile(path, key string) error {
	data, ok, err := c.get(path)
	if err != nil || !ok {
		return err
	}
	items, err := jsonObjects(data, key)
	if err != nil {
		return err
	}
	c.rememberItems(items)
	return nil
}

func (c *Catalog) loadBestiaryFluff(file string) map[string]string {
	fluffName := "fluff-" + strings.TrimSuffix(file, ".json") + ".json"
	raw, found, err := c.get("data/bestiary/" + fluffName)
	if err != nil || !found {
		return map[string]string{}
	}
	fluff := fluffIndex(raw, "monsterFluff")
	if len(fluff) == 0 {
		fluff = fluffIndex(raw, "monster")
	}
	if c.fluff == nil {
		c.fluff = map[string]string{}
	}
	for k, v := range fluff {
		c.fluff[k] = v
	}
	return fluff
}

func (c *Catalog) rememberMonsters(items []map[string]any) {
	for _, item := range items {
		name := asString(item["name"])
		src := itemSource(item)
		if name == "" {
			continue
		}
		c.monsters[strings.ToLower(name+"|"+src)] = item
	}
}

func (c *Catalog) rememberItems(items []map[string]any) {
	for _, item := range items {
		name := asString(item["name"])
		src := itemSource(item)
		if name == "" {
			continue
		}
		c.items[strings.ToLower(name+"|"+src)] = item
	}
}

func (c *Catalog) rememberVariants(items []map[string]any) {
	for _, item := range items {
		name := asString(item["name"])
		if name == "" {
			continue
		}
		src := itemSource(item)
		if src == "" {
			if inherits := asMap(item["inherits"]); inherits != nil {
				src = itemSource(inherits)
			}
		}
		c.variants[strings.ToLower(name+"|"+src)] = item
		c.variants[strings.ToLower(name)] = item
	}
}

func (c *Catalog) resolveMonster(item map[string]any) map[string]any {
	item = c.resolveMonsterDepth(item, 0)
	return c.applyLegendaryGroup(item)
}

func (c *Catalog) resolveMonsterDepth(item map[string]any, depth int) map[string]any {
	copy := asMap(item["_copy"])
	if copy == nil || depth > 4 {
		return item
	}
	parentName := asString(copy["name"])
	parentSrc := asString(copy["source"])
	parent, err := c.lookupMonster(parentName, parentSrc)
	if err != nil || parent == nil {
		item = cloneMap(item)
		item["_unresolvedCopy"] = strings.TrimSpace(parentName + " (" + parentSrc + ")")
		return item
	}
	parent = c.resolveMonsterDepth(parent, depth+1)
	merged := mergeMaps(parent, item)
	for _, tmplRef := range asList(copy["_templates"]) {
		ref := asMap(tmplRef)
		if ref == nil {
			continue
		}
		tmpl := c.lookupTemplate(asString(ref["name"]), asString(ref["source"]))
		if tmpl != nil {
			merged = applyMonsterTemplate(merged, tmpl)
		}
	}
	return merged
}

func (c *Catalog) lookupMonster(name, code string) (map[string]any, error) {
	if name == "" {
		return nil, nil
	}
	if item, _, ok := c.findMonster(name, code); ok {
		return item, nil
	}
	if code == "" {
		return nil, nil
	}
	file := indexLookup(c.bestiaryIndex, code)
	if file == "" {
		return nil, nil
	}
	data, ok, err := c.get("data/bestiary/" + file)
	if err != nil || !ok {
		return nil, err
	}
	items, err := jsonObjects(data, "monster")
	if err != nil {
		return nil, err
	}
	c.rememberMonsters(items)
	c.loadBestiaryFluff(file)
	item, _, ok := c.findMonster(name, code)
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (c *Catalog) lookupTemplate(name, code string) map[string]any {
	if name == "" {
		return nil
	}
	if code != "" {
		if item, ok := c.templates[strings.ToLower(name+"|"+code)]; ok {
			return item
		}
	}
	return c.templates[strings.ToLower(name)]
}

func (c *Catalog) applyLegendaryGroup(item map[string]any) map[string]any {
	ref := asMap(item["legendaryGroup"])
	if ref == nil {
		return item
	}
	name := asString(ref["name"])
	src := asString(ref["source"])
	group := c.legendary[strings.ToLower(name+"|"+src)]
	if group == nil {
		group = c.legendary[strings.ToLower(name)]
	}
	if group == nil {
		return item
	}
	out := cloneMap(item)
	if out["lairActions"] == nil && group["lairActions"] != nil {
		out["lairActions"] = group["lairActions"]
	}
	if out["regionalEffects"] == nil && group["regionalEffects"] != nil {
		out["regionalEffects"] = group["regionalEffects"]
	}
	return out
}

func (c *Catalog) findMonster(name, source string) (map[string]any, string, bool) {
	return pickNamed(c.monsters, name, source, c.preferred)
}

func (c *Catalog) findItem(name, source string) (map[string]any, string, bool) {
	return pickNamed(c.items, name, source, c.preferred)
}

func (c *Catalog) findVariant(name, source string) (map[string]any, string, bool) {
	if source != "" {
		if item, ok := c.variants[strings.ToLower(name+"|"+source)]; ok {
			return item, itemSource(item), true
		}
	}
	if item, ok := c.variants[strings.ToLower(name)]; ok {
		src := itemSource(item)
		if src == "" {
			if inherits := asMap(item["inherits"]); inherits != nil {
				src = itemSource(inherits)
			}
		}
		return item, src, true
	}
	return nil, "", false
}

func (c *Catalog) renderMonster(item map[string]any, source string) Hit {
	item = c.resolveMonster(item)
	name := asString(item["name"])
	src := firstNonEmpty(itemSource(item), source)
	body := formatMonster(item)
	body = appendFluff(body, name, src, c.fluff)
	return Hit{
		Kind:    KindCreature,
		Name:    name,
		Source:  src,
		Summary: monsterSummary(item),
		Body:    body,
	}
}

func (c *Catalog) renderItem(item map[string]any, source string) Hit {
	name := asString(item["name"])
	src := firstNonEmpty(itemSource(item), source)
	return Hit{
		Kind:    KindItem,
		Name:    name,
		Source:  src,
		Summary: itemSummary(item),
		Body:    formatItem(item),
	}
}

func (c *Catalog) renderVariant(item map[string]any, source string) Hit {
	name := asString(item["name"])
	src := firstNonEmpty(source, itemSource(item))
	if src == "" {
		if inherits := asMap(item["inherits"]); inherits != nil {
			src = itemSource(inherits)
		}
	}
	return Hit{
		Kind:    KindItem,
		Name:    name,
		Source:  src,
		Summary: firstNonEmpty(itemTypeLine(asMap(item["inherits"])), "magic item variant"),
		Body:    formatMagicVariant(item),
	}
}

func pickNamed(store map[string]map[string]any, name, source string, preferred []string) (map[string]any, string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || store == nil {
		return nil, "", false
	}
	if source != "" {
		if item, ok := store[strings.ToLower(name+"|"+source)]; ok {
			return item, itemSource(item), true
		}
		return nil, "", false
	}
	for _, code := range preferred {
		if item, ok := store[strings.ToLower(name+"|"+code)]; ok {
			return item, itemSource(item), true
		}
	}
	prefix := strings.ToLower(name + "|")
	var fallback map[string]any
	var fallbackSrc string
	for key, item := range store {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		src := itemSource(item)
		if fallback == nil {
			fallback, fallbackSrc = item, src
		}
		if len(preferred) == 0 {
			return item, src, true
		}
	}
	if fallback != nil {
		return fallback, fallbackSrc, true
	}
	return nil, "", false
}

func applyMonsterTemplate(item, tmpl map[string]any) map[string]any {
	apply := asMap(tmpl["apply"])
	if apply == nil {
		return item
	}
	out := cloneMap(item)
	if root := asMap(apply["_root"]); root != nil {
		for k, v := range root {
			out[k] = v
		}
	}
	if mod := asMap(apply["_mod"]); mod != nil {
		out = applyTemplateMod(out, mod, asString(out["name"]))
	}
	return out
}

func applyTemplateMod(item map[string]any, mod map[string]any, name string) map[string]any {
	out := cloneMap(item)
	for key, spec := range mod {
		if key == "_" {
			continue
		}
		m := asMap(spec)
		if m == nil {
			continue
		}
		switch asString(m["mode"]) {
		case "appendArr", "appendIfNotExistsArr":
			existing := append([]any{}, asList(out[key])...)
			for _, add := range asList(m["items"]) {
				add = fillTemplatePlaceholders(add, name)
				if asString(m["mode"]) == "appendIfNotExistsArr" && listContains(existing, add) {
					continue
				}
				existing = append(existing, add)
			}
			out[key] = existing
		}
	}
	return out
}

func fillTemplatePlaceholders(v any, name string) any {
	switch t := v.(type) {
	case string:
		t = strings.ReplaceAll(t, "<$title_short_name$>", name)
		t = strings.ReplaceAll(t, "<$title_name$>", name)
		return t
	case map[string]any:
		out := cloneMap(t)
		for k, val := range out {
			out[k] = fillTemplatePlaceholders(val, name)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = fillTemplatePlaceholders(val, name)
		}
		return out
	default:
		return v
	}
}

func listContains(list []any, want any) bool {
	wantS := fmt.Sprint(want)
	for _, item := range list {
		if fmt.Sprint(item) == wantS {
			return true
		}
	}
	return false
}
