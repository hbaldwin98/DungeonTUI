package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

// applyLayoutPreset switches the preset's tree, keeps focus on a visible
// pane, persists locally, and says how to undo it.
func (m Model) applyLayoutPreset(preset prefs.Preset) (tea.Model, tea.Cmd) {
	if err := m.layout.ApplyPreset(preset, m.width); err != nil {
		m.status = err.Error()
		return m, nil
	}
	if m.session == nil && !prefs.FindVisibleLeaf(m.layout.Browser.Root, m.layout.Focus) {
		m.layout.Focus = prefs.PaneDetail
	}
	if m.session != nil {
		m.configureTranscriptViewport()
	}
	m.persistPreferences()
	m.status = preset.Label() + " layout · " + preset.Describe() + " · : restore my layout"
	return m, nil
}

func (m Model) restoreLayout() (tea.Model, tea.Cmd) {
	if !m.layout.RestoreLayout() {
		m.status = "Already on your own layout"
		return m, nil
	}
	if m.session != nil {
		m.configureTranscriptViewport()
	}
	m.persistPreferences()
	m.status = "Restored your layout"
	return m, nil
}

// presetCommands lists the preset switches for the palette in any context.
func (m Model) presetCommands() []paletteCommand {
	commands := make([]paletteCommand, 0, len(prefs.Presets)+1)
	for _, preset := range prefs.Presets {
		label := "Layout: " + preset.Label() + " · " + preset.Describe()
		if m.layout.Preset == string(preset) {
			label += " (current)"
		}
		commands = append(commands, paletteCommand{
			ID:      "layout." + string(preset),
			Label:   label,
			Aliases: "preset view panes " + string(preset),
		})
	}
	commands = append(commands, paletteCommand{
		ID: "layout.restore", Label: "Layout: restore my layout", Aliases: "preset undo reset panes",
		Enabled: m.layout.Previous != nil, Reason: "No preset applied",
	})
	return commands
}

func presetExecutors() map[string]paletteCommandExecutor {
	out := map[string]paletteCommandExecutor{
		"layout.restore": func(m Model) (tea.Model, tea.Cmd) { return m.restoreLayout() },
	}
	for _, preset := range prefs.Presets {
		preset := preset
		out["layout."+string(preset)] = func(m Model) (tea.Model, tea.Cmd) { return m.applyLayoutPreset(preset) }
	}
	return out
}
