package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

// Suggestion is one completable @ / $ / # choice in the session input.
type Suggestion struct {
	Label  string
	Insert string
	Record *domain.Record
}

func (m *Model) setSessionFocus(pane prefs.Pane) {
	m.layout.Focus = pane
	switch pane {
	case prefs.PaneInput:
		m.sessionInput.Focus()
	default:
		m.sessionInput.Blur()
	}
}

func (m Model) sessionFocus() prefs.Pane {
	if m.layout.Focus == "" {
		return prefs.PaneInput
	}
	return m.layout.Focus
}

func (m Model) panelStyleFor(pane prefs.Pane) lipgloss.Style {
	if m.session != nil && m.sessionFocus() == pane {
		return focusedPanelStyle
	}
	return panelStyle
}

func activeCommandToken(value string) (trigger rune, start int, query string) {
	start = -1
	for index := 0; index < len(value); index++ {
		r := rune(value[index])
		if r == '@' || r == '$' || r == '#' {
			if index == 0 || value[index-1] == ' ' || value[index-1] == '\n' {
				start = index
				trigger = r
			}
		}
	}
	if start < 0 {
		return 0, -1, ""
	}
	return trigger, start, value[start+1:]
}

func (m Model) entitySuggestions(query string) []Suggestion {
	out := make([]Suggestion, 0, 5)
	for _, record := range m.campaignRecords() {
		if record.Authority == domain.Proposal {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(record.Title), query) {
			continue
		}
		candidate := record
		out = append(out, Suggestion{
			Label:  string(record.Type) + "  " + record.Title,
			Insert: "@" + record.Title + " ",
			Record: &candidate,
		})
		if len(out) == 5 {
			break
		}
	}
	return out
}

func (m Model) createSuggestions(query string) []Suggestion {
	types := []struct {
		token string
		label string
	}{
		{"npc", "$npc Name: description"},
		{"character", "$character Name: description"},
		{"location", "$location Name: description"},
		{"item", "$item Name: description"},
		{"faction", "$faction Name: description"},
		{"thread", "$thread Name: description"},
		{"creature", "$creature Name: description"},
		{"event", "$event Name: description"},
		{"note", "$note Name: description"},
	}
	out := make([]Suggestion, 0, len(types))
	for _, item := range types {
		if query != "" && !strings.HasPrefix(item.token, query) && !strings.Contains(strings.ToLower(item.token), query) {
			continue
		}
		out = append(out, Suggestion{Label: item.label, Insert: "$" + item.token + " "})
		if len(out) == 6 {
			break
		}
	}
	return out
}

func (m Model) hashSuggestions(query string) []Suggestion {
	commands := []Suggestion{
		{Label: "#location Name", Insert: "#location "},
		{Label: "#location CURRENTLOCATION", Insert: "#location CURRENTLOCATION"},
		{Label: "#random npc", Insert: "#random npc"},
		{Label: "#random item", Insert: "#random item"},
		{Label: "#random location", Insert: "#random location"},
		{Label: "#d20", Insert: "#d20"},
		{Label: "#d20+5", Insert: "#d20+5"},
		{Label: "#2d6+3", Insert: "#2d6+3"},
		{Label: "#damage 2d6+3", Insert: "#damage 2d6+3"},
	}
	if query == "" {
		return commands
	}
	out := make([]Suggestion, 0, len(commands))
	for _, command := range commands {
		haystack := strings.ToLower(strings.TrimPrefix(command.Insert, "#"))
		if strings.HasPrefix(haystack, query) || strings.Contains(haystack, query) {
			out = append(out, command)
		}
	}
	return out
}

func (m *Model) refreshSuggestions() {
	m.suggestions = nil
	m.suggestion = 0
	value := m.sessionInput.Value()
	if strings.TrimSpace(value) == "" {
		m.suggestions = []Suggestion{
			{Label: "@ entity reference", Insert: "@"},
			{Label: "$ create draft entity", Insert: "$"},
			{Label: "# roll / random / location", Insert: "#"},
		}
	} else {
		trigger, _, query := activeCommandToken(value)
		if trigger != 0 {
			query = strings.ToLower(strings.TrimSpace(query))
			switch trigger {
			case '@':
				m.suggestions = m.entitySuggestions(query)
			case '$':
				m.suggestions = m.createSuggestions(query)
			case '#':
				m.suggestions = m.hashSuggestions(query)
			}
		}
	}
	if len(m.suggestions) > 0 {
		m.sessionInput.SetHeight(2)
	} else {
		m.sessionInput.SetHeight(4)
	}
	m.configureTranscriptViewport()
}

func (m *Model) acceptSuggestion() {
	if len(m.suggestions) == 0 {
		return
	}
	choice := m.suggestions[clamp(m.suggestion, 0, len(m.suggestions)-1)]
	value := m.sessionInput.Value()
	if strings.TrimSpace(value) == "" {
		m.sessionInput.SetValue(choice.Insert)
		m.suggestions = nil
		m.suggestion = 0
		m.setSessionFocus(prefs.PaneInput)
		m.refreshSuggestions()
		return
	}
	_, start, _ := activeCommandToken(value)
	if start < 0 {
		return
	}
	m.sessionInput.SetValue(value[:start] + choice.Insert)
	if choice.Record != nil {
		record := *choice.Record
		m.review = &record
	}
	m.suggestions = nil
	m.suggestion = 0
	m.setSessionFocus(prefs.PaneInput)
	m.refreshSuggestions()
}
