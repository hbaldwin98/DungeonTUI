package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

type commandContext string

const (
	commandBrowser        commandContext = "browser"
	commandPicker         commandContext = "picker"
	commandPreview        commandContext = "preview"
	commandPlayback       commandContext = "playback"
	commandReconciliation commandContext = "reconciliation"
)

type paletteCommand struct {
	ID      string
	Label   string
	Aliases string
	Enabled bool
	Reason  string
}

type commandPalette struct {
	Open       bool
	Query      string
	Selected   int
	OriginHelp bool
	OriginPane prefs.Pane
}

func (m Model) commandContext() commandContext {
	switch {
	case m.preview != nil:
		return commandPreview
	case m.playingBack:
		return commandPlayback
	case m.reconciling && !m.reconEditing:
		return commandReconciliation
	case m.picking && !m.namingPicker:
		return commandPicker
	default:
		return commandBrowser
	}
}

func (m Model) canOpenCommandPalette() bool {
	return !m.settingsOpen && !m.searching && !m.editing && !m.planning && !m.reconEditing &&
		!m.importing && !m.namingPicker && !m.namingFolder && !m.namingCollection && m.session == nil
}

func (m Model) openCommandPalette() (tea.Model, tea.Cmd) {
	m.commands = commandPalette{Open: true, OriginHelp: m.helping, OriginPane: m.layout.Focus}
	m.helping = false
	return m, nil
}

func (m *Model) closeCommandPalette(restoreHelp bool) {
	origin := m.commands
	m.commands = commandPalette{}
	m.layout.Focus = origin.OriginPane
	if restoreHelp {
		m.helping = origin.OriginHelp
	}
}

func (m Model) paletteCommands() []paletteCommand {
	help := paletteCommand{ID: "help.open", Label: "Show keyboard reference", Aliases: "shortcuts keys"}
	capture := paletteCommand{ID: "capture.new", Label: "Capture a quick note", Aliases: "inbox thought idea jot", Enabled: m.canQuickCapture(), Reason: "Open a world or campaign first"}
	switch m.commandContext() {
	case commandPicker:
		return []paletteCommand{
			help,
			capture,
			{ID: "picker.open", Label: "Open selected world or campaign", Aliases: "enter choose"},
			{ID: "picker.new", Label: "Create world or campaign", Aliases: "new"},
			{ID: "picker.rename", Label: "Rename selected world or campaign", Aliases: "edit"},
			{ID: "picker.delete", Label: "Delete selected world or campaign", Aliases: "remove"},
			{ID: "picker.back", Label: "Back to worlds", Enabled: m.pickerLevel == "campaign", Reason: "Already viewing worlds"},
			{ID: "import.open", Label: "Import content", Aliases: "5e markdown"},
		}
	case commandPreview:
		return []paletteCommand{
			help,
			capture,
			{ID: "preview.open", Label: "Follow previewed link", Aliases: "open enter"},
			{ID: "preview.close", Label: "Close preview", Aliases: "back escape"},
			{ID: "preview.top", Label: "Scroll preview to top", Aliases: "first home"},
			{ID: "preview.bottom", Label: "Scroll preview to bottom", Aliases: "last end"},
		}
	case commandPlayback:
		return []paletteCommand{
			help,
			capture,
			{ID: "playback.previous", Label: "Previous playback beat", Aliases: "rewind"},
			{ID: "playback.next", Label: "Next playback beat", Aliases: "forward"},
			{ID: "playback.first", Label: "First playback beat", Aliases: "home"},
			{ID: "playback.last", Label: "Last playback beat", Aliases: "end"},
			{ID: "playback.close", Label: "Close playback", Aliases: "back escape"},
		}
	case commandReconciliation:
		hasItem := len(m.unresolvedReconciliationItems()) > 0
		reason := "Inbox complete"
		return []paletteCommand{
			help,
			capture,
			{ID: "recon.accept", Label: "Accept selected change", Aliases: "approve apply", Enabled: hasItem, Reason: reason},
			{ID: "recon.edit", Label: "Edit selected change", Enabled: hasItem, Reason: reason},
			{ID: "recon.defer", Label: "Defer selected change", Aliases: "later", Enabled: hasItem, Reason: reason},
			{ID: "recon.reject", Label: "Reject selected change", Aliases: "dismiss", Enabled: hasItem, Reason: reason},
			{ID: "recon.next", Label: "Next unresolved change", Enabled: hasItem, Reason: reason},
			{ID: "recon.previous", Label: "Previous unresolved change", Enabled: hasItem, Reason: reason},
			{ID: "recon.close", Label: "Close post-session inbox", Aliases: "back escape"},
		}
	default:
		record := m.selectedRecord()
		hasRecord := record != nil
		hasPlan := m.selectedPlanID != ""
		hasRecon := len(m.workspace.Reconciliations) > 0
		unfiled := len(domain.UnfiledCaptures(m.workspace, m.workspace.Scope))
		selectedCapture := record != nil && domain.IsUnfiledCapture(*record)
		return []paletteCommand{
			help,
			capture,
			{ID: "capture.file", Label: fmt.Sprintf("File selected capture · %d unfiled", unfiled), Aliases: "inbox process classify", Enabled: selectedCapture, Reason: "Select an unfiled capture note"},
			{ID: "browser.open", Label: "Open selected item", Aliases: "enter inspect"},
			{ID: "search.open", Label: "Search campaign and library", Aliases: "find"},
			{ID: "entity.new", Label: "Create entity", Aliases: "new wiki npc location"},
			{ID: "entity.edit", Label: "Edit selected entity", Enabled: hasRecord, Reason: "Select a wiki entity"},
			{ID: "prep.new", Label: "Create prep notes", Aliases: "plan"},
			{ID: "prep.edit", Label: "Edit selected prep notes", Enabled: hasPlan, Reason: "Select prep notes"},
			{ID: "session.start", Label: "Start live session", Aliases: "run play"},
			{ID: "reconciliation.open", Label: "Open post-session inbox", Aliases: "review changes", Enabled: hasRecon, Reason: "No reconciliations yet"},
			{ID: "library.open", Label: "Switch world or campaign", Aliases: "vault"},
			{ID: "import.open", Label: "Import content", Aliases: "5e markdown"},
			{ID: "settings.open", Label: "Open settings", Aliases: "5e path preferences"},
			{ID: "record.delete", Label: "Delete selected item", Aliases: "remove", Enabled: hasRecord || m.selectedSession() != nil, Reason: "Select an entity or session"},
			{ID: "record.supersede", Label: "Supersede selected entity", Aliases: "archive", Enabled: hasRecord, Reason: "Select a wiki entity"},
			{ID: "browser.back", Label: "Go back", Aliases: "history", Enabled: len(m.browserHistory) > 0, Reason: "No earlier location"},
		}
	}
}

