package tui

import (
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/fivecli"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

// Settings hold personal, machine-local configuration in preferences.json.
// Nothing here is campaign content, so it never reaches the workspace store.

func (m Model) openSettings() (tea.Model, tea.Cmd) {
	m.settingsOpen = true
	m.settingsFromSearch = m.searching
	if m.searching {
		m.searchInput.Blur()
	}
	m.settingsInput.SetValue(strings.TrimSpace(m.layout.FiveCLIBinary))
	m.settingsInput.SetWidth(max(24, min(64, m.width-18)))
	m.settingsInput.CursorEnd()
	m.settingsInput.Focus()
	m.status = "Settings · Enter save and check · Esc close"
	return m, tea.Batch(textinput.Blink, m.ensureRulesStatus())
}

func (m *Model) closeSettings() {
	m.settingsOpen = false
	m.settingsInput.Blur()
	if m.settingsFromSearch {
		m.searchInput.Focus()
	} else if m.session != nil {
		m.setSessionFocus(m.sessionFocus())
	}
	m.settingsFromSearch = false
}

func (m Model) updateSettings(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeSettings()
		m.status = "Closed settings"
		return m, nil
	case "enter":
		return m.saveFiveCLIBinary()
	}
	var cmd tea.Cmd
	m.settingsInput, cmd = m.settingsInput.Update(msg)
	return m, cmd
}

// saveFiveCLIBinary persists the configured path and re-diagnoses it. The
// diagnosis runs as a command so an unreachable binary cannot block the TUI.
func (m Model) saveFiveCLIBinary() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.settingsInput.Value())
	m.layout.FiveCLIBinary = value
	m.persistPreferences()
	m.rulesStatus = rulesStatus{}
	cmds := []tea.Cmd{m.rulesStatusCmd()}
	if value == "" {
		m.status = "5e path cleared · using DUNGEON_5E_BIN or PATH"
	} else {
		m.status = "Saved 5e path · " + value
	}
	// A rules search already on screen should reflect the new binary.
	if m.searching && m.searchScope == searchsvc.RulesReference {
		if refresh := m.scheduleRulesSearch(); refresh != nil {
			cmds = append(cmds, refresh)
		}
	}
	return m, tea.Batch(cmds...)
}

// fivecliSettingLine describes the configured binary and where it came from.
func (m Model) fivecliSettingLine() string {
	if binary := strings.TrimSpace(m.layout.FiveCLIBinary); binary != "" {
		return "Configured: " + binary
	}
	if env := strings.TrimSpace(os.Getenv(fivecli.BinaryEnv)); env != "" {
		return "Configured: (unset) · using " + fivecli.BinaryEnv + "=" + env
	}
	return "Configured: (unset) · looking for " + fivecli.DefaultBinary + " on PATH"
}

func (m Model) renderSettingsOverlay(background string) string {
	width := min(76, max(44, m.width-12))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("SETTINGS"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("personal · stored in preferences.json"))
	builder.WriteString("\n\n")
	builder.WriteString(sectionStyle.Render("5E RULES LOOKUP"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render("Path to the external 5e binary. Empty falls back to " + fivecli.BinaryEnv + ", then PATH."))
	builder.WriteString("\n")
	builder.WriteString(m.settingsInput.View())
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(truncateImportLine(m.fivecliSettingLine(), max(1, width-4))))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(truncateImportLine(m.fivecliStatusLine(), max(1, width-4))))
	builder.WriteString("\n\n")
	builder.WriteString(helpStyle.Render("Enter save and check  Esc close"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	if background == "" {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
		)
	}
	w := lipgloss.Width(overlay)
	h := lipgloss.Height(overlay)
	base := lipgloss.NewLayer(background).X(0).Y(0).Z(0)
	float := lipgloss.NewLayer(overlay).X(max(0, (m.width-w)/2)).Y(max(0, (m.height-h)/2)).Z(1)
	return lipgloss.NewCompositor(base, float).Render()
}

// fivecliStatusLine reports the cached diagnosis without running the tool.
func (m Model) fivecliStatusLine() string {
	if !m.rulesStatus.loaded || m.rulesStatus.binary != m.fivecliBinaryKey() {
		return "Status: checking…"
	}
	return "Status: " + m.rulesStatus.summary
}
