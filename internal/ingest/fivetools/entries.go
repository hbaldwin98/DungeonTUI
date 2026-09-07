package fivetools

import (
	"fmt"
	"regexp"
	"strings"
)

var tagPattern = regexp.MustCompile(`\{@([a-zA-Z0-9]+)(?:\s+([^}]*))?\}`)

func renderValue(v any) string {
	return renderSkipping(v, nil)
}

func renderSkipping(v any, skip func(map[string]any) bool) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return renderTags(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if m := asMap(item); m != nil && skip != nil && skip(m) {
				continue
			}
			if s := strings.TrimSpace(renderSkipping(item, skip)); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n\n")
	case map[string]any:
		if skip != nil && skip(t) {
			return ""
		}
		return renderBlock(t, skip)
	default:
		return renderTags(asString(v))
	}
}

func renderBlock(block map[string]any, skip func(map[string]any) bool) string {
	typ := asString(block["type"])
	name := asString(block["name"])
	switch typ {
	case "image", "gallery", "inlineToken":
		return ""
	case "list":
		var b strings.Builder
		for _, item := range asList(block["items"]) {
			line := strings.TrimSpace(renderSkipping(item, skip))
			if line == "" {
				continue
			}
			for i, part := range strings.Split(line, "\n") {
				if i == 0 {
					b.WriteString("- ")
					b.WriteString(part)
				} else {
					b.WriteString("\n  ")
					b.WriteString(part)
				}
			}
			b.WriteString("\n")
		}
		return strings.TrimRight(b.String(), "\n")
	case "table":
		return renderTable(block)
	case "inset", "insetReadaloud", "quote":
		body := renderSkipping(block["entries"], skip)
		if body == "" {
			body = renderSkipping(block["text"], skip)
		}
		if name != "" {
			body = "*" + name + "*\n\n" + body
		}
		return prefixLines(body, "> ")
	case "statblock":
		title := firstNonEmpty(asString(block["name"]), asString(block["tag"]))
		if title == "" {
			return ""
		}
		return "@" + title
	case "section", "entries", "item", "itemSpell", "homebrew":
		body := renderSkipping(block["entries"], skip)
		if name == "" {
			return body
		}
		return "**" + name + ".** " + body
	default:
		if entries := block["entries"]; entries != nil {
			body := renderSkipping(entries, skip)
			if name == "" {
				return body
			}
			return "**" + name + ".** " + body
		}
		if name != "" {
			return renderTags(name)
		}
		return ""
	}
}

func renderTable(block map[string]any) string {
	labels := asList(block["colLabels"])
	if len(labels) == 0 {
		labels = asList(block["colLabel"])
	}
	rows := asList(block["rows"])
	if len(labels) == 0 && len(rows) == 0 {
		return ""
	}
	width := len(labels)
	if width == 0 {
		if first := asList(rows[0]); len(first) > 0 {
			width = len(first)
		} else {
			return ""
		}
	}
	var b strings.Builder
	if cap := asString(block["caption"]); cap != "" {
		b.WriteString("*" + cap + "*\n")
	}
	if len(labels) > 0 {
		b.WriteString("|")
		for _, label := range labels {
			b.WriteString(" ")
			b.WriteString(plainCell(label))
			b.WriteString(" |")
		}
		b.WriteByte('\n')
		b.WriteString("|")
		for range labels {
			b.WriteString(" --- |")
		}
		b.WriteByte('\n')
	}
	for _, row := range rows {
		cells := asList(row)
		if len(cells) == 0 {
			cells = []any{row}
		}
		b.WriteString("|")
		for i := 0; i < width; i++ {
			cell := ""
			if i < len(cells) {
				cell = plainCell(cells[i])
			}
			b.WriteString(" ")
			b.WriteString(cell)
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func plainCell(v any) string {
	s := renderValue(v)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}

func prefixLines(s, prefix string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func renderTags(s string) string {
	if !strings.Contains(s, "{@") {
		return s
	}
	return tagPattern.ReplaceAllStringFunc(s, func(raw string) string {
		m := tagPattern.FindStringSubmatch(raw)
		if len(m) < 2 {
			return raw
		}
		return renderTag(m[1], strings.TrimSpace(m[2]))
	})
}

func renderTag(kind, arg string) string {
	kind = strings.ToLower(kind)
	parts := splitTagArg(arg)
	head := ""
	if len(parts) > 0 {
		head = parts[0]
	}
	display := head
	if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
		display = parts[2]
	}
	switch kind {
	case "atk":
		return attackLabel(head)
	case "hit":
		if strings.HasPrefix(head, "-") || strings.HasPrefix(head, "+") {
			return head
		}
		return "+" + head
	case "h", "hom":
		return "Hit:"
	case "damage", "dicedisplay", "dice", "scaledamage", "scaledice":
		return head
	case "dc":
		return "DC " + head
	case "recharge":
		if head == "" {
			return "(Recharge 6)"
		}
		return "(Recharge " + head + ")"
	case "chance":
		return head + "%"
	case "filter", "link", "loader", "book", "adventure", "quickref":
		return display
	case "b":
		return "**" + display + "**"
	case "i":
		return "*" + display + "*"
	case "u":
		return display
	case "s":
		return "~~" + display + "~~"
	case "note", "footnote":
		return "(" + display + ")"
	case "creature", "spell", "item", "class", "race", "feat", "background",
		"optfeature", "condition", "disease", "status", "hazard", "trap",
		"vehicle", "object", "deity", "reward", "variantrule", "action",
		"sense", "skill", "language", "table", "card":
		name := strings.TrimSpace(head)
		if name == "" {
			return display
		}
		return "@" + name
	default:
		if display != "" {
			return display
		}
		return head
	}
}

func splitTagArg(arg string) []string {
	if arg == "" {
		return nil
	}
	return strings.Split(arg, "|")
}

func attackLabel(code string) string {
	parts := strings.Split(strings.ToLower(code), ",")
	var labels []string
	for _, part := range parts {
		switch strings.TrimSpace(part) {
		case "mw":
			labels = append(labels, "Melee Weapon Attack:")
		case "rw":
			labels = append(labels, "Ranged Weapon Attack:")
		case "ms":
			labels = append(labels, "Melee Spell Attack:")
		case "rs":
			labels = append(labels, "Ranged Spell Attack:")
		case "m":
			labels = append(labels, "Melee Attack:")
		case "r":
			labels = append(labels, "Ranged Attack:")
		}
	}
	if len(labels) == 0 {
		return "Attack:"
	}
	return strings.Join(labels, " or ")
}

func abilityMod(score int) string {
	mod := (score - 10) / 2
	return fmt.Sprintf("%+d", mod)
}
