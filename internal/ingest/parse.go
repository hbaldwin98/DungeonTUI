package ingest

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

var (
	headingLine   = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	rdName        = regexp.MustCompile(`data-rd-name="([^"]+)"`)
	loadingLine   = regexp.MustCompile(`(?i)Loading\s+".*?"\s*\.\.\.`)
	numberedRoom  = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
	italicItem    = regexp.MustCompile(`^\s*[-*]\s+\*(.+?)\*\s*$`)
	editionParens = regexp.MustCompile(`\((\d{4})\)`)
	tableRow      = regexp.MustCompile(`^\|([^|]+)\|([^|]+)\|\s*$`)
	tableDivider  = regexp.MustCompile(`^\|[\s:|-]+\|[\s:|-]+\|\s*$`)
	boldName      = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	letterIndexH2 = regexp.MustCompile(`(?i)^monsters\s+\([a-z]\)$`)
)

// ParsedBook is the deterministic extract of a 5etools-style markdown export.
type ParsedBook struct {
	Title   string
	Edition string
	Kind    domain.SourceKind
	Entries []ParsedEntry
	Aliases map[string][]string // heading title → extra names from embeds
}

// ParsedEntry is one heading-sized chunk after HTML is stripped.
type ParsedEntry struct {
	Level int
	Title string
	Path  []string
	Body  string
}

func Parse(markdown string, kind domain.SourceKind) ParsedBook {
	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	aliases := collectEmbedAliases(text)
	plain := stripHTML(text)
	plain = loadingLine.ReplaceAllString(plain, "")
	book := ParsedBook{
		Title:   firstHeading(plain, 1),
		Kind:    kind,
		Aliases: aliases,
	}
	if book.Title == "" {
		book.Title = "Imported source"
	}
	if kind == "" {
		book.Kind = detectKind(book.Title, text)
	}
	if m := editionParens.FindStringSubmatch(book.Title); len(m) > 1 {
		book.Edition = m[1]
	}
	book.Entries = splitHeadings(plain)
	return book
}

func detectKind(title, raw string) domain.SourceKind {
	lower := strings.ToLower(title + "\n" + raw[:min(len(raw), 2000)])
	switch {
	case strings.Contains(lower, "monster manual") || strings.Count(raw, "data-statblock-hash=") > 80:
		return domain.SourceBestiary
	case strings.Contains(lower, "player's handbook") || strings.Contains(lower, "players handbook"):
		return domain.SourceRules
	default:
		return domain.SourceAdventure
	}
}

func firstHeading(text string, level int) string {
	prefix := strings.Repeat("#", level) + " "
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) && !strings.HasPrefix(line, prefix+"#") {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
		if level == 1 && strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func splitHeadings(text string) []ParsedEntry {
	lines := strings.Split(text, "\n")
	type mark struct {
		line  int
		level int
		title string
	}
	var marks []mark
	for i, line := range lines {
		m := headingLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		marks = append(marks, mark{line: i, level: len(m[1]), title: strings.TrimSpace(m[2])})
	}
	out := make([]ParsedEntry, 0, len(marks))
	var stack []string
	for i, m := range marks {
		end := len(lines)
		if i+1 < len(marks) {
			end = marks[i+1].line
		}
		body := strings.TrimSpace(strings.Join(lines[m.line+1:end], "\n"))
		for len(stack) >= m.level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, m.title)
		path := append([]string(nil), stack...)
		out = append(out, ParsedEntry{Level: m.level, Title: m.title, Path: path, Body: body})
	}
	return out
}

func collectEmbedAliases(raw string) map[string][]string {
	out := map[string][]string{}
	current := ""
	for _, line := range strings.Split(raw, "\n") {
		if m := headingLine.FindStringSubmatch(line); m != nil {
			current = strings.TrimSpace(m[2])
			continue
		}
		if current == "" {
			continue
		}
		name := ""
		if m := rdName.FindStringSubmatch(line); m != nil {
			name = strings.TrimSpace(m[1])
		}
		if name == "" || strings.EqualFold(name, current) {
			continue
		}
		name = strings.TrimSuffix(name, " (Info)")
		if strings.EqualFold(name, current) {
			continue
		}
		seen := false
		for _, existing := range out[current] {
			if strings.EqualFold(existing, name) {
				seen = true
				break
			}
		}
		if !seen {
			out[current] = append(out[current], name)
		}
	}
	return out
}

func stripHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func slug(parts ...string) string {
	var b strings.Builder
	for _, part := range parts {
		cleaned := strings.Map(func(r rune) rune {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				return unicode.ToLower(r)
			case r == ' ' || r == '-' || r == '_' || r == '\'':
				return '-'
			default:
				return -1
			}
		}, part)
		cleaned = strings.Trim(cleaned, "-")
		for strings.Contains(cleaned, "--") {
			cleaned = strings.ReplaceAll(cleaned, "--", "-")
		}
		if cleaned == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
		b.WriteString(cleaned)
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

func summaryOf(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ">") || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "!") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		return truncate(line, 220)
	}
	return ""
}

func isSkippedBestiary(title string) bool {
	switch strings.ToLower(title) {
	case "stat block overview", "monster entries", "parts of a stat block",
		"how to use a monster", "habitat", "treasure", "narrative description",
		"special lairs", "stat blocks", "credits", "animals", "monster lists":
		return true
	}
	return letterIndexH2.MatchString(title)
}

func isSkippedAdventure(title string) bool {
	lower := strings.ToLower(title)
	return lower == "credits" || strings.HasPrefix(lower, "appendix")
}

func parseNPCTable(body string) [][2]string {
	var rows [][2]string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if tableDivider.MatchString(line) {
			continue
		}
		m := tableRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := strings.TrimSpace(m[1])
		note := strings.TrimSpace(m[2])
		name = strings.Trim(name, "*")
		if name == "" || strings.EqualFold(name, "name") {
			continue
		}
		rows = append(rows, [2]string{name, note})
	}
	return rows
}

func parseItemList(body string) []string {
	var items []string
	for _, line := range strings.Split(body, "\n") {
		m := italicItem.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		items = append(items, strings.TrimSpace(m[1]))
	}
	return items
}

func roomTitle(site, heading string) string {
	if site == "" || strings.EqualFold(site, heading) {
		return heading
	}
	return site + " — " + heading
}

func firstNumberedParent(path []string) string {
	for i := len(path) - 2; i >= 0; i-- {
		if numberedRoom.MatchString(path[i]) {
			continue
		}
		if path[i] != "" {
			return path[i]
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}
