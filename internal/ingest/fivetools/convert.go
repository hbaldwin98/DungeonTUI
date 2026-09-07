package fivetools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

var numberedRoom = regexp.MustCompile(`^(\d+)(?:\.(\s+|$)|$)`)

type draft struct {
	Type    domain.EntityType
	Title   string
	Summary string
	Body    string
	Aliases []string
	Tags    []string
	Prefix  string
	Group   string
}

func (d draft) record(doc domain.SourceDocument, scope domain.Scope) domain.Record {
	body := strings.TrimSpace(d.Body)
	if body == "" {
		body = d.Title + " from " + doc.Title + ".\n"
	}
	prefix := d.Prefix
	if prefix == "" {
		prefix = strings.ToLower(string(d.Type))
	}
	idKey := d.Title
	if d.Type == domain.Location && d.Group != "" && !strings.EqualFold(d.Group, d.Title) {
		idKey = d.Group + " " + d.Title
	}
	return domain.Record{
		ID:        doc.ID + "-" + prefix + "-" + slug(idKey),
		Type:      d.Type,
		Title:     d.Title,
		Summary:   firstNonEmpty(d.Summary, summaryOf(body), d.Title+" from "+doc.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Scope:     scope,
		Source:    doc.Title,
		SourceID:  doc.ID,
		Aliases:   unique(d.Aliases),
		Tags:      unique(append([]string{"source", "5etools"}, d.Tags...)),
		Folder:    domain.SourceFolderWithGroup(doc.Title, domain.Record{Type: d.Type, Tags: d.Tags, Source: doc.Title, SourceID: doc.ID}, d.Group),
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}

func filterSource(items []map[string]any, code string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, item := range items {
		if sourceMatch(item, code) {
			out = append(out, item)
		}
	}
	return out
}

func fluffIndex(data []byte, key string) map[string]string {
	out := map[string]string{}
	items, err := jsonObjects(data, key)
	if err != nil {
		return out
	}
	for _, item := range items {
		name := asString(item["name"])
		if name == "" {
			continue
		}
		key := strings.ToLower(name + "|" + itemSource(item))
		body := strings.TrimSpace(renderValue(item["entries"]))
		if body != "" {
			out[key] = body
			out[strings.ToLower(name)] = body
		}
	}
	return out
}

func appendFluff(body, name, code string, fluff map[string]string) string {
	if len(fluff) == 0 {
		return body
	}
	text := fluff[strings.ToLower(name+"|"+code)]
	if text == "" {
		text = fluff[strings.ToLower(name)]
	}
	if text == "" {
		return body
	}
	return strings.TrimSpace(body) + "\n\n## Description\n\n" + text
}

func convertMonsters(items []map[string]any, fluff map[string]string, code string, resolve func(map[string]any) map[string]any, adventure bool) []draft {
	out := make([]draft, 0, len(items))
	for _, item := range items {
		if resolve != nil {
			item = resolve(item)
		}
		name := asString(item["name"])
		if name == "" {
			continue
		}
		body := formatMonster(item)
		body = appendFluff(body, name, code, fluff)
		typ := domain.Creature
		prefix := "creature"
		tags := []string{"creature", "bestiary"}
		if adventure {
			tags = []string{"creature", "adventure"}
		}
		if truthy(item["isNpc"]) || truthy(item["isNamedCreature"]) {
			typ = domain.NPC
			prefix = "npc"
			tags = []string{"npc", "adventure"}
		}
		out = append(out, draft{
			Type:    typ,
			Title:   name,
			Summary: monsterSummary(item),
			Body:    truncate(body, 8000),
			Aliases: []string{name},
			Tags:    tags,
			Prefix:  prefix,
		})
	}
	return out
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

func formatMonster(item map[string]any) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(asString(item["name"]))
	b.WriteString("\n\n")
	if line := monsterTypeLine(item); line != "" {
		b.WriteString("*")
		b.WriteString(line)
		b.WriteString("*\n\n")
	}
	if copy := asString(item["_unresolvedCopy"]); copy != "" {
		b.WriteString("This entry copies ")
		b.WriteString(copy)
		b.WriteString(". Parent stats were not available to resolve.\n\n")
	}
	writeKV(&b, "AC", formatAC(item["ac"]))
	writeKV(&b, "HP", formatHP(item["hp"]))
	writeKV(&b, "Speed", formatSpeed(item["speed"]))
	if scores := formatAbilities(item); scores != "" {
		b.WriteString("\n")
		b.WriteString(scores)
		b.WriteString("\n\n")
	}
	writeKV(&b, "Saves", formatKeyed(item["save"]))
	writeKV(&b, "Skills", formatKeyed(item["skill"]))
	writeKV(&b, "Senses", joinAny(item["senses"]))
	writeKV(&b, "Languages", joinAny(item["languages"]))
	writeKV(&b, "CR", formatCR(item["cr"]))
	writeNamedBlocks(&b, "Traits", item["trait"])
	writeNamedBlocks(&b, "Actions", item["action"])
	writeNamedBlocks(&b, "Bonus Actions", item["bonus"])
	writeNamedBlocks(&b, "Reactions", item["reaction"])
	writeNamedBlocks(&b, "Legendary Actions", item["legendary"])
	writeNamedBlocks(&b, "Mythic Actions", item["mythic"])
	return strings.TrimSpace(b.String()) + "\n"
}

func monsterSummary(item map[string]any) string {
	parts := []string{}
	if line := monsterTypeLine(item); line != "" {
		parts = append(parts, line)
	}
	if ac := formatAC(item["ac"]); ac != "" {
		parts = append(parts, "AC "+ac)
	}
	if hp := formatHP(item["hp"]); hp != "" {
		parts = append(parts, "HP "+hp)
	}
	if cr := formatCR(item["cr"]); cr != "" {
		parts = append(parts, "CR "+cr)
	}
	return strings.Join(parts, " · ")
}

func monsterTypeLine(item map[string]any) string {
	size := formatSize(item["size"])
	typ := formatType(item["type"])
	align := formatAlignment(item["alignment"])
	parts := []string{}
	if size != "" {
		parts = append(parts, size)
	}
	if typ != "" {
		parts = append(parts, typ)
	}
	line := strings.Join(parts, " ")
	if align != "" {
		if line != "" {
			line += ", " + align
		} else {
			line = align
		}
	}
	return line
}

func formatSize(v any) string {
	names := map[string]string{"T": "Tiny", "S": "Small", "M": "Medium", "L": "Large", "H": "Huge", "G": "Gargantuan"}
	parts := []string{}
	for _, item := range asList(v) {
		code := asString(item)
		if n, ok := names[strings.ToUpper(code)]; ok {
			parts = append(parts, n)
			continue
		}
		if code != "" {
			parts = append(parts, code)
		}
	}
	return strings.Join(parts, " or ")
}

func formatType(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		core := firstNonEmpty(asString(t["type"]), asString(t["choose"]))
		tags := []string{}
		for _, tag := range asList(t["tags"]) {
			if s := asString(tag); s != "" {
				tags = append(tags, s)
			}
		}
		if len(tags) > 0 && core != "" {
			return core + " (" + strings.Join(tags, ", ") + ")"
		}
		return core
	default:
		return asString(v)
	}
}

func formatAlignment(v any) string {
	names := map[string]string{
		"L": "Lawful", "N": "Neutral", "C": "Chaotic",
		"G": "Good", "E": "Evil", "U": "Unaligned", "A": "Any alignment",
	}
	switch t := v.(type) {
	case map[string]any:
		return asString(t["special"])
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			code := asString(item)
			if n, ok := names[strings.ToUpper(code)]; ok {
				parts = append(parts, n)
				continue
			}
			if code != "" {
				parts = append(parts, code)
			}
		}
		return strings.Join(parts, " ")
	default:
		return asString(v)
	}
}

