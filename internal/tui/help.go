package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) updateHelp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?":
		m.helping = false
		m.status = ""
		return m, nil
	}
	return m, nil
}

func (m Model) openHelp() (tea.Model, tea.Cmd) {
	m.helping = true
	m.status = ""
	return m, nil
}

func (m Model) helpSections() [][]string {
	switch {
	case m.editing:
		return [][]string{
			{"Markdown editor", "Ctrl+S save", "Ctrl+T cycle type", "@ reference · Tab insert", "↑↓ choose · peek PgUp/PgDn", "Esc cancel"},
		}
	case m.planning:
		return [][]string{
			{"Planned notes", "Tab title/body", "Ctrl+S save", "Ctrl+P prior sits", "@ / #location · peek", "Esc cancel"},
		}
	case m.session != nil:
		return [][]string{
			{"Session", "Enter capture", "Shift+Enter newline", "@ / $ / # suggest · peek", "Tab cycle panes", "Ctrl+E end", "Ctrl+P upper panes", "- close upper pane"},
		}
	case m.playingBack:
		return [][]string{
			{"Playback", "← / h rewind", "→ / l fast-forward", "Home first · End last", "Esc close"},
		}
	case m.searching:
		return [][]string{
			{"Search", "Type to filter", "↑↓ select", "Enter open", "Ctrl+S scope", "Ctrl+A AI proposals", "Esc close"},
		}
	default:
		return [][]string{
			{"Browser", "j/k move", "←/→ or Tab panes", "Enter open/playback", "n new · e edit", "p prep · s live", "d delete · x supersede", "f tag filter · o scope", "b library · / search", "r reconcile · q quit"},
		}
	}
}

func (m Model) renderHelpOverlay() string {
	width := min(72, max(48, m.width-8))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("COMMANDS"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("? or Esc closes"))
	builder.WriteString("\n")
	for _, section := range m.helpSections() {
		if len(section) == 0 {
			continue
		}
		builder.WriteString("\n")
		builder.WriteString(sectionStyle.Render(section[0]))
		builder.WriteString("\n")
		for _, line := range section[1:] {
			builder.WriteString(searchResultStyle.Render("  " + line))
			builder.WriteString("\n")
		}
	}
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
	)
}
