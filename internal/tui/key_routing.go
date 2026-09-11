package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

// This file holds Model.update's input routing: which surface owns a key or
// a click, and the browser's own key table.

func (m Model) resize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	if m.session != nil {
		m.configureTranscriptViewport()
	}
	if m.editing {
		sizeMarkdownTextArea(&m.editBody, m.width, m.height)
	}
	if m.planning {
		sizeMarkdownTextArea(&m.planBody, m.width, m.height-4)
		m.planTitle.SetWidth(max(20, m.width-10))
	}
	if m.capture.Open {
		m.capture.Input.SetWidth(captureInputWidth(m.width))
	}
	return m, nil
}

// keyHandler is one surface's own key handling.
type keyHandler func(Model, tea.KeyPressMsg) (tea.Model, tea.Cmd)

// helpKey reports whether a key opens help over a surface.
type helpKey func(Model, string) bool

func questionOpensHelp(_ Model, key string) bool { return key == "?" }

func noHelpKey(Model, string) bool { return false }

// updateKey routes a key press. Quick capture and the command palette sit
// above every surface; otherwise the surface that owns the keyboard takes
// it, and the browser handles the keys no surface claims.
func (m Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch {
	case m.capture.Open:
		return m.updateQuickCapture(msg)
	case key == "ctrl+n":
		return m.openQuickCapture()
	case m.commands.Open:
		return m.updateCommandPalette(msg)
	case key == ":" && m.canOpenCommandPalette():
		return m.openCommandPalette()
	}
	handler, opensHelp := m.keySurface()
	if opensHelp(m, key) {
		return m.openHelp()
	}
	if handler == nil {
		return m.updateBrowserKey(key)
	}
	return handler(m, msg)
}

// keySurface names the surface that owns the keyboard, topmost first, and
// how help opens over it. A nil handler means the browser owns it.
func (m Model) keySurface() (keyHandler, helpKey) {
	switch {
	case m.helping:
		return Model.updateHelp, noHelpKey
	case m.settingsOpen:
		return Model.updateSettings, noHelpKey
	case m.preview != nil:
		return Model.updatePreview, questionOpensHelp
	case m.picking:
		return Model.updatePicker, noHelpKey
	case m.importing:
		return Model.updateImport, questionOpensHelp
	case m.editing:
		return Model.updateEditor, Model.editorHelpKey
	case m.planning:
		return Model.updatePlannedNotes, Model.editorHelpKey
	case m.playingBack:
		return Model.updatePlayback, questionOpensHelp
	case m.reconciling:
		return Model.updateReconciliation, noHelpKey
	case m.searching:
		return Model.updateSearch, questionOpensHelp
	case m.session != nil:
		return Model.updateSession, questionOpensHelp
	case m.namingFolder:
		return Model.updateFolderName, noHelpKey
	case m.namingCollection:
		return Model.updateCollectionName, noHelpKey
	}
	return nil, questionOpensHelp
}

// browserKeyAction is what one browser key does.
type browserKeyAction func(Model) (tea.Model, tea.Cmd)

// browserKeys is filled in init because its actions reach back into
// Model.update, which a package-level initializer may not depend on.
var browserKeys map[string]browserKeyAction

func init() {
	browserKeys = map[string]browserKeyAction{
		"q": Model.quit, "ctrl+c": Model.quit,
		"j": Model.browserDown, "down": Model.browserDown,
		"k": Model.browserUp, "up": Model.browserUp,
		"pgdown": Model.detailPageDown, "pgup": Model.detailPageUp,
		"home": Model.detailTop, "end": Model.detailBottom,
		"/": Model.openSearch, ",": Model.openSettings,
		"n": Model.newEntityOrCancel, "e": Model.editSelection,
		"t": Model.focusNextBrowserPane, "right": Model.focusNextBrowserPane, "tab": Model.focusNextBrowserPane,
		"left": Model.focusPrevBrowserPane, "shift+tab": Model.focusPrevBrowserPane,
		"ctrl+p": Model.cycleTypePanes, "+": Model.addTypePane, "-": Model.closeFocusedPane,
		"b": Model.openLibrary, "p": Model.newPlannedNotes, "s": Model.startSessionKey,
		"enter": Model.activateBrowserSelection, "r": Model.openReconciliation,
		"f": Model.nextTagFilter, "c": Model.nextCollectionFilter, "g": Model.openCollectionName,
		"I": Model.openImport, "i": Model.openImport, "m": Model.openFolderName,
		"a": Model.toggleCollectionMembership, "o": Model.nextListScope,
		"d": Model.armDelete, "x": Model.armSupersede, "y": Model.confirmDestructiveAction,
		"esc":       Model.cancelDestructiveConfirm,
		"backspace": Model.goBack, "alt+left": Model.goBack, "alt+right": Model.goForward,
	}
}

