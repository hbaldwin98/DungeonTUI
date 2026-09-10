package domain

import (
	"fmt"
	"strings"
)

type EntityScopeLevel string

const (
	CampaignScope EntityScopeLevel = "campaign"
	WorldScope    EntityScopeLevel = "world"
)

type ParsedEntityMarkdown struct {
	Record     Record
	ScopeLevel EntityScopeLevel
}

// FormatEntityMarkdown renders a record as an editable markdown document.
func FormatEntityMarkdown(record Record) string {
	var builder strings.Builder
	builder.WriteString("type: ")
	builder.WriteString(string(record.Type))
	builder.WriteString("\n")
	authority := record.Authority
	if authority == "" {
		authority = Draft
	}
	builder.WriteString("authority: ")
	builder.WriteString(string(authority))
	builder.WriteString("\n")
	builder.WriteString("scope: ")
	if record.Scope.WorldID != "" && record.Scope.CampaignID == "" {
		builder.WriteString(string(WorldScope))
	} else {
		builder.WriteString(string(CampaignScope))
	}
	builder.WriteString("\n")
	if record.Type == Thread && record.ThreadState != "" {
		builder.WriteString("state: ")
		builder.WriteString(string(record.ThreadState))
		builder.WriteString("\n")
	}
	if len(record.Tags) > 0 {
		builder.WriteString("tags: ")
		builder.WriteString(strings.Join(record.Tags, ", "))
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

// ParseEntityMarkdown extracts record fields and authoring scope from a markdown entity doc.
func ParseEntityMarkdown(text string, fallbackType EntityType) (ParsedEntityMarkdown, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	entityType := fallbackType
	if entityType == "" {
		entityType = NPC
	}
	authority := Draft
	scopeLevel := CampaignScope
	var tags []string
	var threadState ThreadState
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
			parsed, ok := parseEntityTypeToken(value)
			if !ok {
				return ParsedEntityMarkdown{}, fmt.Errorf("unknown entity type %q", value)
			}
			entityType = parsed
			index++
			continue
		case strings.HasPrefix(lower, "authority:"):
			value := strings.ToLower(strings.TrimSpace(trimmed[len("authority:"):]))
			parsed, ok := parseAuthorityToken(value)
			if !ok {
				return ParsedEntityMarkdown{}, fmt.Errorf("unknown authority %q", value)
			}
			authority = parsed
			index++
			continue
		case strings.HasPrefix(lower, "scope:"):
			value := strings.ToLower(strings.TrimSpace(trimmed[len("scope:"):]))
			switch EntityScopeLevel(value) {
			case CampaignScope, WorldScope:
				scopeLevel = EntityScopeLevel(value)
			default:
				return ParsedEntityMarkdown{}, fmt.Errorf("unknown scope %q; use campaign or world", value)
			}
			index++
			continue
		case strings.HasPrefix(lower, "state:"):
			value := strings.TrimSpace(trimmed[len("state:"):])
			parsed, ok := ParseThreadState(value)
			if !ok {
				return ParsedEntityMarkdown{}, fmt.Errorf("unknown thread state %q; use open, advancing, dormant, or resolved", value)
			}
			threadState = parsed
			index++
			continue
		case strings.HasPrefix(lower, "tags:"):
			tags = parseTagList(trimmed[len("tags:"):])
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
		return ParsedEntityMarkdown{}, fmt.Errorf("markdown entity needs a # Title heading")
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

	if entityType != Thread {
		// State is thread vocabulary; on any other type it would be invisible
		// data, so it is dropped rather than stored.
		threadState = ""
	}
	return ParsedEntityMarkdown{
		Record: Record{
			ThreadState: threadState,
			Type:        entityType,
			Title:       title,
			Summary:     summary,
			Body:        body,
			Authority:   authority,
			Tags:        tags,
		},
		ScopeLevel: scopeLevel,
	}, nil
}

func parseTagList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	return out
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