func formatAC(v any) string {
	parts := []string{}
	for _, item := range asList(v) {
		switch t := item.(type) {
		case map[string]any:
			if spec := asString(t["special"]); spec != "" {
				parts = append(parts, spec)
				continue
			}
			ac := asString(t["ac"])
			from := joinAny(t["from"])
			if ac != "" && from != "" {
				parts = append(parts, ac+" ("+from+")")
			} else {
				parts = append(parts, ac)
			}
		default:
			if s := asString(item); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, ", ")
}

func formatHP(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if spec := asString(t["special"]); spec != "" {
			return spec
		}
		avg := asString(t["average"])
		formula := asString(t["formula"])
		switch {
		case avg != "" && formula != "":
			return avg + " (" + formula + ")"
		case avg != "":
			return avg
		default:
			return formula
		}
	default:
		return asString(v)
	}
}

func formatSpeed(v any) string {
	m := asMap(v)
	if m == nil {
		return asString(v)
	}
	order := []string{"walk", "burrow", "climb", "fly", "swim"}
	parts := []string{}
	seen := map[string]bool{}
	writeSpeed := func(mode string) {
		val, ok := m[mode]
		if !ok {
			return
		}
		seen[mode] = true
		num, cond := speedPart(val)
		if num == "" {
			return
		}
		piece := num + " ft."
		if cond != "" {
			piece += " " + cond
		}
		if mode == "walk" {
			parts = append(parts, piece)
			return
		}
		parts = append(parts, mode+" "+piece)
	}
	for _, mode := range order {
		writeSpeed(mode)
	}
	for mode := range m {
		if !seen[mode] && mode != "canHover" {
			writeSpeed(mode)
		}
	}
	if truthy(m["canHover"]) {
		parts = append(parts, "hover")
	}
	return strings.Join(parts, ", ")
}

