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
			{"Markdown editor", "Ctrl+S save", "Ctrl+T cycle type", "@ reference · Tab insert · Ctrl+O preview", "↑↓ choose · peek PgUp/PgDn", "Esc cancel"},
		}
	case m.planning:
		return [][]string{
			{"Planned notes", "Tab title/body", "Ctrl+S save", "Ctrl+P prior sits", "@ / #location · peek · Ctrl+O preview", "Esc cancel"},
		}
	case m.session != nil:
		return [][]string{
			{"Session", "Enter capture", "Shift+Enter newline", "@ / $ / # suggest · peek · Ctrl+O preview", "Tab cycle panes", "prep pane j/k · PgUp/PgDn · Home/End", "Ctrl+E end", "Ctrl+P upper panes", "- close upper pane"},
		}
	case m.preview != nil:
		return [][]string{
			{"Link preview", "j/k or PgUp/PgDn scroll", "Enter again follow in navigator", "Esc or q return to the list", "Click [open] or [close] · click outside dismisses"},
		}
	case m.playingBack:
		return [][]string{
			{"Playback", "← / h rewind", "→ / l fast-forward", "Home first · End last", "Esc close"},
		}
	case m.namingPicker:
		return [][]string{
			{"Rename vault", "Type a world or campaign name", "Enter save", "Esc cancel"},
		}
	case m.picking:
		return [][]string{
			{"Library", "j/k move", "Enter open", "n new", "e rename", "d delete (y confirm)", "I import", "Esc back to worlds", "q quit"},
		}
	case m.namingFolder:
		return [][]string{
			{"Session folder", "Type a slash path", "Enter save", "Empty unfiles to month groups", "Esc cancel"},
		}
	case m.namingCollection:
		return [][]string{
			{"New collection", "Type a name", "Enter save", "Esc cancel"},
		}
	case m.importing:
		return [][]string{
			{"Import", "Tab sources / 5e.tools / files", "j/k move · Enter caches a 5e.tools adventure (no wiki rows)", "FILES markdown still becomes owner wiki", "type to filter the adventure catalog", "Ctrl+T cycle markdown kind · Ctrl+R refresh catalog", "e enable/disable in campaign", "d remove source (y confirm)", "Esc or b back"},
		}
	case m.reconciling:
		if m.reconEditing {
			return [][]string{
				{"Edit mutation", "Ctrl+S save proposed wiki write", "Esc cancel", "Transcript stays immutable"},
			}
		}
		return [][]string{
			{"Reconciliation", "j/k move", "e edit mutation", "a apply to wiki", "x reject", "Esc close", "Transcript stays immutable"},
		}
	case m.searching:
		return [][]string{
			{"Search", "Type to filter wiki, prep, sessions, notes, recon", "↑↓ select", "Enter open", "Ctrl+S scope", "Ctrl+A AI proposals", "Esc close"},
		}
	default:
		return [][]string{
			{"Browser", "j/k move", "←/→ or Tab panes", "detail PgUp/PgDn scroll · j/k select links", "Enter preview · Enter again follow / playback / expand source folder", "Link rows show why they appear", "n new · e edit · I import", "p prep · s live", "d delete · x supersede", "m session folder", "f tag · c collection · g new collection · a add", "o scope · b library · / search", "Sources: chapters in the list · Enter expands names · detail shows that slice", "@ peeks wiki then enabled adventure reference", "r reconcile · q quit"},
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