func commandEnabled(command paletteCommand) bool {
	return command.Enabled || command.Reason == ""
}

func (m Model) filteredPaletteCommands() []paletteCommand {
	commands := m.paletteCommands()
	query := normalizeCommandText(m.commands.Query)
	if query == "" {
		return commands
	}
	type scored struct {
		command paletteCommand
		score   int
		order   int
	}
	var matches []scored
	for index, command := range commands {
		score := commandMatchScore(query, normalizeCommandText(command.Label+" "+command.ID+" "+command.Aliases))
		if score >= 0 {
			matches = append(matches, scored{command, score, index})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if commandEnabled(matches[i].command) != commandEnabled(matches[j].command) {
			return commandEnabled(matches[i].command)
		}
		return matches[i].order < matches[j].order
	})
	result := make([]paletteCommand, len(matches))
	for i := range matches {
		result[i] = matches[i].command
	}
	return result
}

func normalizeCommandText(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}

func commandMatchScore(query, target string) int {
	score := 0
	for _, token := range strings.Fields(query) {
		if index := strings.Index(target, token); index >= 0 {
			score += 100 - len(token) - index
			continue
		}
		position := 0
		for _, char := range token {
			found := strings.IndexRune(target[position:], char)
			if found < 0 {
				return -1
			}
			position += found + 1
			score++
		}
	}
	return score
}

func (m *Model) moveCommandPalette(delta int) {
	results := m.filteredPaletteCommands()
	m.commands.Selected = clamp(m.commands.Selected+delta, 0, max(0, len(results)-1))
}