func (m Model) updateBrowserKey(key string) (tea.Model, tea.Cmd) {
	if action, ok := browserKeys[key]; ok {
		return action(m)
	}
	return m, nil
}

func (m Model) quit() (tea.Model, tea.Cmd) { return m, tea.Quit }

// moveFocused moves whatever the focused pane selects: the campaign tree, a
// prep beat, a detail link, or the list row.
func (m Model) moveFocused(delta int) (tea.Model, tea.Cmd) {
	switch {
	case m.layout.Focus == prefs.PaneNav:
		m.moveNavCursor(delta)
	case m.layout.Focus == prefs.PaneDetail && m.currentNav().Kind == NavPrep:
		m.movePrepBeat(delta)
	case m.layout.Focus == prefs.PaneDetail:
		m.moveDetailFocus(delta)
	default:
		m.moveBrowserCursor(delta)
	}
	return m, nil
}

func (m Model) browserDown() (tea.Model, tea.Cmd) { return m.moveFocused(1) }

func (m Model) browserUp() (tea.Model, tea.Cmd) { return m.moveFocused(-1) }

// scrollFocusedDetail scrolls the detail pane only while it has focus.
func (m Model) scrollFocusedDetail(scroll func(*Model)) (tea.Model, tea.Cmd) {
	if m.layout.Focus == prefs.PaneDetail {
		scroll(&m)
	}
	return m, nil
}

func (m Model) detailPageDown() (tea.Model, tea.Cmd) {
	return m.scrollFocusedDetail(func(m *Model) { m.scrollDetail(m.detailPageStep()) })
}

func (m Model) detailPageUp() (tea.Model, tea.Cmd) {
	return m.scrollFocusedDetail(func(m *Model) { m.scrollDetail(-m.detailPageStep()) })
}

func (m Model) detailTop() (tea.Model, tea.Cmd) {
	return m.scrollFocusedDetail(func(m *Model) { m.scrollDetailTo(0) })
}

func (m Model) detailBottom() (tea.Model, tea.Cmd) {
	return m.scrollFocusedDetail(func(m *Model) { m.scrollDetailTo(m.detailMaxScroll()) })
}

// newEntityOrCancel is n: it answers "no" to an armed delete or supersede,
// and otherwise opens a new entity.
func (m Model) newEntityOrCancel() (tea.Model, tea.Cmd) {
	if m.deleteConfirm {
		m.clearDestructiveConfirm("Cancelled")
		return m, nil
	}
	return m.openEditor(true)
}

func (m Model) editSelection() (tea.Model, tea.Cmd) {
	if m.usesCampaignTree() && m.currentNav().Kind == NavPrep && m.selectedPlanID != "" {
		m.planID = m.selectedPlanID
		return m.openPlannedNotes(false)
	}
	return m.openEditor(false)
}

func (m Model) focusNextBrowserPane() (tea.Model, tea.Cmd) {
	m.cycleBrowserFocus()
	return m, nil
}

func (m Model) focusPrevBrowserPane() (tea.Model, tea.Cmd) {
	m.cycleBrowserFocusBack()
	return m, nil
}

func (m Model) cycleTypePanes() (tea.Model, tea.Cmd) {
	if m.usesCampaignTree() {
		m.status = "Use the campaign tree to switch sections"
		return m, nil
	}
	m.layout.CycleBrowserTypeVisibility()
	m.persistPreferences()
	m.status = "Cycled browser type panes"
	return m, nil
}

func (m Model) addTypePane() (tea.Model, tea.Cmd) {
	if m.usesCampaignTree() {
		m.status = "Sections live in the campaign tree · Tab focuses panes"
		return m, nil
	}
	m.addNextBrowserTypePane()
	return m, nil
}

func (m Model) openLibrary() (tea.Model, tea.Cmd) {
	if m.session == nil && !m.deleteConfirm {
		m.openPicker()
	}
	return m, nil
}

func (m Model) newPlannedNotes() (tea.Model, tea.Cmd) { return m.openPlannedNotes(true) }

