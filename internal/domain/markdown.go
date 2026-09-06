package domain

import (
	"fmt"
	"strings"
)

// FormatEntityMarkdown renders a record as an editable markdown document.
func FormatEntityMarkdown(record Record) string {
	var builder strings.Builder
	builder.WriteString("type: ")
	builder.WriteString(string(record.Type))
	builder.WriteString("\n")
	if record.Authority != "" && record.Authority != Draft {
		builder.WriteString("authority: ")
		builder.WriteString(string(record.Authority))
		builder.WriteString("\n")
	}
	builder.WriteString("\n# ")
	builder.WriteString(record.Title)
	builder.WriteString("\n")
	if strings.TrimSpace(record.Summary) != "" {
		builder.WriteString("\n")
		builder.WriteString(strings.TrimSpace(record.Summary))
		builder.WriteString("\n")
	}
	if strings.TrimSpace(record.Body) != "" {
		builder.WriteString("\n")
		builder.WriteString(strings.TrimRight(record.Body, "\n"))
		builder.WriteString("\n")
	}
	return builder.String()
}

// ParseEntityMarkdown extracts type/title/summary/body from a markdown entity doc.
// Metadata lines (type:, authority:) may appear before the first heading.
func ParseEntityMarkdown(text string, fallbackType EntityType) (Record, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	entityType := fallbackType
	if entityType == "" {
		entityType = NPC
	}
	authority := Draft
	index := 0
	for index < len(lines) {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" {
			index++
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "type:"):
			value := strings.TrimSpace(trimmed[len("type:"):])
			if parsed, ok := parseEntityTypeToken(value); ok {
				entityType = parsed
			}
			index++
			continue
		case strings.HasPrefix(lower, "authority:"):
			value := strings.ToLower(strings.TrimSpace(trimmed[len("authority:"):]))
			if parsed, ok := parseAuthorityToken(value); ok {
				authority = parsed
			}
			index++
			continue
		}
		break
	}

	title := ""
	for index < len(lines) {
		trimmed := strings.TrimSpace(lines[index])
		index++
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "##") {
			title = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			break
		}
	}
	if title == "" {
		return Record{}, fmt.Errorf("markdown entity needs a # Title heading")
	}

	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}

	summary := ""
	bodyStart := index
	if index < len(lines) {
		var summaryLines []string
		for index < len(lines) {
			trimmed := strings.TrimSpace(lines[index])
			if trimmed == "" {
				index++
				break
			}
			if strings.HasPrefix(trimmed, "#") {
				break
			}
			summaryLines = append(summaryLines, trimmed)
			index++
		}
		summary = strings.Join(summaryLines, " ")
		bodyStart = index
	}
	for bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) == "" {
		bodyStart++
	}
	body := strings.TrimRight(strings.Join(lines[bodyStart:], "\n"), "\n")
	if body != "" {
		body += "\n"
	}

	return Record{
		Type:      entityType,
		Title:     title,
		Summary:   summary,
		Body:      body,
		Authority: authority,
	}, nil
}

func parseEntityTypeToken(value string) (EntityType, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", "_"))
	switch normalized {
	case "NPC":
		return NPC, true
	case "CHARACTER":
		return Character, true
	case "LOCATION":
		return Location, true
	case "FACTION":
		return Faction, true
	case "ITEM":
		return Item, true
	case "CREATURE":
		return Creature, true
	case "THREAD":
		return Thread, true
	case "SESSION":
		return Session, true
	case "SCENE":
		return Scene, true
	case "EVENT":
		return Event, true
	case "NOTE":
		return Note, true
	case "RULE":
		return Rule, true
	default:
		return "", false
	}
}

func parseAuthorityToken(value string) (Authority, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "canon":
		return Canon, true
	case "secret":
		return Secret, true
	case "draft":
		return Draft, true
	case "proposal", "ai proposal":
		return Proposal, true
	case "unknown":
		return Unknown, true
	case "superseded":
		return Superseded, true
	default:
		return "", false
	}
}