func (m Model) updateCommandPalette(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeCommandPalette(true)
		return m, nil
	case "up", "ctrl+k":
		m.moveCommandPalette(-1)
		return m, nil
	case "down", "ctrl+j":
		m.moveCommandPalette(1)
		return m, nil
	case "enter":
		return m.executePaletteSelection()
	case "backspace":
		runes := []rune(m.commands.Query)
		if len(runes) > 0 {
			m.commands.Query = string(runes[:len(runes)-1])
		}
		m.commands.Selected = 0
		return m, nil
	}
	if msg.Text != "" && !msg.Mod.Contains(tea.ModCtrl) && !msg.Mod.Contains(tea.ModAlt) {
		m.commands.Query += msg.Text
		m.commands.Selected = 0
	}
	return m, nil
}

func (m Model) executePaletteSelection() (tea.Model, tea.Cmd) {
	results := m.filteredPaletteCommands()
	if len(results) == 0 {
		return m, nil
	}
	command := results[clamp(m.commands.Selected, 0, len(results)-1)]
	if !commandEnabled(command) {
		m.status = command.Reason
		return m, nil
	}
	m.closeCommandPalette(false)
	if execute := paletteCommandExecutors()[command.ID]; execute != nil {
		return execute(m)
	}
	return m, nil
}

type paletteCommandExecutor func(Model) (tea.Model, tea.Cmd)

func paletteCommandExecutors() map[string]paletteCommandExecutor {
	return map[string]paletteCommandExecutor{
		"help.open":    func(m Model) (tea.Model, tea.Cmd) { m.helping = true; return m, nil },
		"capture.new":  func(m Model) (tea.Model, tea.Cmd) { return m.openQuickCapture() },
		"capture.file": func(m Model) (tea.Model, tea.Cmd) { return m.fileSelectedCapture() },
		"browser.open": func(m Model) (tea.Model, tea.Cmd) { return m.activateBrowserSelection() },
		"search.open":  func(m Model) (tea.Model, tea.Cmd) { return m.openSearch() },
		"entity.new":   func(m Model) (tea.Model, tea.Cmd) { return m.openEditor(true) },
		"entity.edit":  func(m Model) (tea.Model, tea.Cmd) { return m.openEditor(false) },
		"prep.new":     func(m Model) (tea.Model, tea.Cmd) { return m.openPlannedNotes(true) },
		"prep.edit": func(m Model) (tea.Model, tea.Cmd) {
			m.planID = m.selectedPlanID
			return m.openPlannedNotes(false)
		},
		"session.start":       func(m Model) (tea.Model, tea.Cmd) { return m.startSession() },
		"reconciliation.open": func(m Model) (tea.Model, tea.Cmd) { return m.openReconciliation() },
		"library.open":        func(m Model) (tea.Model, tea.Cmd) { m.openPicker(); return m, nil },
		"import.open":         func(m Model) (tea.Model, tea.Cmd) { return m.openImport() },
		"settings.open":       func(m Model) (tea.Model, tea.Cmd) { return m.openSettings() },
		"record.delete":       func(m Model) (tea.Model, tea.Cmd) { return m.armDestructiveConfirm("delete") },
		"record.supersede":    func(m Model) (tea.Model, tea.Cmd) { return m.armDestructiveConfirm("supersede") },
		"browser.back":        func(m Model) (tea.Model, tea.Cmd) { m.restoreBrowserLocation(); return m, nil },
		"picker.open":         func(m Model) (tea.Model, tea.Cmd) { return m.activatePickerItem() },
		"picker.new":          func(m Model) (tea.Model, tea.Cmd) { return m.createPickerItem() },
		"picker.rename":       func(m Model) (tea.Model, tea.Cmd) { return m.openPickerRename() },
		"picker.delete":       func(m Model) (tea.Model, tea.Cmd) { return m.armPickerDelete() },
		"picker.back": func(m Model) (tea.Model, tea.Cmd) {
			m.pickerLevel = "world"
			m.pickerWorldID = ""
			m.pickerCursor = 0
			return m, nil
		},
		"preview.open":      func(m Model) (tea.Model, tea.Cmd) { return m.commitPreview() },
		"preview.close":     func(m Model) (tea.Model, tea.Cmd) { m.closePreview(); return m, nil },
		"preview.top":       func(m Model) (tea.Model, tea.Cmd) { m.preview.Scroll = 0; return m, nil },
		"preview.bottom":    func(m Model) (tea.Model, tea.Cmd) { m.preview.Scroll = m.previewMaxScroll(); return m, nil },
		"playback.previous": func(m Model) (tea.Model, tea.Cmd) { return m.updatePlayback(tea.KeyPressMsg{Code: tea.KeyLeft}) },
		"playback.next":     func(m Model) (tea.Model, tea.Cmd) { return m.updatePlayback(tea.KeyPressMsg{Code: tea.KeyRight}) },
		"playback.first":    func(m Model) (tea.Model, tea.Cmd) { m.playbackCursor = 0; return m, nil },
		"playback.last":     func(m Model) (tea.Model, tea.Cmd) { return m.updatePlayback(tea.KeyPressMsg{Code: tea.KeyEnd}) },
		"playback.close":    func(m Model) (tea.Model, tea.Cmd) { m.playingBack = false; return m, nil },
		"recon.accept": func(m Model) (tea.Model, tea.Cmd) {
			return m.approveReconciliationItem(&m.workspace.Reconciliations[m.reconIndex])
		},
		"recon.edit": func(m Model) (tea.Model, tea.Cmd) {
			return m.startReconEdit(&m.workspace.Reconciliations[m.reconIndex])
		},
		"recon.defer": func(m Model) (tea.Model, tea.Cmd) { return m.deferReconciliationItem() },
		"recon.reject": func(m Model) (tea.Model, tea.Cmd) {
			return m.rejectReconciliationItem(&m.workspace.Reconciliations[m.reconIndex])
		},
		"recon.next":     func(m Model) (tea.Model, tea.Cmd) { m.moveReconciliationCursor(1); return m, nil },
		"recon.previous": func(m Model) (tea.Model, tea.Cmd) { m.moveReconciliationCursor(-1); return m, nil },
		"recon.close":    func(m Model) (tea.Model, tea.Cmd) { m.reconciling = false; return m, nil },
	}
}

