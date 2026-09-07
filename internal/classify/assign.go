package classify

import (
	"regexp"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

var numberedHeading = regexp.MustCompile(`^(\d+)(?:\.(\s+|$)|$)`)

// FrontMatter reports titles that are book apparatus, not sites. This list is
// the same across 5e adventures; it is not a catalog of one module's scenes.
func FrontMatter(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	if lower == "" {
		return false
	}
	switch lower {
	case "introduction", "intro", "preface", "foreword", "prologue",
		"contents", "table of contents", "credits", "overview", "afterword",
		"epilogue":
		return true
	}
	return strings.HasPrefix(lower, "appendix")
}

func NumberedRoom(title string) bool {
	return numberedHeading.MatchString(strings.TrimSpace(title))
}

func NPCRosterTitle(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "important npc") {
		return true
	}
	switch lower {
	case "npcs", "npc", "cast", "dramatis personae":
		return true
	}
	return false
}

func UnderFrontMatter(group string) bool {
	group = strings.Trim(group, "/")
	if group == "" {
		return false
	}
	first, _, _ := strings.Cut(group, "/")
	return FrontMatter(first)
}

// Assign picks a wiki type from structure: front matter, numbered rooms,
// 5e.tools section nodes, and chapter (no parent). Advice boxes stay notes.
func Assign(title, group string, tags []string) domain.EntityType {
	if FrontMatter(title) || UnderFrontMatter(group) {
		return domain.Note
	}
	if NumberedRoom(title) {
		return domain.Location
	}
	if hasTag(tags, "5e-section") || strings.TrimSpace(group) == "" {
		return domain.Location
	}
	return domain.Note
}

func PrefixFor(typ domain.EntityType) string {
	switch typ {
	case domain.Location:
		return "location"
	case domain.NPC:
		return "npc"
	case domain.Item:
		return "item"
	case domain.Creature:
		return "creature"
	case domain.Note:
		return "note"
	default:
		return strings.ToLower(string(typ))
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}