func (m Model) startSessionKey() (tea.Model, tea.Cmd) {
	if m.deleteConfirm {
		return m, nil
	}
	return m.startSession()
}

func (m Model) nextTagFilter() (tea.Model, tea.Cmd) {
	m.cycleTagFilter()
	return m, nil
}

func (m Model) nextCollectionFilter() (tea.Model, tea.Cmd) {
	m.cycleCollectionFilter()
	return m, nil
}

func (m Model) nextListScope() (tea.Model, tea.Cmd) {
	m.cycleListScope()
	return m, nil
}

func (m Model) armDelete() (tea.Model, tea.Cmd) { return m.armDestructiveConfirm("delete") }

func (m Model) armSupersede() (tea.Model, tea.Cmd) { return m.armDestructiveConfirm("supersede") }

func (m Model) cancelDestructiveConfirm() (tea.Model, tea.Cmd) {
	if m.deleteConfirm {
		m.clearDestructiveConfirm("Cancelled")
	}
	return m, nil
}

func (m Model) goBack() (tea.Model, tea.Cmd) {
	cmd := m.restoreBrowserLocation()
	return m, cmd
}

func (m Model) goForward() (tea.Model, tea.Cmd) {
	cmd := m.advanceBrowserLocation()
	return m, cmd
}

func (m Model) routeMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.capture.Open:
		return m, nil
	case m.commands.Open:
		return m.updateCommandPaletteClick(msg)
	case m.reconciling:
		return m.updateReconciliationMouse(msg)
	case m.session != nil && m.preview == nil && !m.searching:
		return m.updateSessionMouseClick(msg)
	}
	return m.updateMouseClick(msg)
}

// wheelDelta is 1 for wheel down, -1 for wheel up, and 0 for sideways.
func wheelDelta(msg tea.MouseWheelMsg) int {
	switch msg.Button {
	case tea.MouseWheelDown:
		return 1
	case tea.MouseWheelUp:
		return -1
	}
	return 0
}

func (m Model) routeMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.commands.Open:
		if delta := wheelDelta(msg); delta != 0 {
			m.moveCommandPalette(delta)
		}
		return m, nil
	case m.reconciling:
		if delta := wheelDelta(msg); delta != 0 {
			m.moveReconciliationCursor(delta)
		}
		return m, nil
	case m.session != nil && m.preview == nil && !m.searching:
		m.setSessionFocus(prefs.PaneTranscript)
		var cmd tea.Cmd
		m.transcriptView, cmd = m.transcriptView.Update(msg)
		return m, cmd
	}
	return m.updateMouseWheel(msg)
}

// dragSplit follows a gutter drag: the live session's upper/lower split,
// the outer vertical split, or the campaign tree's inner list/detail split.
func (m Model) dragSplit(msg tea.MouseMotionMsg) Model {
	if !m.draggingSplit {
		return m
	}
	switch {
	case m.dragAxis == "horizontal":
		available := max(8, m.height-2)
		work := max(8, available-m.sessionInputHeight())
		ratio := float64(clamp(msg.Y-1, 6, work-5)) / float64(work)
		m.layout.SetSessionUpperRatio(ratio)
		m.configureTranscriptViewport()
	case m.session != nil || m.draggingNavSplit || !m.usesCampaignTree():
		ratio := float64(clamp(msg.X, 24, max(25, m.width-24))) / float64(max(1, m.width))
		m.layout.SetVerticalSplitRatio(ratio)
	case m.layout.Browser.Root.Type == "split" && len(m.layout.Browser.Root.Children) > 1:
		m.dragBrowserInnerSplit(msg.X)
	}
	return m
}

func (m *Model) dragBrowserInnerSplit(x int) {
	navW := clamp(int(float64(m.width)*m.layout.Browser.Root.Ratio), 10, max(10, m.width-20))
	remain := max(1, m.width-navW)
	inner := float64(clamp(x-navW, 12, max(13, remain-12))) / float64(remain)
	child := &m.layout.Browser.Root.Children[1]
	if child.Type == "split" && child.Axis == prefs.AxisVertical {
		child.Ratio = clampBrowserRatio(inner)
	}
}

func (m Model) endDrag() Model {
	wasDragging := m.draggingSplit
	m.draggingSplit = false
	m.draggingNavSplit = false
	m.dragAxis = ""
	if wasDragging {
		m.persistPreferences()
	}
	return m
}