func (m Model) renderCommandPalette(background string) string {
	width := min(72, max(20, m.width-4))
	results := m.filteredPaletteCommands()
	visible := min(len(results), max(1, m.height-8))
	selected := clamp(m.commands.Selected, 0, max(0, len(results)-1))
	start := clamp(selected-visible/2, 0, max(0, len(results)-visible))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("COMMAND PALETTE"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("Esc returns"))
	builder.WriteString("\n\n")
	builder.WriteString(filterStyle.Render(": " + m.commands.Query))
	builder.WriteString("\n\n")
	if len(results) == 0 {
		builder.WriteString(mutedStyle.Render("No matching commands"))
	}
	for index := start; index < start+visible; index++ {
		command := results[index]
		label := command.Label
		if !commandEnabled(command) {
			label += " · " + command.Reason
		}
		style := searchResultStyle
		if index == selected {
			style = selectedSearchResultStyle
		}
		if !commandEnabled(command) {
			style = mutedStyle
		}
		builder.WriteString(style.Render("  " + label))
		builder.WriteString("\n")
	}
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	x := max(0, (m.width-lipgloss.Width(overlay))/2)
	y := max(0, (m.height-lipgloss.Height(overlay))/3)
	return lipgloss.NewCompositor(lipgloss.NewLayer(background).X(0).Y(0).Z(0), lipgloss.NewLayer(overlay).X(x).Y(y).Z(1)).Render()
}

func (m Model) updateCommandPaletteClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	left, top, width, height, rowY, start, visible := m.commandPaletteGeometry()
	if msg.X < left || msg.X >= left+width || msg.Y < top || msg.Y >= top+height {
		m.closeCommandPalette(true)
		return m, nil
	}
	row := msg.Y - rowY
	if row < 0 || row >= visible {
		return m, nil
	}
	m.commands.Selected = start + row
	return m.executePaletteSelection()
}

func (m Model) commandPaletteGeometry() (left, top, width, height, rowY, start, visible int) {
	results := m.filteredPaletteCommands()
	visible = min(len(results), max(1, m.height-8))
	selected := clamp(m.commands.Selected, 0, max(0, len(results)-1))
	start = clamp(selected-visible/2, 0, max(0, len(results)-visible))
	panelWidth := min(72, max(20, m.width-4))
	panelHeight := 6 + visible
	width = panelWidth + searchPanelStyle.GetHorizontalFrameSize()
	height = panelHeight + searchPanelStyle.GetVerticalFrameSize()
	left = max(0, (m.width-width)/2)
	top = max(0, (m.height-height)/3)
	rowY = top + 6
	return
}
