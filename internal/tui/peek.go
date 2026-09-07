package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

// referenceAtCursor finds a simple @token (no spaces) containing the cursor.
func referenceAtCursor(line string, col int) (text string, ok bool) {
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	start := -1
	for i := min(col, len(line)); i >= 0; i-- {
		if i < len(line) && line[i] == '@' && (i == 0 || isTokenBoundary(line[i-1])) {
			start = i
			break
		}
		if i < col && i < len(line) && isTokenBoundary(line[i]) {
			break
		}
		if i == 0 {
			break
		}
	}
	if start < 0 {
		if col < len(line) && line[col] == '@' && (col == 0 || isTokenBoundary(line[col-1])) {
			start = col
		} else {
			return "", false
		}
	}
	end := start + 1
	for end < len(line) {
		r := rune(line[end])
		if unicode.IsSpace(r) || r == ',' || r == ';' {
			break
		}
		end++
	}
	text = strings.TrimSpace(line[start+1 : end])
	if text == "" {
		return "", false
	}
	return text, true
}

func isTokenBoundary(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t'
}

func (m *Model) refreshPeek() {
	m.peek = nil
	m.peekScroll = 0

	if len(m.suggestions) > 0 {
		choice := m.suggestions[clamp(m.suggestion, 0, len(m.suggestions)-1)]
		if choice.Record != nil {
			record := *choice.Record
			m.peek = &record
			return
		}
	}

	value := ""
	line := 0
	col := 0
	switch {
	case m.session != nil && m.sessionFocus() == prefs.PaneInput:
		value = m.sessionInput.Value()
		line = m.sessionInput.Line()
		col = m.sessionInput.Column()
	case m.editing:
		value = m.editBody.Value()
		line = m.editBody.Line()
		col = m.editBody.Column()
	case m.planning && m.planField == 1:
		value = m.planBody.Value()
		line = m.planBody.Line()
		col = m.planBody.Column()
	default:
		return
	}
	lines := strings.Split(value, "\n")
	if line < 0 || line >= len(lines) {
		return
	}
	m.peek = m.resolveReferenceAtCursor(lines[line], col)
}

// resolveReferenceAtCursor resolves a multi-word @ reference under the cursor.
func (m Model) resolveReferenceAtCursor(line string, col int) *domain.Record {
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}
	start := -1
	limit := col
	if limit < len(line) {
		limit++
	}
	for i := 0; i < limit && i < len(line); i++ {
		if line[i] == '@' && (i == 0 || isTokenBoundary(line[i-1])) {
			start = i
		}
	}
	if start < 0 || col < start {
		return nil
	}
	remaining := line[start+1:]
	var best *domain.Record
	bestLen := 0
	for _, record := range m.campaignRecords() {
		title := record.Title
		if title == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(remaining), strings.ToLower(title)) {
			// Allow incomplete typed prefix while the cursor is still in the token.
			if strings.HasPrefix(strings.ToLower(title), strings.ToLower(remaining)) && col <= len(line) {
				candidate := record
				if len(remaining) >= bestLen {
					best = &candidate
					bestLen = len(remaining)
				}
			}
			continue
		}
		refEnd := start + 1 + len(title)
		if refEnd < len(line) {
			next := line[refEnd]
			if !isTokenBoundary(next) && next != ',' && next != ';' && next != '.' && next != '!' && next != '?' {
				continue
			}
		}
		if col <= refEnd {
			candidate := record
			if len(title) >= bestLen {
				best = &candidate
				bestLen = len(title)
			}
		}
	}
	if best != nil {
		return best
	}
	token := domain.ScanMentionName(remaining)
	if token == "" {
		return nil
	}
	if rec, ok := m.lookupRuleset(token); ok {
		return &rec
	}
	return nil
}

func (m Model) renderPeekPanel(maxLines int) string {
	if m.peek == nil {
		return ""
	}
	record := m.peek
	sessions := domain.CountSessionsTouching(m.workspace, record.ID)
	var lines []string
	lines = append(lines, sectionStyle.Render("PEEK")+"  "+typeStyle.Render(string(record.Type))+"  "+
		authorityStyle(record.Authority).Render(record.Authority.Marker()+" "+record.Authority.Label()))
	lines = append(lines, detailTitleStyle.Render(record.Title))
	if record.Summary != "" {
		lines = append(lines, strings.Split(m.renderMarkdown(record.Summary, max(24, m.width-16)), "\n")...)
	}
	if fivetools.IsPluginID(record.ID) {
		if record.Source != "" {
			lines = append(lines, mutedStyle.Render("5e · "+record.Source))
		}
		if strings.TrimSpace(record.Body) != "" {
			lines = append(lines, strings.Split(m.renderMarkdown(record.Body, max(24, m.width-16)), "\n")...)
		}
	}
	if len(record.Tags) > 0 {
		lines = append(lines, mutedStyle.Render("#"+strings.Join(record.Tags, "  #")))
	}
	lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d sessions · scroll PgUp/PgDn", sessions)))
	if maxLines < 1 {
		maxLines = 6
	}
	scroll := m.peekScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(lines) {
		scroll = max(0, len(lines)-1)
	}
	end := min(len(lines), scroll+maxLines)
	return strings.Join(lines[scroll:end], "\n")
}

func (m *Model) scrollPeek(delta int) {
	if m.peek == nil {
		return
	}
	m.peekScroll = max(0, m.peekScroll+delta)
}
