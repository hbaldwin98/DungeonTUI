package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
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
			{"Markdown editor", "Ctrl+N quick capture", "Ctrl+S save", "Ctrl+T cycle type", "@ reference · Tab insert · Ctrl+O preview", "↑↓ choose · peek PgUp/PgDn", "Esc cancel"},
		}
	case m.planning:
		return [][]string{
			{"Planned notes", "Ctrl+N quick capture", "Tab title/body", "Ctrl+S save", "Ctrl+P prior sits", "@ / #location · peek · Ctrl+O preview", "Esc cancel"},
		}
	case m.preview != nil:
		if m.session != nil {
			return [][]string{
				{"Live inspection", "j/k or PgUp/PgDn scroll", "Enter, Esc, or q returns to capture"},
			}
		}
		return [][]string{
			{"Link preview", "j/k or PgUp/PgDn scroll", "Enter again follow in navigator", "Esc or q return to the list", "Click [open] or [close] · click outside dismisses"},
		}
	case m.playingBack:
		return [][]string{
			{"Playback", "Ctrl+N quick capture", "← / h rewind", "→ / l fast-forward", "Home first · End last", "Esc close"},
		}
	case m.namingPicker:
		return [][]string{
			{"Rename vault", "Type a world or campaign name", "Enter save", "Esc cancel"},
		}
	case m.picking:
		return [][]string{
			{"Library", ": search commands", "j/k move", "Enter open", "n new", "e rename", "d delete (y confirm)", "I import", "Esc back to worlds", "q quit"},
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
			{"Reconciliation", ": search commands", "Ctrl+N quick capture", "j/k move", "e edit mutation", "a accept", "d defer", "x reject", "Esc close", "Transcript stays immutable"},
		}
	case m.settingsOpen:
		return [][]string{
			{"Settings", "Personal preferences for this machine", "5e path locates the external 5e binary", "Empty falls back to DUNGEON_5E_BIN, then PATH", "Enter save and check", "Esc close"},
		}
	case m.searching:
		if m.searchScope == searchsvc.RulesReference {
			return [][]string{
				{"Search 5e rules", "Ctrl+S cycles campaign/world/library/5e rules", "Type to query the external 5e index", "Ctrl+G set the 5e binary path", "↑↓ select", "Enter inspect", "Esc close"},
			}
		}
		return [][]string{
			{"Search", "Ctrl+N quick capture", "Type to filter wiki, prep, sessions, notes, recon", "↑↓ select", "Enter inspect", "Ctrl+S scope", "Ctrl+A AI proposals", "Esc close"},
		}
	case m.session != nil:
		return [][]string{
			{"Session", "Enter capture", "Ctrl+N unclassified quick capture", "Shift+Enter newline", "/ search · @ / $ / # suggest · peek · Ctrl+O preview", "Tab cycle panes", "context p pin · x clear", "prep pane j/k beats · ↑/↓ or PgUp/PgDn scroll · d done · x skip", "Ctrl+E end", "Ctrl+P upper panes", "- close upper pane"},
		}
	default:
		return [][]string{
			{"Browser", ": search commands", "Ctrl+N quick capture (: files it)", "j/k move", "←/→ or Tab panes", "detail PgUp/PgDn scroll · j/k select links", "Enter preview · Enter again follow / playback / expand source folder", "Link rows show why they appear", "n new · e edit · I import", "p prep · s live", "d delete · x supersede", "m session folder", "f tag · c collection · g new collection · a add", "o scope · b library · / search", "Sources: chapters in the list · Enter expands names · detail shows that slice", "@ peeks wiki then enabled adventure reference", "r reconcile · q quit"},
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
	)
}