func speedPart(v any) (string, string) {
	switch t := v.(type) {
	case map[string]any:
		return firstNonEmpty(asString(t["number"]), asString(t["walk"])), asString(t["condition"])
	default:
		return asString(v), ""
	}
}

func formatAbilities(item map[string]any) string {
	keys := []string{"str", "dex", "con", "int", "wis", "cha"}
	labels := []string{"STR", "DEX", "CON", "INT", "WIS", "CHA"}
	vals := make([]string, 0, 6)
	for _, key := range keys {
		n, ok := asInt(item[key])
		if !ok {
			return ""
		}
		vals = append(vals, fmt.Sprintf("%d (%s)", n, abilityMod(n)))
	}
	var b strings.Builder
	b.WriteString("|")
	for _, label := range labels {
		b.WriteString(" ")
		b.WriteString(label)
		b.WriteString(" |")
	}
	b.WriteString("\n|")
	for range labels {
		b.WriteString(" --- |")
	}
	b.WriteString("\n|")
	for _, val := range vals {
		b.WriteString(" ")
		b.WriteString(val)
		b.WriteString(" |")
	}
	return b.String()
}

func formatKeyed(v any) string {
	m := asMap(v)
	if m == nil {
		return asString(v)
	}
	parts := []string{}
	for k, val := range m {
		parts = append(parts, k+" "+asString(val))
	}
	return strings.Join(parts, ", ")
}

func formatCR(v any) string {
	switch t := v.(type) {
	case map[string]any:
		return firstNonEmpty(asString(t["cr"]), asString(t["lair"]))
	default:
		return asString(v)
	}
}

func joinAny(v any) string {
	parts := []string{}
	for _, item := range asList(v) {
		if s := asString(item); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

func writeKV(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("**")
	b.WriteString(label)
	b.WriteString("** ")
	b.WriteString(value)
	b.WriteString("\n")
}

func writeNamedBlocks(b *strings.Builder, heading string, v any) {
	items := asList(v)
	if len(items) == 0 {
		return
	}
	b.WriteString("\n### ")
	b.WriteString(heading)
	b.WriteString("\n\n")
	for _, item := range items {
		m := asMap(item)
		if m == nil {
			if s := strings.TrimSpace(renderValue(item)); s != "" {
				b.WriteString(s)
				b.WriteString("\n\n")
			}
			continue
		}
		name := asString(m["name"])
		body := strings.TrimSpace(renderValue(m["entries"]))
		if name != "" {
			b.WriteString("**")
			b.WriteString(name)
			b.WriteString(".** ")
		}
		b.WriteString(body)
		b.WriteString("\n\n")
	}
}

func convertSpells(items []map[string]any, fluff map[string]string, code string) []draft {
	out := make([]draft, 0, len(items))
	for _, item := range items {
		name := asString(item["name"])
		if name == "" {
			continue
		}
		body := formatSpell(item)
		body = appendFluff(body, name, code, fluff)
		out = append(out, draft{
			Type:    domain.Rule,
			Title:   name,
			Summary: spellSummary(item),
			Body:    truncate(body, 6000),
			Tags:    []string{"spell", "rules"},
			Prefix:  "spell",
		})
	}
	return out
}

func formatSpell(item map[string]any) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(asString(item["name"]))
	b.WriteString("\n\n")
	if line := spellTypeLine(item); line != "" {
		b.WriteString("*")
		b.WriteString(line)
		b.WriteString("*\n\n")
	}
	writeKV(&b, "Casting Time", formatSpellTime(item["time"]))
	writeKV(&b, "Range", formatSpellRange(item["range"]))
	writeKV(&b, "Components", formatComponents(item["components"]))
	writeKV(&b, "Duration", formatDuration(item["duration"]))
	if text := strings.TrimSpace(renderValue(item["entries"])); text != "" {
		b.WriteString("\n")
		b.WriteString(text)
		b.WriteString("\n")
	}
	if higher := strings.TrimSpace(renderValue(item["entriesHigherLevel"])); higher != "" {
		b.WriteString("\n")
		b.WriteString(higher)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func spellSummary(item map[string]any) string {
	return strings.Join([]string{spellTypeLine(item), formatSpellTime(item["time"]), formatSpellRange(item["range"])}, " · ")
}

func spellTypeLine(item map[string]any) string {
	level, ok := asInt(item["level"])
	school := schoolName(asString(item["school"]))
	levelLabel := "Cantrip"
	if ok && level > 0 {
		levelLabel = fmt.Sprintf("Level %d", level)
	}
	if school == "" {
		return levelLabel
	}
	if level == 0 {
		return school + " cantrip"
	}
	return levelLabel + " " + school
}

func schoolName(code string) string {
	switch strings.ToUpper(code) {
	case "A":
		return "Abjuration"
	case "C":
		return "Conjuration"
	case "D":
		return "Divination"
	case "E":
		return "Enchantment"
	case "V":
		return "Evocation"
	case "I":
		return "Illusion"
	case "N":
		return "Necromancy"
	case "T":
		return "Transmutation"
	default:
		return code
	}
}

func formatSpellTime(v any) string {
	parts := []string{}
	for _, item := range asList(v) {
		m := asMap(item)
		if m == nil {
			if s := asString(item); s != "" {
				parts = append(parts, s)
			}
			continue
		}
		n := asString(m["number"])
		unit := asString(m["unit"])
		cond := asString(m["condition"])
		piece := strings.TrimSpace(n + " " + unit)
		if cond != "" {
			piece += " (" + cond + ")"
		}
		if piece != "" {
			parts = append(parts, piece)
		}
	}
	return strings.Join(parts, ", ")
}

func formatSpellRange(v any) string {
	m := asMap(v)
	if m == nil {
		return asString(v)
	}
	typ := asString(m["type"])
	dist := asMap(m["distance"])
	if dist != nil {
		amount := asString(dist["amount"])
		unit := asString(dist["type"])
		if amount != "" && unit != "" {
			if unit == "feet" {
				unit = "feet"
			}
			return amount + " " + unit
		}
		if unit != "" {
			return unit
		}
	}
	return typ
}

func formatComponents(v any) string {
	m := asMap(v)
	if m == nil {
		return asString(v)
	}
	parts := []string{}
	if truthy(m["v"]) {
		parts = append(parts, "V")
	}
	if truthy(m["s"]) {
		parts = append(parts, "S")
	}
	if m["m"] != nil {
		mat := asString(m["m"])
		if mat == "" || mat == "true" {
			parts = append(parts, "M")
		} else {
			parts = append(parts, "M ("+mat+")")
		}
	}
	return strings.Join(parts, ", ")
}

func formatDuration(v any) string {
	parts := []string{}
	for _, item := range asList(v) {
		m := asMap(item)
		if m == nil {
			parts = append(parts, asString(item))
			continue
		}
		typ := asString(m["type"])
		if typ == "instant" {
			parts = append(parts, "Instantaneous")
			continue
		}
		if typ == "timed" {
			dur := asMap(m["duration"])
			piece := strings.TrimSpace(asString(dur["amount"]) + " " + asString(dur["type"]))
			if truthy(m["concentration"]) {
				piece = "Concentration, up to " + piece
			}
			parts = append(parts, piece)
			continue
		}
		if s := asString(m["special"]); s != "" {
			parts = append(parts, s)
			continue
		}
		parts = append(parts, typ)
	}
	return strings.Join(parts, ", ")
}

func convertItems(items []map[string]any, code string) []draft {
	out := make([]draft, 0, len(items))
	for _, item := range items {
		if !sourceMatch(item, code) {
			continue
		}
		name := asString(item["name"])
		if name == "" {
			continue
		}
		body := formatItem(item)
		out = append(out, draft{
			Type:    domain.Item,
			Title:   name,
			Summary: itemSummary(item),
			Body:    truncate(body, 5000),
			Tags:    []string{"item"},
			Prefix:  "item",
		})
	}
	return out
}

func formatItem(item map[string]any) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(asString(item["name"]))
	b.WriteString("\n\n")
	if line := itemTypeLine(item); line != "" {
		b.WriteString("*")
		b.WriteString(line)
		b.WriteString("*\n\n")
	}
	writeKV(&b, "Type", asString(item["type"]))
	writeKV(&b, "Weight", asString(item["weight"]))
	writeKV(&b, "Value", asString(item["value"]))
	writeKV(&b, "Requires Attunement", attuneText(item))
	if text := strings.TrimSpace(renderValue(item["entries"])); text != "" {
		b.WriteString("\n")
		b.WriteString(text)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func itemTypeLine(item map[string]any) string {
	parts := []string{}
	if rarity := asString(item["rarity"]); rarity != "" && rarity != "none" {
		parts = append(parts, rarity)
	}
	if truthy(item["wondrous"]) {
		parts = append(parts, "wondrous item")
	}
	if req := attuneText(item); req != "" && req != "no" {
		parts = append(parts, "attunement")
	}
	return strings.Join(parts, ", ")
}

func itemSummary(item map[string]any) string {
	return firstNonEmpty(itemTypeLine(item), summaryOf(renderValue(item["entries"])))
}

func attuneText(item map[string]any) string {
	switch t := item["reqAttune"].(type) {
	case nil:
		return ""
	case bool:
		if t {
			return "yes"
		}
		return ""
	case string:
		if t == "" || t == "false" {
			return ""
		}
		if t == "true" {
			return "yes"
		}
		return t
	default:
		return asString(t)
	}
}

func convertNamedRules(items []map[string]any, code, tag, prefix string, titleOf func(map[string]any) string) []draft {
	out := make([]draft, 0)
	for _, item := range items {
		if !sourceMatch(item, code) {
			continue
		}
		title := asString(item["name"])
		if titleOf != nil {
			if named := titleOf(item); named != "" {
				title = named
			}
		}
		if title == "" {
			continue
		}
		body := formatGeneric(title, item)
		out = append(out, draft{
			Type:    domain.Rule,
			Title:   title,
			Summary: summaryOf(body),
			Body:    truncate(body, 6000),
			Tags:    []string{tag, "rules"},
			Prefix:  prefix,
		})
	}
	return out
}

func formatGeneric(title string, item map[string]any) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if text := strings.TrimSpace(renderValue(item["entries"])); text != "" {
		b.WriteString(text)
		b.WriteString("\n")
	} else if text := strings.TrimSpace(renderValue(item["entry"])); text != "" {
		b.WriteString(text)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func convertClasses(data []byte, code string) []draft {
	classes, _ := jsonObjects(data, "class")
	subclasses, _ := jsonObjects(data, "subclass")
	features, _ := jsonObjects(data, "classFeature")
	subFeatures, _ := jsonObjects(data, "subclassFeature")
	out := make([]draft, 0)
	for _, class := range filterSource(classes, code) {
		name := asString(class["name"])
		if name == "" {
			continue
		}
		var b strings.Builder
		b.WriteString(formatGeneric(name, class))
		for _, feat := range features {
			if !sourceMatch(feat, code) || !strings.EqualFold(asString(feat["className"]), name) {
				continue
			}
			level := asString(feat["level"])
			title := asString(feat["name"])
			b.WriteString("\n### ")
			b.WriteString(title)
			if level != "" {
				b.WriteString(" (Level ")
				b.WriteString(level)
				b.WriteString(")")
			}
			b.WriteString("\n\n")
			b.WriteString(strings.TrimSpace(renderValue(feat["entries"])))
			b.WriteString("\n")
		}
		out = append(out, draft{
			Type:    domain.Rule,
			Title:   name,
			Summary: "Class from source " + code + ".",
			Body:    truncate(b.String(), 12000),
			Tags:    []string{"class", "rules"},
			Prefix:  "class",
		})
	}
	for _, sub := range filterSource(subclasses, code) {
		name := asString(sub["name"])
		className := asString(sub["className"])
		if name == "" {
			continue
		}
		title := name
		if className != "" {
			title = className + " (" + name + ")"
		}
		var b strings.Builder
		b.WriteString(formatGeneric(title, sub))
		short := firstNonEmpty(asString(sub["shortName"]), name)
		for _, feat := range subFeatures {
			if !sourceMatch(feat, code) {
				continue
			}
			if !strings.EqualFold(asString(feat["className"]), className) {
				continue
			}
			if !strings.EqualFold(asString(feat["subclassShortName"]), short) && !strings.EqualFold(asString(feat["subclassShortName"]), name) {
				continue
			}
			b.WriteString("\n### ")
			b.WriteString(asString(feat["name"]))
			if level := asString(feat["level"]); level != "" {
				b.WriteString(" (Level ")
				b.WriteString(level)
				b.WriteString(")")
			}
			b.WriteString("\n\n")
			b.WriteString(strings.TrimSpace(renderValue(feat["entries"])))
			b.WriteString("\n")
		}
		out = append(out, draft{
			Type:    domain.Rule,
			Title:   title,
			Summary: "Subclass from source " + code + ".",
			Body:    truncate(b.String(), 8000),
			Tags:    []string{"subclass", "class", "rules"},
			Prefix:  "subclass",
		})
	}
	return out
}

func convertAdventure(data []byte) ([]draft, []planDraft) {
	sections, err := jsonObjects(data, "data")
	if err != nil || len(sections) == 0 {
		var envelope map[string]any
		if jsonErr := mapJSON(data, &envelope); jsonErr == nil {
			sections = mapsFrom(envelope["data"])
		}
	}
	npcs := indexAdventureNPCs(sections)
	split := func(m map[string]any) bool {
		return shouldSplitAdventureBlock(m, npcs)
	}
	drafts := make([]draft, 0)
	plans := make([]planDraft, 0)
	seen := map[string]bool{}
	add := func(d draft) {
		key := d.Prefix + ":" + strings.ToLower(d.Group+"\x00"+d.Title)
		if d.Title == "" || seen[key] {
			return
		}
		seen[key] = true
		drafts = append(drafts, d)
	}
	for _, section := range sections {
		title := asString(section["name"])
		lower := strings.ToLower(title)
		if title == "" || lower == "credits" {
			continue
		}
		body := truncate(renderSkipping(section["entries"], split), 8000)
		if strings.HasPrefix(lower, "appendix b") {
			continue
		}
		if strings.HasPrefix(lower, "appendix a") {
			for _, name := range extractItemNames(section["entries"]) {
				add(draft{Type: domain.Item, Title: name, Body: "Magic item from " + title + ".", Tags: []string{"item", "adventure"}, Prefix: "item"})
			}
			add(draft{Type: domain.Note, Title: title, Body: body, Tags: []string{"adventure"}, Prefix: "note"})
			continue
		}
		if lower == "introduction" {
			add(draft{Type: domain.Note, Title: title, Body: body, Tags: []string{"adventure"}, Prefix: "note"})
			walkNamed(section["entries"], title, 1, add, npcs, split)
			continue
		}
		add(draft{Type: domain.Location, Title: title, Body: body, Tags: []string{"adventure"}, Prefix: "location"})
		plans = append(plans, planDraft{Title: title, Body: body})
		walkNamed(section["entries"], title, 1, add, npcs, split)
	}
	for _, npc := range npcs {
		if npc == nil || npc.Title == "" {
			continue
		}
		body := strings.TrimSpace(npc.Body)
		if body == "" {
			body = strings.TrimSpace(npc.Role)
		}
		if body == "" {
			body = "NPC from " + firstNonEmpty(npc.Group, "this adventure") + "."
		}
		add(draft{
			Type:    domain.NPC,
			Title:   npc.Title,
			Summary: firstNonEmpty(npc.Role, npc.Title+" from the adventure."),
			Body:    body,
			Tags:    []string{"adventure", "npc"},
			Prefix:  "npc",
			Group:   npc.Group,
		})
	}
	applyLocationGroups(drafts)
	return drafts, plans
}

type planDraft struct {
	Title string
	Body  string
}

type npcDraft struct {
	Title string
	Role  string
	Body  string
	Group string
}

func shouldSplitAdventureBlock(m map[string]any, npcs map[string]*npcDraft) bool {
	title := asString(m["name"])
	if title == "" {
		return false
	}
	typ := asString(m["type"])
	lower := strings.ToLower(title)
	if lower == "credits" || strings.Contains(lower, "important npc") {
		return true
	}
	if numberedRoom.MatchString(title) || typ == "section" {
		return true
	}
	_, ok := npcs[lower]
	return ok
}

func indexAdventureNPCs(sections []map[string]any) map[string]*npcDraft {
	out := map[string]*npcDraft{}
	var walk func(any, string)
	walk = func(v any, parent string) {
		for _, item := range asList(v) {
			m := asMap(item)
			if m == nil {
				continue
			}
			title := asString(m["name"])
			if strings.Contains(strings.ToLower(title), "important npc") {
				for _, hit := range npcRowsFrom(m) {
					key := strings.ToLower(hit.Title)
					if out[key] == nil {
						hit.Group = parent
						copyHit := hit
						out[key] = &copyHit
					}
				}
			}
			next := parent
			if title != "" && asString(m["type"]) == "section" && !numberedRoom.MatchString(title) {
				next = title
			}
			walk(m["entries"], next)
		}
	}
	for _, section := range sections {
		walk(section["entries"], asString(section["name"]))
	}
	return out
}

func npcRowsFrom(block map[string]any) []npcDraft {
	out := make([]npcDraft, 0)
	var walk func(any)
	walk = func(v any) {
		m := asMap(v)
		if m != nil && asString(m["type"]) == "table" {
			for _, row := range asList(m["rows"]) {
				cells := asList(row)
				if len(cells) == 0 {
					continue
				}
				name := strings.Trim(plainCell(cells[0]), "@* ")
				if name == "" || strings.EqualFold(name, "name") {
					continue
				}
				role := ""
				if len(cells) > 1 {
					role = plainCell(cells[1])
				}
				out = append(out, npcDraft{Title: name, Role: role})
			}
			return
		}
		switch t := v.(type) {
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			for _, val := range t {
				walk(val)
			}
		}
	}
	walk(block)
	return out
}

func walkNamed(entries any, parent string, depth int, add func(draft), npcs map[string]*npcDraft, split func(map[string]any) bool) {
	for _, item := range asList(entries) {
		m := asMap(item)
		if m == nil {
			continue
		}
		title := asString(m["name"])
		typ := asString(m["type"])
		if title == "" {
			walkNamed(m["entries"], parent, depth+1, add, npcs, split)
			continue
		}
		lower := strings.ToLower(title)
		if lower == "credits" || strings.Contains(lower, "important npc") {
			continue
		}
		if npc, ok := npcs[lower]; ok {
			body := strings.TrimSpace(renderSkipping(m["entries"], split))
			if len(body) > len(npc.Body) {
				npc.Body = body
			}
			if npc.Group == "" {
				npc.Group = parent
			}
			continue
		}
		numbered := numberedRoom.MatchString(title)
		isSection := typ == "section"
		keep := isSection || numbered
		if keep {
			group := ""
			if numbered && parent != "" {
				group = parent
			}
			body := truncate(renderSkipping(m["entries"], split), 6000)
			add(draft{Type: domain.Location, Title: title, Body: body, Tags: []string{"adventure"}, Prefix: "location", Group: group})
		}
		nextParent := title
		if numbered {
			nextParent = parent
		}
		if isSection || numbered || depth < 2 {
			walkNamed(m["entries"], nextParent, depth+1, add, npcs, split)
		}
	}
}

func applyLocationGroups(drafts []draft) {
	hasKids := map[string]bool{}
	for _, d := range drafts {
		if d.Type == domain.Location && d.Group != "" {
			hasKids[strings.ToLower(d.Group)] = true
		}
	}
	for i, d := range drafts {
		if d.Type == domain.Location && d.Group == "" && hasKids[strings.ToLower(d.Title)] {
			drafts[i].Group = d.Title
		}
	}
}

func extractItemNames(v any) []string {
	names := []string{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			if strings.Contains(t, "{@item ") {
				for _, m := range tagPattern.FindAllStringSubmatch(t, -1) {
					if len(m) >= 3 && strings.EqualFold(m[1], "item") {
						parts := splitTagArg(m[2])
						if len(parts) > 0 {
							names = append(names, strings.TrimSpace(parts[0]))
						}
					}
				}
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			if name := asString(t["name"]); name != "" && asString(t["type"]) == "item" {
				names = append(names, name)
			}
			for _, val := range t {
				walk(val)
			}
		}
	}
	walk(v)
	return unique(names)
}

func extractNPCNames(v any) []string {
	names := []string{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			if strings.Contains(t, "{@creature ") {
				for _, m := range tagPattern.FindAllStringSubmatch(t, -1) {
					if len(m) >= 3 && strings.EqualFold(m[1], "creature") {
						parts := splitTagArg(m[2])
						if len(parts) > 0 {
							names = append(names, strings.TrimSpace(parts[0]))
						}
					}
				}
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			if rows := asList(t["rows"]); len(rows) > 0 {
				for _, row := range rows {
					cells := asList(row)
					if len(cells) > 0 {
						if name := strings.TrimSpace(renderValue(cells[0])); name != "" && !strings.EqualFold(name, "name") {
							names = append(names, strings.Trim(name, "@* "))
						}
					}
				}
			}
			for _, val := range t {
				walk(val)
			}
		}
	}
	walk(v)
	return unique(names)
}

func convertBook(data []byte) []draft {
	sections, _ := jsonObjects(data, "data")
	if len(sections) == 0 {
		var envelope map[string]any
		if err := mapJSON(data, &envelope); err == nil {
			sections = mapsFrom(envelope["data"])
		}
	}
	out := make([]draft, 0)
	for _, section := range sections {
		title := asString(section["name"])
		lower := strings.ToLower(title)
		if title == "" || lower == "credits" {
			continue
		}
		body := truncate(renderValue(section["entries"]), 8000)
		out = append(out, draft{
			Type:   domain.Rule,
			Title:  title,
			Body:   body,
			Tags:   []string{"chapter", "rules"},
			Prefix: "chapter",
		})
	}
	return out
}

func mapJSON(data []byte, dest *map[string]any) error {
	return json.Unmarshal(data, dest)
}

func mapsFrom(v any) []map[string]any {
	out := make([]map[string]any, 0)
	for _, item := range asList(v) {
		if m := asMap(item); m != nil {
			out = append(out, m)
		}
	}
	return out
}
