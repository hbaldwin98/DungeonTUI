package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/app"
	"github.com/hbaldwin98/DungeonTUI/internal/dice"
	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/ingest/fivetools"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

type Model struct {
	workspace          domain.Workspace
	workspaceRevision  uint64
	app                app.Service
	search             searchsvc.Service
	cursor             int
	selectedID         string
	width              int
	height             int
	searching          bool
	searchInput        textinput.Model
	searchScope        searchsvc.Scope
	includeIdeas       bool
	results            []searchsvc.Result
	selected           int
	typeFilter         domain.EntityType
	store              storage.Store
	prefs              prefs.Store
	status             string
	statusSeq          uint64 // advances each time status changes; see trackStatus
	editing            bool
	creating           bool
	editID             string
	editType           domain.EntityType
	editBody           textarea.Model
	session            *domain.SessionRecord
	sessionInput       textarea.Model
	review             *domain.Record
	reviewPinned       bool
	suggestions        []Suggestion
	suggestion         int
	campaignCursor     int
	prepScroll         int
	prepBeat           map[string]int
	prepBeatState      map[string]map[string]beatState
	contextCursor      int
	previousInput      string
	transcriptView     viewport.Model
	layout             prefs.Layout
	draggingSplit      bool
	dragAxis           string
	rollRNG            dice.RNG
	reconciling        bool
	reconIndex         int
	reconCursor        int
	reconEditing       bool
	reconEdit          textarea.Model
	deleteConfirm      bool
	confirmKind        string // "delete" or "supersede"
	planning           bool
	planID             string
	planDraft          domain.PlannedNotes
	planTitle          textinput.Model
	planBody           textarea.Model
	planField          int
	picking            bool
	pickerLevel        string // "world" or "campaign"
	pickerCursor       int
	pickerWorldID      string
	namingPicker       bool
	navCursor          int
	navKind            NavKind
	navType            domain.EntityType
	selectedSessionID  string
	selectedPlanID     string
	draggingNavSplit   bool
	tagFilter          string
	listScope          searchsvc.Scope
	helping            bool
	peek               *domain.Record
	peekScroll         int
	historyCursor      int
	detailView         *paneScroll
	browserHistory     []browserLocation
	browserForward     []browserLocation
	playingBack        bool
	playbackCursor     int
	collectionFilter   string
	lastCollectionID   string
	namingCollection   bool
	collectionName     textinput.Model
	collapsedFolders   map[string]bool
	expandedFolders    map[string]bool
	selectedFolderPath string
	namingFolder       bool
	preview            *previewBuf
	rulesNote          string
	rulesSearchSeq     uint64
	rulesQuerySeq      uint64
	rulesStatus        rulesStatus
	settingsOpen       bool
	settingsFromSearch bool
	settingsInput      textinput.Model
	importing          bool
	importFromPicker   bool
	importFocus        string // "sources", "tools", or "files"
	importKind         domain.SourceKind
	importDir          string
	importFiles        []importFile
	importFileCursor   int
	importSourceCursor int
	importTools        []fivetools.Entry
	importToolsCursor  int
	importToolsQuery   string
	importBusy         bool
	ingestJob          *ingestJob
	toolsFetcher       fivetools.Fetcher
	adventures         []fivetools.AdventureBook
	selectedSourceID   string
	selectedHitName    string
	selectedChapter    string
	searchPath         string
	commands           commandPalette
	capture            captureOverlay
}

func New() Model {
	model := newModel(demoWorkspace(), nil, nil)
	model.enterDemoCampaign()
	return model
}

// NewPersistent loads the owner's workspace from the platform config
// directory. A first run opens an empty library picker so the owner creates a
// world instead of inheriting demo campaign data. Layout preferences load from
// a sibling file. Launch opens an Obsidian-style world/campaign picker unless
// a prior scope can be restored from preferences.
func NewPersistent() Model {
	store, err := storage.OpenDefault()
	if err != nil {
		if store == nil || store.Path == "" {
			return newPickerModel(demoWorkspace(), nil, nil, "")
		}
		prefStore := prefs.NewJSON(prefs.BesideWorkspace(store.Path))
		return newLoadFailureModel(store, prefStore, err)
	}
	prefStore := prefs.NewJSON(prefs.BesideWorkspace(store.Path))
	workspace, err := store.Load()
	if err != nil {
		return newLoadFailureModel(store, prefStore, err)
	}
	return newLoadedModel(workspace, store, prefStore)
}

func newLoadFailureModel(store storage.Store, prefStore prefs.Store, err error) Model {
	if !errors.Is(err, os.ErrNotExist) {
		return newPickerModel(domain.Workspace{}, nil, prefStore, "Workspace could not be loaded; writes disabled: "+err.Error())
	}
	return newPickerModel(domain.Workspace{}, store, prefStore, "New library · n creates a world")
}

func newPickerModel(workspace domain.Workspace, store storage.Store, prefStore prefs.Store, status string) Model {
	model := newModel(workspace, store, prefStore)
	model.applyPreferences()
	model.openPicker()
	model.status = status
	return model
}

func newLoadedModel(workspace domain.Workspace, store storage.Store, prefStore prefs.Store) Model {
	model := newModel(workspace, store, prefStore)
	model.applyPreferences()
	model.workspace.EnsureLibrary()
	if scope, ok := model.workspace.ScopeFor(model.layout.ActiveWorldID, model.layout.ActiveCampaignID); ok {
		model.enterScope(scope)
		if model.session == nil {
			model.status = "Restored " + scope.Campaign + " · b back to library"
		}
		return model
	}
	model.openPicker()
	return model
}

func newModel(workspace domain.Workspace, store storage.Store, prefStore prefs.Store) Model {
	input := textinput.New()
	input.Placeholder = "Search wiki, prep, sessions, and sources"
	input.Prompt = "> "
	input.CharLimit = 120

	name := textinput.New()
	name.Placeholder = "Collection name"
	name.Prompt = "Name: "
	name.CharLimit = 80

	settings := textinput.New()
	settings.Placeholder = "/usr/local/bin/5e"
	settings.Prompt = "5e path: "
	settings.CharLimit = 400

	model := Model{
		workspace:        workspace,
		app:              app.New(store),
		store:            store,
		prefs:            prefStore,
		searchPath:       indexPathFromStore(store),
		searchInput:      input,
		searchScope:      searchsvc.CurrentCampaign,
		includeIdeas:     false,
		listScope:        searchsvc.CurrentCampaign,
		collectionName:   name,
		settingsInput:    settings,
		collapsedFolders: map[string]bool{},
		expandedFolders:  map[string]bool{},
		detailView:       &paneScroll{},
	}
	model.sessionInput = textarea.New()
	model.sessionInput.Prompt = "│ "
	model.sessionInput.Placeholder = "Start a session to capture play…"
	model.sessionInput.SetHeight(5)
	// The bubbles default paints the cursor line black, which fights the
	// terminal's own background (decision #27); reuse the editor's styles.
	model.sessionInput.SetStyles(dungeonTextAreaStyles())
	model.transcriptView = viewport.New()
	model.transcriptView.SoftWrap = true
	model.transcriptView.MouseWheelEnabled = true
	model.layout = prefs.DefaultLayout()
	model.rollRNG = dice.DefaultRNG()
	model.workspace.EnsureLibrary()
	model.attachReferences()
	model.rebuildSearch()
	model.refreshResults()
	model.ensureBrowserSelection()
	return model
}

func (m *Model) replaceWorkspace(workspace domain.Workspace) {
	m.workspace = workspace
	m.workspaceRevision++
}

func (m *Model) markWorkspaceChanged() {
	m.workspaceRevision++
}

func indexPathFromStore(store storage.Store) string {
	if _, ok := store.(*storage.SQLiteStore); ok {
		return ""
	}
	js, ok := store.(storage.JSONStore)
	if !ok || js.Path == "" {
		return ""
	}
	return searchsvc.IndexPath(js.Path)
}

func sqliteStore(store storage.Store) *storage.SQLiteStore {
	s, _ := store.(*storage.SQLiteStore)
	return s
}

func (m Model) Init() tea.Cmd {
	return nil
}

// Update routes every message and then lets trackStatus time any status the
// message produced, so status feedback expires without per-call-site timers.
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := message.(statusExpireMsg); ok {
		return m.expireStatus(msg)
	}
	before := m.status
	next, cmd := m.update(message)
	return trackStatus(before, next, cmd)
}

func (m Model) update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case toolsCatalogMsg:
		return m.handleToolsCatalog(msg)
	case toolsIngestMsg:
		return m.handleToolsIngest(msg)
	case rulesDebounceMsg:
		return m.handleRulesDebounce(msg)
	case rulesSearchMsg:
		return m.handleRulesSearch(msg)
	case rulesStatusMsg:
		return m.handleRulesStatus(msg)
	case ruleEntityMsg:
		return m.handleRuleEntity(msg)
	case ingestTickMsg:
		return m.handleIngestTick()
	case tea.WindowSizeMsg:
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
	case tea.KeyPressMsg:
		if m.capture.Open {
			return m.updateQuickCapture(msg)
		}
		if msg.String() == "ctrl+n" {
			return m.openQuickCapture()
		}
		if m.commands.Open {
			return m.updateCommandPalette(msg)
		}
		if msg.String() == ":" && m.canOpenCommandPalette() {
			return m.openCommandPalette()
		}
		if m.helping {
			return m.updateHelp(msg)
		}
		if m.settingsOpen {
			return m.updateSettings(msg)
		}
		if m.preview != nil {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updatePreview(msg)
		}
		if m.picking {
			return m.updatePicker(msg)
		}
		if m.importing {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updateImport(msg)
		}
		if m.editing {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updateEditor(msg)
		}
		if m.planning {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updatePlannedNotes(msg)
		}
		if m.playingBack {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updatePlayback(msg)
		}
		if m.reconciling {
			return m.updateReconciliation(msg)
		}
		if m.searching {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updateSearch(msg)
		}
		if m.session != nil {
			if msg.String() == "?" {
				return m.openHelp()
			}
			return m.updateSession(msg)
		}
		if m.namingFolder {
			return m.updateFolderName(msg)
		}
		if m.namingCollection {
			return m.updateCollectionName(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			return m.openHelp()
		case "j", "down":
			if m.layout.Focus == prefs.PaneNav {
				m.moveNavCursor(1)
			} else if m.layout.Focus == prefs.PaneDetail {
				if m.currentNav().Kind == NavPrep {
					m.movePrepBeat(1)
				} else {
					m.moveDetailFocus(1)
				}
			} else {
				m.moveBrowserCursor(1)
			}
		case "k", "up":
			if m.layout.Focus == prefs.PaneNav {
				m.moveNavCursor(-1)
			} else if m.layout.Focus == prefs.PaneDetail {
				if m.currentNav().Kind == NavPrep {
					m.movePrepBeat(-1)
				} else {
					m.moveDetailFocus(-1)
				}
			} else {
				m.moveBrowserCursor(-1)
			}
		case "pgdown":
			if m.layout.Focus == prefs.PaneDetail {
				m.scrollDetail(m.detailPageStep())
			}
		case "pgup":
			if m.layout.Focus == prefs.PaneDetail {
				m.scrollDetail(-m.detailPageStep())
			}
		case "home":
			if m.layout.Focus == prefs.PaneDetail {
				m.scrollDetailTo(0)
			}
		case "end":
			if m.layout.Focus == prefs.PaneDetail {
				m.scrollDetailTo(m.detailMaxScroll())
			}
		case "/":
			return m.openSearch()
		case ",":
			return m.openSettings()
		case "n":
			if m.deleteConfirm {
				m.clearDestructiveConfirm("Cancelled")
				return m, nil
			}
			return m.openEditor(true)
		case "e":
			if m.usesCampaignTree() && m.currentNav().Kind == NavPrep && m.selectedPlanID != "" {
				m.planID = m.selectedPlanID
				return m.openPlannedNotes(false)
			}
			return m.openEditor(false)
		case "t", "right":
			m.cycleBrowserFocus()
		case "left", "shift+tab":
			m.cycleBrowserFocusBack()
		case "ctrl+p":
			if m.usesCampaignTree() {
				m.status = "Use the campaign tree to switch sections"
				return m, nil
			}
			m.layout.CycleBrowserTypeVisibility()
			m.persistPreferences()
			m.status = "Cycled browser type panes"
		case "+":
			if m.usesCampaignTree() {
				m.status = "Sections live in the campaign tree · Tab focuses panes"
				return m, nil
			}
			m.addNextBrowserTypePane()
		case "-":
			return m.closeFocusedPane()
		case "b":
			if m.session == nil && !m.deleteConfirm {
				m.openPicker()
				return m, nil
			}
		case "p":
			return m.openPlannedNotes(true)
		case "s":
			if m.deleteConfirm {
				return m, nil
			}
			return m.startSession()
		case "enter":
			return m.activateBrowserSelection()
		case "r":
			return m.openReconciliation()
		case "f":
			m.cycleTagFilter()
		case "c":
			m.cycleCollectionFilter()
		case "g":
			return m.openCollectionName()
		case "I", "i":
			return m.openImport()
		case "m":
			return m.openFolderName()
		case "a":
			return m.toggleCollectionMembership()
		case "o":
			m.cycleListScope()
		case "d":
			return m.armDestructiveConfirm("delete")
		case "x":
			return m.armDestructiveConfirm("supersede")
		case "y":
			return m.confirmDestructiveAction()
		case "esc":
			if m.deleteConfirm {
				m.clearDestructiveConfirm("Cancelled")
			}
		case "tab":
			m.cycleBrowserFocus()
		case "backspace", "alt+left":
			return m, m.restoreBrowserLocation()
		case "alt+right":
			return m, m.advanceBrowserLocation()
		}
	case tea.MouseClickMsg:
		if m.capture.Open {
			return m, nil
		}
		if m.commands.Open {
			return m.updateCommandPaletteClick(msg)
		}
		if m.reconciling {
			return m.updateReconciliationMouse(msg)
		}
		if m.preview != nil || m.searching {
			return m.updateMouseClick(msg)
		}
		if m.session != nil {
			return m.updateSessionMouseClick(msg)
		}
		return m.updateMouseClick(msg)
	case tea.MouseWheelMsg:
		if m.commands.Open {
			if msg.Button == tea.MouseWheelDown {
				m.moveCommandPalette(1)
			} else if msg.Button == tea.MouseWheelUp {
				m.moveCommandPalette(-1)
			}
			return m, nil
		}
		if m.reconciling {
			if msg.Button == tea.MouseWheelDown {
				m.moveReconciliationCursor(1)
			} else if msg.Button == tea.MouseWheelUp {
				m.moveReconciliationCursor(-1)
			}
			return m, nil
		}
		if m.preview != nil || m.searching {
			return m.updateMouseWheel(msg)
		}
		if m.session != nil {
			m.setSessionFocus(prefs.PaneTranscript)
			var cmd tea.Cmd
			m.transcriptView, cmd = m.transcriptView.Update(msg)
			return m, cmd
		}
		return m.updateMouseWheel(msg)
	case tea.MouseMotionMsg:
		if m.draggingSplit {
			if m.dragAxis == "horizontal" {
				available := max(8, m.height-2)
				work := max(8, available-m.sessionInputHeight())
				ratio := float64(clamp(msg.Y-1, 6, work-5)) / float64(work)
				m.layout.SetSessionUpperRatio(ratio)
				m.configureTranscriptViewport()
			} else if m.session != nil || m.draggingNavSplit || !m.usesCampaignTree() {
				ratio := float64(clamp(msg.X, 24, max(25, m.width-24))) / float64(max(1, m.width))
				m.layout.SetVerticalSplitRatio(ratio)
			} else if m.usesCampaignTree() && m.layout.Browser.Root.Type == "split" && len(m.layout.Browser.Root.Children) > 1 {
				navW := clamp(int(float64(m.width)*m.layout.Browser.Root.Ratio), 10, max(10, m.width-20))
				remain := max(1, m.width-navW)
				inner := float64(clamp(msg.X-navW, 12, max(13, remain-12))) / float64(remain)
				child := &m.layout.Browser.Root.Children[1]
				if child.Type == "split" && child.Axis == prefs.AxisVertical {
					child.Ratio = clampBrowserRatio(inner)
				}
			}
		}
		return m, nil
	case tea.MouseReleaseMsg:
		wasDragging := m.draggingSplit
		m.draggingSplit = false
		m.draggingNavSplit = false
		m.dragAxis = ""
		if wasDragging {
			m.persistPreferences()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) startSession() (tea.Model, tea.Cmd) {
	started := time.Now().UTC()
	if m.review != nil && m.operationalRecordByID(m.review.ID) == nil {
		m.review = nil
	}
	session := domain.SessionRecord{
		ID:        fmt.Sprintf("session-%d", started.UnixNano()),
		Title:     "Session " + started.Format("2006-01-02 15:04"),
		Scope:     m.workspace.Scope,
		StartedAt: started,
	}
	if folder := m.folderForNewSit(); folder != "" {
		session.Folder = folder
	}
	plan := m.activePlannedNotes()
	if plan != nil {
		session.PlannedNotesID = plan.ID
		already := domain.SessionsSeededFrom(m.workspace.Sessions, plan.ID)
		session.Title = domain.NextSitTitle(plan.Title, len(already), started)
		if location := m.operationalRecordByID(plan.LocationID); location != nil && location.Type == domain.Location {
			session.LocationID = location.ID
			session.LocationName = location.Title
		} else if location := m.findLocation(plan.LocationName); location != nil {
			session.LocationID = location.ID
			session.LocationName = location.Title
		}
		planReviewSet := false
		for _, link := range plan.Links {
			if record := m.operationalRecordByID(link.RecordID); record != nil {
				session = session.Associate(link)
				if !planReviewSet {
					candidate := *record
					m.review = &candidate
					planReviewSet = true
				}
			}
		}
	}
	if session.LocationID == "" && session.LocationName == "" {
		if location := m.defaultSessionLocation(); location != nil {
			session.LocationID = location.ID
			session.LocationName = location.Title
		}
	}
	m.session = &session
	m.prepScroll = 0
	m.sessionInput.SetValue("")
	m.sessionInput.SetWidth(max(30, m.width-8))
	m.sessionInput.SetHeight(5)
	m.sessionInput.Focus()
	m.setSessionFocus(prefs.PaneInput)
	m.configureTranscriptViewport()
	m.refreshTranscriptViewport()
	m.refreshSuggestions()
	if plan != nil {
		m.status = "Live from planned notes · " + session.Title + " · Ctrl+E ends"
	} else {
		m.status = "Session started · click panes to focus · Ctrl+E ends capture"
	}
	return m, nil
}

func (m Model) updateSession(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	model := m
	focus := model.sessionFocus()

	if next, cmd, handled := model.updateSessionLeadingKey(msg, focus); handled {
		return next, cmd
	}
	if next, cmd, handled := model.updateSessionPaneKey(msg, focus); handled {
		return next, cmd
	}

	// Printable input and suggestion navigation belong to the input pane.
	if focus != prefs.PaneInput {
		if len(msg.Text) > 0 || msg.String() == "backspace" || msg.String() == "ctrl+enter" || msg.String() == "enter" || msg.String() == "\r" || msg.String() == "\n" {
			model.setSessionFocus(prefs.PaneInput)
		} else {
			return model, nil
		}
	}

	if next, cmd, handled := model.updateSessionInputKey(msg); handled {
		return next, cmd
	}
	previous := model.sessionInput.Value()
	var cmd tea.Cmd
	model.sessionInput, cmd = model.sessionInput.Update(msg)
	if model.sessionInput.Value() != previous {
		model.previousInput = previous
	}
	model.refreshSuggestions()
	if !model.reviewPinned {
		model.review = model.resolveReference(model.sessionInput.Value())
	}
	return model, cmd
}

func (m Model) updateSessionLeadingKey(msg tea.KeyPressMsg, focus prefs.Pane) (tea.Model, tea.Cmd, bool) {
	if msg.String() == "ctrl+up" || msg.String() == "ctrl+down" {
		m.setSessionFocus(prefs.PaneTranscript)
		var cmd tea.Cmd
		m.transcriptView, cmd = m.transcriptView.Update(msg)
		return m, cmd, true
	}
	if msg.Code == tea.KeyEnter && msg.Mod&tea.ModShift != 0 {
		m.appendSessionInputNewline()
		return m, nil, true
	}
	switch msg.String() {
	case "/":
		next, cmd := m.openSearch()
		return next, cmd, true
	case "ctrl+e":
		next, cmd := m.endSession()
		return next, cmd, true
	case "ctrl+p":
		m.layout.CycleSessionUpperVisibility()
		m.persistPreferences()
		return m, nil, true
	case "ctrl+o":
		return m, nil, m.openPeekPreview()
	case "-":
		return m.closeFocusedSessionPane(), nil, true
	case "tab":
		// Tab completes only mid-draft. On an empty draft the suggestion list
		// is just a hint, and Tab must cycle panes as the help promises.
		if focus == prefs.PaneInput && len(m.suggestions) > 0 && strings.TrimSpace(m.sessionInput.Value()) != "" {
			m.acceptSuggestion()
		} else {
			m.cycleSessionFocus()
		}
		return m, nil, true
	case "shift+tab":
		m.cycleSessionFocusReverse()
		return m, nil, true
	case "esc":
		m.sessionInput.Blur()
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m Model) closeFocusedSessionPane() Model {
	switch m.sessionFocus() {
	case prefs.PaneCampaign:
		if !m.layout.CloseSessionUpperPane(prefs.PaneCampaign) {
			m.status = "Keep at least one upper pane"
			return m
		}
		m.persistPreferences()
		m.setSessionFocus(prefs.PaneContext)
		m.status = "Closed campaign pane · Ctrl+P to restore"
	case prefs.PaneContext:
		if !m.layout.CloseSessionUpperPane(prefs.PaneContext) {
			m.status = "Keep at least one upper pane"
			return m
		}
		m.persistPreferences()
		m.setSessionFocus(prefs.PaneCampaign)
		m.status = "Closed context pane · Ctrl+P to restore"
	default:
		m.status = "Focus campaign or context, then - to close"
	}
	return m
}

func (m Model) updateSessionPaneKey(msg tea.KeyPressMsg, focus prefs.Pane) (Model, tea.Cmd, bool) {
	switch focus {
	case prefs.PaneTranscript:
		return m.updateTranscriptPaneKey(msg)
	case prefs.PaneCampaign:
		handled := isSessionCampaignKey(msg.String())
		if m.sessionPlannedNotes() != nil {
			handled = isSessionPrepKey(msg.String())
		}
		return m.updateCampaignPaneKey(msg), nil, handled
	case prefs.PaneContext:
		return m.updateContextPaneKey(msg), nil, isSessionContextKey(msg.String())
	default:
		return m, nil, false
	}
}

func (m Model) updateTranscriptPaneKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down", "pgdown":
		key := tea.KeyDown
		if msg.String() == "pgdown" {
			key = tea.KeyPgDown
		}
		var cmd tea.Cmd
		m.transcriptView, cmd = m.transcriptView.Update(tea.KeyPressMsg(tea.Key{Code: key}))
		return m, cmd, true
	case "k", "up", "pgup":
		key := tea.KeyUp
		if msg.String() == "pgup" {
			key = tea.KeyPgUp
		}
		var cmd tea.Cmd
		m.transcriptView, cmd = m.transcriptView.Update(tea.KeyPressMsg(tea.Key{Code: key}))
		return m, cmd, true
	default:
		return m, nil, false
	}
}

func (m Model) updateCampaignPaneKey(msg tea.KeyPressMsg) Model {
	if m.sessionPlannedNotes() != nil {
		return m.updatePrepPaneKey(msg.String())
	}
	switch msg.String() {
	case "j", "down":
		m.campaignCursor = clamp(m.campaignCursor+1, 0, max(0, len(m.campaignNavItems())-1))
	case "k", "up":
		m.campaignCursor = clamp(m.campaignCursor-1, 0, max(0, len(m.campaignNavItems())-1))
	case "enter":
		m.activateCampaignCursor()
	}
	return m
}

func (m Model) updatePrepPaneKey(key string) Model {
	switch key {
	case "j":
		m.movePrepBeat(1)
	case "k":
		m.movePrepBeat(-1)
	case "down":
		m.scrollSessionPrep(1)
	case "up":
		m.scrollSessionPrep(-1)
	case "d":
		m.togglePrepBeatState(beatDone)
	case "x":
		m.togglePrepBeatState(beatSkipped)
	case "pgdown":
		m.scrollSessionPrep(m.sessionPrepPageSize())
	case "pgup":
		m.scrollSessionPrep(-m.sessionPrepPageSize())
	case "home":
		plan := m.runSheetPlan()
		if plan != nil {
			m.ensurePrepState()
			m.prepBeat[m.prepRunKey(plan.ID)] = 0
		}
		m.prepScroll = 0
	case "end":
		plan := m.runSheetPlan()
		if plan != nil {
			beats := domain.ParsePlannedOutline(plan.Body)
			if len(beats) > 0 {
				m.ensurePrepState()
				m.prepBeat[m.prepRunKey(plan.ID)] = len(beats) - 1
			}
		}
		m.prepScroll = 0
	}
	return m
}

func isSessionCampaignKey(key string) bool {
	return key == "j" || key == "down" || key == "k" || key == "up" || key == "enter"
}

func isSessionPrepKey(key string) bool {
	return key == "j" || key == "down" || key == "k" || key == "up" ||
		key == "pgdown" || key == "pgup" || key == "home" || key == "end" || key == "d" || key == "x"
}

func (m Model) updateContextPaneKey(msg tea.KeyPressMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.contextCursor = clamp(m.contextCursor+1, 0, max(0, len(m.sessionContextItems())-1))
	case "k", "up":
		m.contextCursor = clamp(m.contextCursor-1, 0, max(0, len(m.sessionContextItems())-1))
	case "enter":
		m.activateContextCursor()
	case "p":
		if m.review != nil {
			m.reviewPinned = !m.reviewPinned
		}
	case "esc", "x":
		m.review = nil
		m.reviewPinned = false
	}
	return m
}

func isSessionContextKey(key string) bool {
	return key == "j" || key == "down" || key == "k" || key == "up" || key == "enter" ||
		key == "p" || key == "esc" || key == "x"
}

func (m Model) updateSessionInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "enter", "ctrl+enter", "\r", "\n":
		next, cmd := m.submitTranscript()
		return next, cmd, true
	case "shift+enter":
		m.appendSessionInputNewline()
		return m, nil, true
	case "ctrl+z":
		return m.undoSessionInputOrEntry(), nil, true
	case "ctrl+y":
		return m.redoSessionEntry(), nil, true
	case "up", "down":
		return m.moveSessionSuggestion(msg.String()), nil, len(m.suggestions) > 0
	case "pgup", "pgdown":
		return m.moveSessionPeek(msg.String()), nil, m.peek != nil
	default:
		return m, nil, false
	}
}

func (m *Model) appendSessionInputNewline() {
	m.setSessionFocus(prefs.PaneInput)
	previous := m.sessionInput.Value()
	m.sessionInput.SetValue(previous + "\n")
	m.previousInput = previous
}

func (m Model) undoSessionInputOrEntry() Model {
	if m.sessionInput.Value() != "" && m.previousInput != "" {
		m.sessionInput.SetValue(m.previousInput)
		m.refreshSuggestions()
		return m
	}
	for index := len(m.session.Entries) - 1; index >= 0; index-- {
		if !m.session.Entries[index].Undone {
			m.session.Entries[index].Undone = true
			m.refreshTranscriptViewport()
			m.status = "Undid transcript entry · Ctrl+Y restores it"
			m.persistWorkspace()
			return m
		}
	}
	return m
}

func (m Model) redoSessionEntry() Model {
	for index := len(m.session.Entries) - 1; index >= 0; index-- {
		if m.session.Entries[index].Undone {
			m.session.Entries[index].Undone = false
			m.refreshTranscriptViewport()
			m.status = "Restored transcript entry"
			m.persistWorkspace()
			return m
		}
	}
	return m
}

func (m Model) moveSessionSuggestion(key string) Model {
	if key == "up" {
		m.suggestion = clamp(m.suggestion-1, 0, len(m.suggestions)-1)
	} else {
		m.suggestion = clamp(m.suggestion+1, 0, len(m.suggestions)-1)
	}
	m.refreshPeek()
	return m
}

func (m Model) moveSessionPeek(key string) Model {
	if key == "pgup" {
		m.scrollPeek(-1)
	} else {
		m.scrollPeek(1)
	}
	return m
}

func (m *Model) cycleSessionFocus() {
	order := m.sessionFocusOrder()
	if len(order) == 0 {
		return
	}
	current := m.sessionFocus()
	for index, pane := range order {
		if pane == current {
			m.setSessionFocus(order[(index+1)%len(order)])
			return
		}
	}
	m.setSessionFocus(order[0])
}

func (m *Model) cycleSessionFocusReverse() {
	order := m.sessionFocusOrder()
	if len(order) == 0 {
		return
	}
	current := m.sessionFocus()
	for index, pane := range order {
		if pane == current {
			m.setSessionFocus(order[(index-1+len(order))%len(order)])
			return
		}
	}
	m.setSessionFocus(order[len(order)-1])
}

func (m Model) sessionFocusOrder() []prefs.Pane {
	order := make([]prefs.Pane, 0, 4)
	if m.layout.SessionLeafVisible(prefs.PaneCampaign) {
		order = append(order, prefs.PaneCampaign)
	}
	if m.layout.SessionLeafVisible(prefs.PaneContext) {
		order = append(order, prefs.PaneContext)
	}
	order = append(order, prefs.PaneTranscript, prefs.PaneInput)
	return order
}

func (m Model) campaignNavItems() []domain.EntityType {
	return []domain.EntityType{domain.NPC, domain.Location, domain.Faction, domain.Thread, domain.Item, domain.Note}
}

func (m *Model) activateCampaignCursor() {
	items := m.campaignNavItems()
	if len(items) == 0 {
		return
	}
	entityType := items[clamp(m.campaignCursor, 0, len(items)-1)]
	for _, record := range m.operationalRecords() {
		if record.Type == entityType && record.Authority != domain.Proposal {
			candidate := record
			m.review = &candidate
			m.status = "Reviewing " + record.Title
			return
		}
	}
}

func (m *Model) activateContextCursor() {
	items := m.sessionContextItems()
	if len(items) == 0 {
		return
	}
	record := items[clamp(m.contextCursor, 0, len(items)-1)]
	m.review = &record
	m.status = "Reviewing " + record.Title
}

func (m Model) submitTranscript() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.sessionInput.Value())
	if text == "" || m.session == nil {
		return m, nil
	}
	links := m.resolveLinks(text)
	rolls := m.evaluateRolls(text)
	m.handleSessionCommand(text)
	commandStatus := m.status
	m.associateSessionLinks(links)
	now := time.Now().UTC()
	m.session.Entries = append(m.session.Entries, domain.TranscriptEntry{
		ID:        fmt.Sprintf("entry-%d", now.UnixNano()),
		Text:      text,
		CreatedAt: now,
		Links:     links,
		Rolls:     rolls,
		Revision:  1,
	})
	m.sessionInput.SetValue("")
	m.refreshPeek()
	m.refreshTranscriptViewport()
	m.status = "Captured transcript entry"
	if len(rolls) > 0 {
		m.status = formatRollStatus(rolls)
	} else if commandStatus != "" && commandStatus != "Captured transcript entry" {
		m.status = commandStatus
	}
	if m.store != nil {
		m.persistWorkspace()
	}
	return m, nil
}

func (m Model) evaluateRolls(text string) []domain.RollResult {
	matches := dice.Extract(text)
	if len(matches) == 0 {
		return nil
	}
	rng := m.rollRNG
	if rng == nil {
		rng = dice.DefaultRNG()
	}
	now := time.Now().UTC().UnixNano()
	results := make([]domain.RollResult, 0, len(matches))
	for index, match := range matches {
		evaluated, err := dice.Evaluate(match.Expression, rng)
		if err != nil {
			continue
		}
		results = append(results, domain.RollResult{
			ID:         fmt.Sprintf("roll-%d-%d", now, index),
			Label:      match.Label,
			Expression: match.Expression,
			Total:      evaluated.Total,
			Detail:     evaluated.Detail,
			Rolls:      evaluated.Rolls,
		})
	}
	return results
}

func formatRollStatus(rolls []domain.RollResult) string {
	parts := make([]string, 0, len(rolls))
	for _, roll := range rolls {
		label := roll.Expression
		if roll.Label != "" {
			label = roll.Label + " " + roll.Expression
		}
		parts = append(parts, fmt.Sprintf("%s → %s", label, roll.Detail))
	}
	return "Rolled " + strings.Join(parts, " · ")
}

func (m *Model) refreshTranscriptViewport() {
	if m.session == nil {
		return
	}
	var builder strings.Builder
	for _, entry := range m.session.Entries {
		stamp := entry.CreatedAt.Local().Format("15:04")
		builder.WriteString(stamp)
		builder.WriteString("  ")
		if entry.Undone {
			builder.WriteString("[undone] ")
		}
		builder.WriteString(entry.Text)
		builder.WriteRune('\n')
		for _, roll := range entry.Rolls {
			builder.WriteString("      roll ")
			if roll.Label != "" {
				builder.WriteString(roll.Label)
				builder.WriteString(" ")
			}
			builder.WriteString(roll.Expression)
			builder.WriteString(" → ")
			builder.WriteString(roll.Detail)
			builder.WriteRune('\n')
		}
	}
	m.transcriptView.SetContent(builder.String())
	m.transcriptView.GotoBottom()
}

func (m *Model) configureTranscriptViewport() {
	transcriptHeight := m.sessionTranscriptHeight()
	inner := panelInnerHeight(transcriptHeight)
	// Leave one row for the "SESSION TRANSCRIPT" title above the viewport.
	m.transcriptView.SetWidth(max(1, m.width-4))
	m.transcriptView.SetHeight(max(1, inner-1))
}

func (m *Model) associateSessionLinks(links []domain.EntityLink) {
	if m.session == nil {
		return
	}
	for _, link := range links {
		*m.session = m.session.Associate(link)
	}
}

func (m *Model) handleSessionCommand(text string) {
	if strings.HasPrefix(text, "$") {
		if record, ok := parseEntityCommand(text, m.workspace.Scope, m.session); ok {
			m.workspace.Records = append(m.workspace.Records, record)
			m.rebuildSearch()
			m.refreshResults()
			m.review = &record
			if m.session != nil {
				*m.session = m.session.Associate(domain.EntityLink{Text: record.Title, RecordID: record.ID})
			}
			m.status = "Created draft entity: " + record.Title
		} else {
			m.status = "Use $type Name: description"
		}
		return
	}
	if strings.HasPrefix(text, "#location") {
		m.applyLocationCommand(text)
		return
	}
	if strings.HasPrefix(text, "#random ") {
		kind := strings.TrimSpace(strings.TrimPrefix(text, "#random "))
		record, ok := randomDraft(kind, m.workspace.Scope, m.session)
		if !ok {
			m.status = "Use #random npc|character|item|location"
			return
		}
		m.workspace.Records = append(m.workspace.Records, record)
		m.rebuildSearch()
		m.refreshResults()
		m.review = &record
		m.status = "Generated draft entity: " + record.Title
	}
}

func (m *Model) applyLocationCommand(text string) {
	if m.session == nil {
		return
	}
	argument := strings.TrimSpace(strings.TrimPrefix(text, "#location"))
	if argument == "" || strings.EqualFold(argument, "CURRENTLOCATION") {
		name := m.session.LocationName
		if name == "" {
			m.status = "No current location · use #location Name"
			return
		}
		m.status = "Current location: " + name
		if location := m.sessionLocationRecord(); location != nil {
			m.review = location
		}
		return
	}
	if record := m.findLocation(argument); record != nil {
		m.session.LocationID = record.ID
		m.session.LocationName = record.Title
		m.review = record
		m.status = "Current location: " + record.Title
		return
	}
	m.session.LocationID = ""
	m.session.LocationName = argument
	m.status = "Current location: " + argument + " (unlinked)"
}

func parseEntityCommand(text string, scope domain.Scope, session *domain.SessionRecord) (domain.Record, bool) {
	command := strings.TrimSpace(strings.TrimPrefix(text, "$"))
	parts := strings.SplitN(command, " ", 2)
	if len(parts) != 2 {
		return domain.Record{}, false
	}
	entityType, ok := commandEntityType(parts[0])
	if !ok {
		return domain.Record{}, false
	}
	nameDescription := strings.SplitN(parts[1], ":", 2)
	title := strings.TrimSpace(nameDescription[0])
	if title == "" {
		return domain.Record{}, false
	}
	body := ""
	if len(nameDescription) == 2 {
		body = strings.TrimSpace(nameDescription[1])
	}
	source := "Session capture"
	if session != nil {
		source = session.Title
	}
	record := domain.Record{
		ID:        fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(title, " ", "-")), time.Now().UnixNano()),
		Type:      entityType,
		Title:     title,
		Summary:   body,
		Body:      body,
		Authority: domain.Draft,
		Scope:     scope,
		Source:    source,
		Tags:      []string{"session-created"},
	}
	return record, true
}

func randomDraft(kind string, scope domain.Scope, session *domain.SessionRecord) (domain.Record, bool) {
	entityType, ok := commandEntityType(kind)
	if !ok {
		return domain.Record{}, false
	}
	source := "Local session generator"
	if session != nil {
		source = session.Title + " · local generator"
	}
	now := time.Now()
	title := fmt.Sprintf("Generated %s", strings.ToLower(string(entityType)))
	summary := "A context-aware local generator draft."
	body := "Generated during the active session. Review and edit before treating this as campaign truth."
	if session != nil && session.LocationName != "" {
		summary = "Draft generated near " + session.LocationName + "."
		body = "Generated during the active session at " + session.LocationName + ". Review and edit before treating this as campaign truth."
	}
	return domain.Record{
		ID:        fmt.Sprintf("generated-%d", now.UnixNano()),
		Type:      entityType,
		Title:     title,
		Summary:   summary,
		Body:      body,
		Authority: domain.Draft,
		Scope:     scope,
		Source:    source,
		Tags:      []string{"generated", "session-created"},
	}, true
}

func commandEntityType(value string) (domain.EntityType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "npc":
		return domain.NPC, true
	case "character", "pc", "player":
		return domain.Character, true
	case "item":
		return domain.Item, true
	case "location":
		return domain.Location, true
	case "faction":
		return domain.Faction, true
	case "creature":
		return domain.Creature, true
	case "thread", "quest":
		return domain.Thread, true
	case "event":
		return domain.Event, true
	case "note":
		return domain.Note, true
	default:
		return "", false
	}
}

func (m Model) endSession() (tea.Model, tea.Cmd) {
	if m.session == nil {
		return m, nil
	}
	ended := time.Now().UTC()
	session := *m.session
	session.EndedAt = &ended
	entrySnapshot := append([]domain.TranscriptEntry(nil), session.Entries...)
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		ws, _, err := app.EndSession(ws, session)
		return ws, err
	})
	if err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.sessionInput.Blur()
	m.session = nil
	recon := domain.ReconciliationRecord{}
	for index, item := range m.workspace.Reconciliations {
		if item.SessionID == session.ID {
			m.reconIndex = index
			recon = item
			break
		}
	}
	m.status = fmt.Sprintf("Session ended · %d entries · %d review items", len(entrySnapshot), len(recon.Items))
	if !transcriptMatches(m.workspace.Sessions, session.ID, entrySnapshot) {
		m.status = "Session ended · transcript integrity check failed"
		return m, nil
	}
	if len(m.unresolvedReconciliationItems()) > 0 {
		return m.openReconciliation()
	}
	return m, nil
}

func (m *Model) upsertSession(session domain.SessionRecord) {
	for index := range m.workspace.Sessions {
		if m.workspace.Sessions[index].ID == session.ID {
			m.workspace.Sessions[index] = session
			return
		}
	}
	m.workspace.Sessions = append(m.workspace.Sessions, session)
}

func (m *Model) upsertReconciliation(recon domain.ReconciliationRecord) {
	for index := range m.workspace.Reconciliations {
		if m.workspace.Reconciliations[index].SessionID == recon.SessionID {
			m.workspace.Reconciliations[index] = recon
			m.reconIndex = index
			return
		}
	}
	m.workspace.Reconciliations = append(m.workspace.Reconciliations, recon)
	m.reconIndex = len(m.workspace.Reconciliations) - 1
}

func transcriptMatches(sessions []domain.SessionRecord, sessionID string, entries []domain.TranscriptEntry) bool {
	for _, session := range sessions {
		if session.ID == sessionID {
			return slices.EqualFunc(session.Entries, entries, func(a, b domain.TranscriptEntry) bool {
				return a.Text == b.Text
			})
		}
	}
	return false
}

func (m Model) openReconciliation() (tea.Model, tea.Cmd) {
	if len(m.workspace.Reconciliations) == 0 {
		m.status = "No reconciliations yet · end a session first"
		return m, nil
	}
	m.reconciling = true
	m.reconEditing = false
	m.clearDestructiveConfirm("")
	m.reconIndex = clamp(m.reconIndex, 0, len(m.workspace.Reconciliations)-1)
	m.selectFirstUnresolvedReconciliationItem()
	m.status = "Review inbox · a accept  e edit  d defer  x reject"
	return m, nil
}

func (m *Model) clearDestructiveConfirm(status string) {
	m.deleteConfirm = false
	m.confirmKind = ""
	if status != "" {
		m.status = status
	}
}

func (m Model) armDestructiveConfirm(kind string) (tea.Model, tea.Cmd) {
	if m.usesCampaignTree() && m.currentNav().Kind == NavSessions {
		if kind == "supersede" {
			m.status = "Sessions cannot be superseded · d deletes"
			return m, nil
		}
		session := m.selectedSession()
		if session == nil {
			m.status = "Nothing selected"
			return m, nil
		}
		m.deleteConfirm = true
		m.confirmKind = "delete-session"
		m.status = "Delete session “" + session.Title + "”?  y confirm · n/Esc cancel"
		return m, nil
	}
	record := m.selectedRecord()
	if record == nil {
		m.status = "Nothing selected"
		return m, nil
	}
	if kind == "supersede" && record.Authority == domain.Superseded {
		m.status = record.Title + " is already superseded"
		return m, nil
	}
	m.deleteConfirm = true
	m.confirmKind = kind
	verb := "Delete"
	if kind == "supersede" {
		verb = "Supersede"
	}
	m.status = verb + " “" + record.Title + "”?  y confirm · n/Esc cancel"
	return m, nil
}

func (m Model) confirmDestructiveAction() (tea.Model, tea.Cmd) {
	if !m.deleteConfirm {
		return m, nil
	}
	kind := m.confirmKind
	m.clearDestructiveConfirm("")
	switch kind {
	case "delete":
		m.deleteSelected()
	case "delete-session":
		m.deleteSelectedSession()
	case "supersede":
		return m.supersedeSelected()
	}
	return m, nil
}

func (m Model) closeFocusedPane() (tea.Model, tea.Cmd) {
	pane := m.layout.Focus
	if m.usesCampaignTree() {
		m.status = "Campaign tree panes stay open · Tab cycles focus"
		return m, nil
	}
	if _, ok := paneEntityType(pane); ok && pane != prefs.PaneList && pane != prefs.PaneDetail {
		if !m.layout.CloseBrowserTypePane(pane) {
			m.status = "Keep at least one type pane open"
			return m, nil
		}
		m.persistPreferences()
		m.cycleBrowserFocus()
		m.status = "Closed " + string(pane) + " pane · + to restore"
		return m, nil
	}
	if pane == prefs.PaneDetail {
		m.status = "Detail pane stays open"
		return m, nil
	}
	m.status = "Focus a type pane, then press - to close it"
	return m, nil
}

func (m Model) activateBrowserSelection() (tea.Model, tea.Cmd) {
	if !m.usesCampaignTree() {
		return m, nil
	}
	if m.layout.Focus == prefs.PaneDetail {
		if updated, cmd, ok := m.followDetailHop(); ok {
			return updated, cmd
		}
	}
	switch m.currentNav().Kind {
	case NavHome:
		return m.activateHomeAction(m.cursor)
	case NavPrep:
		return m.activatePrepSelection()
	case NavSessions:
		return m.activateSessionSelection()
	case NavSources:
		return m.activateSourceSelection()
	default:
		return m.activateWikiSelection()
	}
}

func (m Model) activatePrepSelection() (tea.Model, tea.Cmd) {
	if m.selectedPlanID == "" {
		return m.openPlannedNotes(true)
	}
	m.planID = m.selectedPlanID
	return m.openPlannedNotes(false)
}

func (m Model) activateSessionSelection() (tea.Model, tea.Cmd) {
	rows := m.sessionTreeRows()
	if len(rows) > 0 && rows[clamp(m.cursor, 0, len(rows)-1)].Kind == domain.SessionTreeFolder {
		m.toggleSelectedFolder()
		return m, nil
	}
	session := m.selectedSession()
	if session == nil {
		return m.startSession()
	}
	if session.EndedAt != nil {
		if index := m.unresolvedReconciliationForSession(session.ID); index >= 0 {
			m.reconIndex = index
			return m.openReconciliation()
		}
		return m.openPlayback()
	}
	return m.resumeSelectedSession(session)
}

func (m Model) resumeSelectedSession(session *domain.SessionRecord) (tea.Model, tea.Cmd) {
	m.session = session
	m.sessionInput.Focus()
	m.configureTranscriptViewport()
	m.refreshTranscriptViewport()
	m.refreshSuggestions()
	m.status = "Resumed " + session.Title + " · Ctrl+E ends capture"
	return m, nil
}

func (m Model) activateSourceSelection() (tea.Model, tea.Cmd) {
	rows := m.sourceListRows()
	if len(rows) == 0 {
		return m, nil
	}
	if sourceFolderRow(rows[clamp(m.cursor, 0, len(rows)-1)].Kind) {
		m.toggleSourceFolder()
	}
	return m, nil
}

func sourceFolderRow(kind sourceRowKind) bool {
	return kind == sourceRowBook || kind == sourceRowChapter
}

func (m Model) activateWikiSelection() (tea.Model, tea.Cmd) {
	if m.selectedRecord() != nil {
		return m.openEditor(false)
	}
	rows := m.recordTreeRows()
	if len(rows) > 0 && rows[clamp(m.cursor, 0, len(rows)-1)].Kind == domain.RecordTreeFolder {
		m.toggleWikiFolder()
	}
	return m, nil
}

func (m *Model) moveBrowserCursor(delta int) {
	if m.usesCampaignTree() && m.layout.Focus == prefs.PaneNav {
		m.moveNavCursor(delta)
		return
	}
	if m.usesCampaignTree() && (m.layout.Focus == prefs.PaneList || m.layout.Focus == prefs.PaneDetail) {
		switch m.currentNav().Kind {
		case NavHome:
			actions := m.homeActions()
			if len(actions) == 0 {
				return
			}
			m.cursor = clamp(m.cursor+delta, 0, len(actions)-1)
			return
		case NavSessions:
			rows := m.sessionTreeRows()
			if len(rows) == 0 {
				return
			}
			m.applySessionTreeCursor(m.cursor + delta)
			return
		case NavPrep:
			plans := m.scopedPlannedNotes()
			if len(plans) == 0 {
				return
			}
			m.cursor = clamp(m.cursor+delta, 0, len(plans)-1)
			m.selectedPlanID = plans[m.cursor].ID
			m.selectedID = ""
			m.selectedSessionID = ""
			return
		case NavSources:
			rows := m.sourceListRows()
			if len(rows) == 0 {
				return
			}
			m.cursor = clamp(m.cursor+delta, 0, len(rows)-1)
			m.bindSourceListRow(rows[m.cursor])
			return
		default:
			rows := m.recordTreeRows()
			if len(rows) == 0 {
				return
			}
			m.applyRecordTreeCursor(m.cursor + delta)
			m.layout.Focus = prefs.PaneList
			return
		}
	}
	pane := m.layout.Focus
	if _, ok := paneEntityType(pane); !ok {
		if panes := m.browserFocusOrder(); len(panes) > 0 {
			pane = panes[0]
			m.setBrowserFocus(pane)
		}
	}
	records := m.recordsForPane(pane)
	rows := m.recordTreeRowsFor(records)
	if len(rows) == 0 {
		return
	}
	m.cursor = clamp(m.cursor+delta, 0, len(rows)-1)
	m.bindRecordTreeRow(rows[m.cursor])
}

func clampBrowserRatio(value float64) float64 {
	if value < 0.15 {
		return 0.15
	}
	if value > 0.7 {
		return 0.7
	}
	return value
}

func (m *Model) deleteSelected() {
	record := m.selectedRecord()
	if record == nil {
		return
	}
	fallback := neighborID(recordIDs(m.selectionRecords()), record.ID)
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.DeleteRecord(ws, record.ID)
	})
	if err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
		return
	}
	m.replaceWorkspace(next)
	m.rebuildSearch()
	m.selectedID = ""
	if replacement, ok := recordByID(m.workspace.Records, fallback); ok {
		m.selectRecord(replacement)
	}
	m.ensureBrowserSelection()
	m.refreshResults()
	m.status = "Deleted " + record.Title
}

func (m Model) selectedSession() *domain.SessionRecord {
	if m.selectedSessionID == "" {
		return nil
	}
	for index := range m.workspace.Sessions {
		if m.workspace.Sessions[index].ID == m.selectedSessionID {
			session := m.workspace.Sessions[index]
			return &session
		}
	}
	return nil
}

func (m *Model) deleteSelectedSession() {
	session := m.selectedSession()
	if session == nil {
		return
	}
	fallback := neighborID(sessionRowIDs(m.sessionTreeRows()), session.ID)
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.DeleteSession(ws, session.ID)
	})
	if err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
		return
	}
	m.replaceWorkspace(next)
	if m.session != nil && m.session.ID == session.ID {
		m.session = nil
	}
	m.selectedSessionID = fallback
	m.selectedFolderPath = ""
	m.syncSessionTreeCursor()
	m.status = "Deleted session " + session.Title
}

func (m Model) supersedeSelected() (tea.Model, tea.Cmd) {
	record := m.selectedRecord()
	if record == nil {
		m.status = "Nothing selected to supersede"
		return m, nil
	}
	if record.Authority == domain.Superseded {
		m.status = record.Title + " is already superseded"
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.SupersedeRecord(ws, record.ID)
	})
	if err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	if updated := m.recordByID(record.ID); updated != nil {
		m.selectRecord(*updated)
	}
	m.rebuildSearch()
	m.clearDestructiveConfirm("")
	m.status = "Superseded " + record.Title + " · still searchable as history"
	return m, nil
}

func (m Model) updateReconciliation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if len(m.workspace.Reconciliations) == 0 {
		m.reconciling = false
		m.reconEditing = false
		return m, nil
	}
	if m.reconEditing {
		return m.updateReconEdit(msg)
	}
	switch msg.String() {
	case "esc", "q":
		m.reconciling = false
		m.status = "Closed reconciliation"
		return m, nil
	case "j", "down":
		m.moveReconciliationCursor(1)
		return m, nil
	case "k", "up":
		m.moveReconciliationCursor(-1)
		return m, nil
	case "e":
		return m.startReconEdit(&m.workspace.Reconciliations[m.reconIndex])
	case "a":
		return m.approveReconciliationItem(&m.workspace.Reconciliations[m.reconIndex])
	case "d":
		return m.deferReconciliationItem()
	case "x":
		return m.rejectReconciliationItem(&m.workspace.Reconciliations[m.reconIndex])
	}
	return m, nil
}

func (m Model) startReconEdit(recon *domain.ReconciliationRecord) (tea.Model, tea.Cmd) {
	if len(recon.Items) == 0 {
		return m, nil
	}
	item := recon.Items[m.reconCursor]
	if !domain.ReconciliationUnresolved(item.Status) {
		m.status = "Only unresolved mutations can be edited"
		return m, nil
	}
	edit := newMarkdownTextArea(m.width, 8)
	edit.Placeholder = "Editable factual mutation · transcript stays immutable"
	edit.SetValue(item.Mutation.Text)
	edit.Focus()
	m.reconEdit = edit
	m.reconEditing = true
	m.status = "Edit mutation · Ctrl+S save · Esc cancel"
	return m, textarea.Blink
}

func (m Model) updateReconEdit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.reconEditing = false
		m.reconEdit.Blur()
		m.status = "Mutation edit cancelled"
		return m, nil
	case "ctrl+s":
		return m.saveReconEdit()
	}
	var cmd tea.Cmd
	m.reconEdit, cmd = m.reconEdit.Update(msg)
	return m, cmd
}

func (m Model) saveReconEdit() (tea.Model, tea.Cmd) {
	if m.reconIndex < 0 || m.reconIndex >= len(m.workspace.Reconciliations) {
		m.reconciling = false
		m.reconEditing = false
		return m, nil
	}
	recon := &m.workspace.Reconciliations[m.reconIndex]
	if m.reconCursor < 0 || m.reconCursor >= len(recon.Items) {
		m.reconEditing = false
		return m, nil
	}
	text := strings.TrimSpace(m.reconEdit.Value())
	if text == "" {
		m.status = "Mutation text is required"
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.EditReconMutation(ws, m.reconIndex, m.reconCursor, text)
	})
	if err != nil {
		m.status = "Mutation edit rolled back: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.reconciling = true
	m.reconEditing = false
	m.reconEdit.Blur()
	m.status = "Mutation updated · a apply to wiki"
	return m, nil
}

func (m Model) approveReconciliationItem(recon *domain.ReconciliationRecord) (tea.Model, tea.Cmd) {
	if len(recon.Items) == 0 {
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.ApplyRecon(ws, m.reconIndex, m.reconCursor)
	})
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.rebuildSearch()
	m.advanceReconciliationCursor()
	m.status = m.reconciliationResultStatus("Accepted")
	return m, nil
}

func (m Model) rejectReconciliationItem(recon *domain.ReconciliationRecord) (tea.Model, tea.Cmd) {
	if len(recon.Items) == 0 {
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.RejectRecon(ws, m.reconIndex, m.reconCursor)
	})
	if err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.advanceReconciliationCursor()
	m.status = m.reconciliationResultStatus("Rejected")
	return m, nil
}

func (m *Model) persistWorkspace() error {
	m.markWorkspaceChanged()
	if m.session != nil {
		m.upsertSession(*m.session)
	}
	if err := m.app.Save(m.workspace); err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
		return err
	}
	m.persistAdventures()
	return nil
}

func (m *Model) persistAdventures() {
	store := sqliteStore(m.store)
	if store == nil {
		return
	}
	books := make([]storage.Blob, 0, len(m.adventures))
	for _, book := range m.adventures {
		data, err := json.Marshal(book)
		if err != nil {
			continue
		}
		books = append(books, storage.Blob{ID: book.ID, Data: data})
	}
	_ = store.SaveAdventures(books)
}

func (m *Model) applyPreferences() {
	if m.prefs == nil {
		m.layout = prefs.DefaultLayout()
		m.ensureBrowserSelection()
		return
	}
	layout, err := m.prefs.Load()
	if err != nil {
		m.layout = prefs.DefaultLayout()
		m.ensureBrowserSelection()
		return
	}
	m.layout = layout.Normalize()
	m.ensureBrowserSelection()
}

func (m *Model) addNextBrowserTypePane() {
	for _, pane := range prefs.BrowserTypePanes {
		if prefs.FindVisibleLeaf(m.layout.Browser.Root, pane) {
			continue
		}
		if m.layout.AddBrowserTypePane(pane) {
			m.persistPreferences()
			m.setBrowserFocus(pane)
			m.status = "Added " + string(pane) + " pane"
			return
		}
	}
	m.status = "All type panes already visible"
}

func (m *Model) persistPreferences() {
	if m.prefs == nil {
		return
	}
	err := m.prefs.Save(m.layout)
	if err != nil && m.status == "" {
		m.status = "Layout preference save failed: " + err.Error()
	}
}

func (m Model) resolveReference(text string) *domain.Record {
	last := strings.LastIndex(text, "@")
	if last < 0 || last+1 >= len(text) {
		return nil
	}
	query := strings.TrimSpace(text[last+1:])
	if query == "" {
		return nil
	}
	query = strings.TrimRight(query, ".,!?;:")
	var best *domain.Record
	bestScore := 0
	for _, record := range m.operationalRecords() {
		title := strings.ToLower(record.Title)
		needle := strings.ToLower(query)
		score := 0
		if strings.HasPrefix(needle, title) {
			score = 4
		} else if title == needle {
			score = 3
		} else if strings.HasPrefix(title, needle) {
			score = 2
		} else if strings.Contains(title, needle) {
			score = 1
		}
		for _, alias := range record.Aliases {
			alias = strings.ToLower(alias)
			if strings.HasPrefix(needle, alias) && len(alias) > len(title) {
				score = 4
			}
		}
		if score > bestScore {
			candidate := record
			best = &candidate
			bestScore = score
		}
	}
	if best != nil {
		return best
	}
	if rec, ok := m.lookupReference(query); ok {
		return &rec
	}
	return nil
}

func (m Model) resolveLinks(text string) []domain.EntityLink {
	var links []domain.EntityLink
	for index := strings.Index(text, "@"); index >= 0; {
		remaining := text[index+1:]
		if remaining == "" {
			break
		}
		var best *domain.Record
		bestLen := 0
		for _, record := range m.operationalRecords() {
			title := record.Title
			if title == "" || !strings.HasPrefix(strings.ToLower(remaining), strings.ToLower(title)) {
				continue
			}
			end := len(title)
			if end < len(remaining) {
				next := remaining[end]
				if next != ' ' && next != '\n' && next != '\t' && next != ',' && next != ';' && next != '.' && next != '!' && next != '?' {
					continue
				}
			}
			if end > bestLen {
				candidate := record
				best = &candidate
				bestLen = end
			}
		}
		if best != nil {
			links = append(links, domain.EntityLink{Text: remaining[:bestLen], RecordID: best.ID})
		} else if token := domain.ScanMentionName(remaining); token != "" {
			if rec, ok := m.lookupReference(token); ok {
				links = append(links, domain.EntityLink{Text: token, RecordID: rec.ID})
			}
		}
		next := strings.Index(remaining, " @")
		if next < 0 {
			break
		}
		index += next + 1
	}
	return links
}

func (m Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchInput.Blur()
		if m.session != nil {
			m.setSessionFocus(m.sessionFocus())
		}
		return m, nil
	case "ctrl+s":
		m.searchScope = (m.searchScope + 1) % 4
		m.selected = 0
		if m.searchScope == searchsvc.RulesReference {
			return m, m.scheduleRulesSearch()
		}
		m.rulesNote = ""
		m.refreshResults()
		return m, nil
	case "ctrl+g":
		return m.openSettings()
	case "ctrl+a":
		if m.searchScope == searchsvc.RulesReference {
			return m, nil
		}
		m.includeIdeas = !m.includeIdeas
		m.selected = 0
		m.refreshResults()
		return m, nil
	case "up", "ctrl+k":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.selected < len(m.results)-1 {
			m.selected++
		}
		return m, nil
	case "enter":
		if len(m.results) > 0 {
			return m.openSearchResult(m.results[m.selected])
		}
		m.searching = false
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.selected = 0
	if m.searchScope == searchsvc.RulesReference {
		return m, tea.Batch(cmd, m.scheduleRulesSearch())
	}
	m.rulesNote = ""
	m.refreshResults()
	return m, cmd
}

func (m Model) openSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.selected = 0
	m.rulesNote = ""
	if m.session != nil {
		m.sessionInput.Blur()
	}
	m.searchInput.Focus()
	if m.searchScope == searchsvc.RulesReference {
		return m, tea.Batch(textinput.Blink, m.scheduleRulesSearch())
	}
	m.refreshResults()
	return m, textinput.Blink
}

func (m Model) updateMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	if m.preview != nil {
		return m.updatePreviewClick(msg)
	}

	if m.searching {
		return m.updateSearchClick(msg)
	}

	if msg.Y == m.height-1 && msg.X <= 10 {
		return m.openSearch()
	}

	if navGutter := m.browserNavGutterX(); navGutter >= 0 && abs(msg.X-navGutter) <= 1 {
		m.draggingSplit = true
		m.draggingNavSplit = true
		m.dragAxis = "vertical"
		return m, nil
	}
	gutter := m.browserDetailGutterX()
	if abs(msg.X-gutter) <= 1 {
		m.draggingSplit = true
		m.draggingNavSplit = false
		m.dragAxis = "vertical"
		return m, nil
	}
	for _, region := range m.browserRegions() {
		if msg.X < region.MinX || msg.X > region.MaxX || msg.Y < region.MinY || msg.Y > region.MaxY {
			continue
		}
		m.setBrowserFocus(region.Pane)
		index := msg.Y - region.Offset
		switch region.Pane {
		case prefs.PaneNav:
			if index >= 0 && index < len(region.NavEntries) {
				m.setNavCursor(index)
			}
			return m, nil
		case prefs.PaneDetail:
			return m, nil
		case prefs.PaneList:
			if len(region.HomeRows) > 0 {
				if index >= 0 && index < len(region.HomeRows) {
					m.cursor = index
					return m.activateHomeAction(index)
				}
				return m, nil
			}
			if len(region.SessionTree) > 0 {
				if index >= 0 && index < len(region.SessionTree) {
					m.cursor = index
					m.bindSessionTreeRow(region.SessionTree[index])
				}
				return m, nil
			}
			if len(region.PlanRows) > 0 {
				if index >= 0 && index < len(region.PlanRows) {
					m.cursor = index
					m.selectedPlanID = region.PlanRows[index].ID
					m.selectedID = ""
					m.selectedSessionID = ""
				}
				return m, nil
			}
			if len(region.SourceRows) > 0 {
				index = msg.Y - region.Offset + region.WindowStart
				if index >= 0 && index < len(region.SourceRows) {
					m.cursor = index
					m.bindSourceListRow(region.SourceRows[index])
					m.layout.Focus = prefs.PaneList
				}
				return m, nil
			}
			if len(region.RecordTree) > 0 {
				index = msg.Y - region.Offset + region.WindowStart
				if index >= 0 && index < len(region.RecordTree) {
					m.cursor = index
					m.bindRecordTreeRow(region.RecordTree[index])
					m.layout.Focus = prefs.PaneList
				}
				return m, nil
			}
			if index >= 0 && index < len(region.Rows) {
				m.selectRecord(region.Rows[index])
				m.layout.Focus = prefs.PaneList
			}
			return m, nil
		default:
			if len(region.RecordTree) > 0 {
				index = msg.Y - region.Offset + region.WindowStart
				if index >= 0 && index < len(region.RecordTree) {
					m.cursor = index
					m.bindRecordTreeRow(region.RecordTree[index])
				}
				return m, nil
			}
			if len(region.Rows) == 0 {
				return m, nil
			}
			if index >= 0 && index < len(region.Rows) {
				m.selectRecord(region.Rows[index])
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) openEditor(create bool) (tea.Model, tea.Cmd) {
	body := newMarkdownTextArea(m.width, m.height)
	body.Placeholder = "type: NPC\nauthority: draft\nscope: campaign\n\n# Title\n\nSummary paragraph.\n\nBody markdown…"

	model := m
	model.editing = true
	model.creating = create
	model.editID = ""
	model.editType = domain.NPC
	model.status = ""
	model.suggestions = nil
	model.suggestion = 0
	if m.usesCampaignTree() && m.currentNav().Kind == NavType && m.navType != "" {
		model.editType = m.navType
	} else if entityType, ok := paneEntityType(m.layout.Focus); ok && m.layout.Focus != prefs.PaneList && entityType != "" {
		model.editType = entityType
	} else if m.typeFilter != "" {
		model.editType = m.typeFilter
	}
	model.editBody = body
	if create {
		draft := domain.Record{
			Type:      model.editType,
			Title:     "New " + string(model.editType),
			Summary:   "",
			Body:      "",
			Authority: domain.Draft,
			Scope:     model.workspace.Scope,
		}
		model.editBody.SetValue(domain.FormatEntityMarkdown(draft))
	} else {
		record := m.selectedRecord()
		if record == nil {
			return m, nil
		}
		model.editID = record.ID
		model.editType = record.Type
		model.editBody.SetValue(domain.FormatEntityMarkdown(*record))
	}
	focusTitleHeading(&model.editBody)
	return model, model.editBody.Focus()
}

func (m Model) updateEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	model := m
	switch msg.String() {
	case "esc":
		model.editing = false
		model.suggestions = nil
		model.status = "Cancelled edit"
		return model, nil
	case "ctrl+t":
		model.editType = nextEntityType(model.editType)
		rewriteTypePreservingCursor(&model.editBody, model.editType)
		model.status = "Type → " + string(model.editType)
		return model, nil
	case "ctrl+o":
		if model.openPeekPreview() {
			return model, nil
		}
	case "ctrl+s", "ctrl+enter":
		return model.saveEditor()
	case "tab":
		if model.acceptEditorSuggestion() {
			return model, nil
		}
	case "up":
		if len(model.suggestions) > 0 {
			model.suggestion = clamp(model.suggestion-1, 0, len(model.suggestions)-1)
			model.refreshPeek()
			return model, nil
		}
	case "down":
		if len(model.suggestions) > 0 {
			model.suggestion = clamp(model.suggestion+1, 0, len(model.suggestions)-1)
			model.refreshPeek()
			return model, nil
		}
	case "pgup":
		if model.peek != nil {
			model.scrollPeek(-1)
			return model, nil
		}
	case "pgdown":
		if model.peek != nil {
			model.scrollPeek(1)
			return model, nil
		}
	}
	if insertMarkdownText(&model.editBody, msg) {
		model.refreshEditorSuggestions()
		return model, nil
	}
	var cmd tea.Cmd
	model.editBody, cmd = model.editBody.Update(msg)
	model.refreshEditorSuggestions()
	return model, cmd
}

func rewriteMarkdownType(doc string, entityType domain.EntityType) string {
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "type:") {
			lines[index] = "type: " + string(entityType)
			return strings.Join(lines, "\n")
		}
	}
	return "type: " + string(entityType) + "\n\n" + doc
}

func (m Model) saveEditor() (tea.Model, tea.Cmd) {
	parsed, err := domain.ParseEntityMarkdown(m.editBody.Value(), m.editType)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	scope := m.workspace.Scope
	if parsed.ScopeLevel == domain.WorldScope {
		scope.CampaignID = ""
		scope.Campaign = ""
	}
	record := domain.Record{
		ID:        fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(parsed.Record.Title, " ", "-")), time.Now().UnixNano()),
		Type:      parsed.Record.Type,
		Title:     parsed.Record.Title,
		Summary:   parsed.Record.Summary,
		Body:      parsed.Record.Body,
		Authority: parsed.Record.Authority,
		Scope:     scope,
		Source:    "DM-authored",
		Tags:      parsed.Record.Tags,
		// Set from the editor's state: line, so a transition is always an
		// explicit owner edit.
		ThreadState: parsed.Record.ThreadState,
	}
	if !m.creating {
		found := false
		for index := range m.workspace.Records {
			if m.workspace.Records[index].ID == m.editID {
				record = m.workspace.Records[index]
				record.Type = parsed.Record.Type
				record.Title = parsed.Record.Title
				record.Summary = parsed.Record.Summary
				record.Body = parsed.Record.Body
				record.Authority = parsed.Record.Authority
				record.Tags = parsed.Record.Tags
				record.ThreadState = parsed.Record.ThreadState
				wasWorldScoped := record.Scope.CampaignID == ""
				if parsed.ScopeLevel == domain.WorldScope {
					record.Scope.CampaignID = ""
					record.Scope.Campaign = ""
				} else if wasWorldScoped {
					if record.Scope.WorldID != m.workspace.Scope.WorldID {
						m.status = "Open a campaign in this record's world before changing its scope"
						return m, nil
					}
					record.Scope = m.workspace.Scope
				}
				found = true
				break
			}
		}
		if !found {
			m.status = "Entity no longer exists"
			m.editing = false
			return m, nil
		}
	}
	if err := record.Validate(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	if m.creating {
		m.workspace.Records = append(m.workspace.Records, record)
	} else {
		for index := range m.workspace.Records {
			if m.workspace.Records[index].ID == record.ID {
				m.workspace.Records[index] = record
				break
			}
		}
	}
	m.selectRecord(record)
	m.rebuildSearch()
	m.refreshResults()
	m.editing = false
	m.suggestions = nil
	m.status = "Saved " + strings.ToLower(record.Authority.Label()) + " in " + entityScopeLabel(record.Scope) + ": " + record.Title
	if m.store != nil {
		if err := m.store.Save(m.workspace); err != nil {
			m.status = "Saved in memory; persistence failed: " + err.Error()
		}
	}
	return m, nil
}

func (m Model) updateSearchClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	width := min(88, max(52, m.width-10))
	visibleRows := min(10, max(4, m.height-12))
	panelHeight := 8 + min(visibleRows, len(m.results))
	left := (m.width - width - 6) / 2
	top := (m.height - panelHeight - 4) / 2

	const resultsOffset = 5
	index := msg.Y - top - resultsOffset
	if msg.X >= left && msg.X <= left+width+4 && index >= 0 && index < min(visibleRows, len(m.results)) {
		start := 0
		if m.selected >= visibleRows {
			start = m.selected - visibleRows + 1
		}
		m.selected = start + index
		return m.openSearchResult(m.results[m.selected])
	}
	return m, nil
}

func (m Model) updateMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
		delta = 1
	}
	if delta == 0 {
		return m, nil
	}

	if m.preview != nil {
		m.scrollPreview(delta)
		return m, nil
	}

	if m.searching {
		m.selected = clamp(m.selected+delta, 0, max(0, len(m.results)-1))
	} else if m.wheelHitsDetail(msg) || m.layout.Focus == prefs.PaneDetail {
		m.scrollDetail(delta)
	} else {
		m.moveBrowserCursor(delta)
	}
	return m, nil
}

func (m Model) wheelHitsDetail(msg tea.MouseWheelMsg) bool {
	for _, region := range m.browserRegions() {
		if region.Pane != prefs.PaneDetail {
			continue
		}
		if msg.X >= region.MinX && msg.X <= region.MaxX && msg.Y >= region.MinY && msg.Y <= region.MaxY {
			return true
		}
	}
	return false
}

func (m Model) updateSessionMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	upperHeight := m.sessionUpperHeight()
	divider := m.splitWidth(m.width)
	horizontal := upperHeight + 1
	if msg.Y <= upperHeight && abs(msg.X-divider) <= 1 {
		m.draggingSplit = true
		m.dragAxis = "vertical"
		return m, nil
	}
	if abs(msg.Y-horizontal) <= 1 {
		m.draggingSplit = true
		m.dragAxis = "horizontal"
		return m, nil
	}
	if hit, ok := m.hitTest(msg.X, msg.Y); ok {
		return m.applyHit(hit)
	}
	if msg.Y > upperHeight+1 {
		m.setSessionFocus(prefs.PaneInput)
	}
	return m, nil
}

func (m Model) applyHit(hit hitTarget) (tea.Model, tea.Cmd) {
	switch hit.Action {
	case hitSelectRecord:
		m.setSessionFocus(prefs.PaneContext)
		if hit.Record != nil {
			record := *hit.Record
			m.review = &record
			m.status = "Reviewing " + record.Title
		}
	case hitAcceptSuggestion:
		m.setSessionFocus(prefs.PaneInput)
		m.suggestion = hit.Suggestion
		m.acceptSuggestion()
		m.status = "Inserted " + m.suggestionsTitle(hit)
	case hitPinReview:
		m.setSessionFocus(prefs.PaneContext)
		if m.review != nil {
			m.reviewPinned = !m.reviewPinned
			if m.reviewPinned {
				m.status = "Pinned review: " + m.review.Title
			} else {
				m.status = "Unpinned review"
			}
		}
	case hitClearReview:
		m.setSessionFocus(prefs.PaneContext)
		m.review = nil
		m.reviewPinned = false
		m.status = "Cleared entity review"
	case hitCampaignType:
		m.setSessionFocus(prefs.PaneCampaign)
		for index, entityType := range m.campaignNavItems() {
			if entityType == hit.EntityType {
				m.campaignCursor = index
				break
			}
		}
		for _, record := range m.operationalRecords() {
			if record.Type == hit.EntityType && record.Authority != domain.Proposal {
				candidate := record
				m.review = &candidate
				m.status = "Reviewing " + record.Title
				break
			}
		}
	case hitFocusPane:
		m.setSessionFocus(hit.Pane)
		m.status = "Focused " + string(hit.Pane)
	}
	return m, nil
}

func (m Model) suggestionsTitle(hit hitTarget) string {
	if hit.Suggestion >= 0 && hit.Suggestion < len(m.suggestions) {
		return m.suggestions[hit.Suggestion].Insert
	}
	if hit.Record != nil {
		return hit.Record.Title
	}
	return "suggestion"
}

func (m *Model) rebuildSearch() {
	m.markWorkspaceChanged()
	docs := searchsvc.DocumentsFromWorkspace(m.workspace)
	docs = append(docs, m.referenceDocuments()...)
	if store := sqliteStore(m.store); store != nil {
		if err := store.Reindex(docs); err != nil {
			m.search = searchsvc.FromDocuments(docs)
			return
		}
		m.search = store.Search()
		return
	}
	m.search.Close()
	if m.searchPath != "" {
		svc, _ := searchsvc.OpenPath(m.searchPath, docs)
		m.search = svc
		return
	}
	m.search = searchsvc.FromDocuments(docs)
}

func (m *Model) refreshResults() {
	if !m.searching && strings.TrimSpace(m.searchInput.Value()) == "" {
		m.results = nil
		return
	}
	if m.searchScope == searchsvc.RulesReference {
		return
	}
	m.results = m.search.Find(searchsvc.Filter{
		Query:            m.searchInput.Value(),
		Scope:            m.searchScope,
		WorldID:          m.workspace.Scope.WorldID,
		CampaignID:       m.workspace.Scope.CampaignID,
		EnabledSourceIDs: m.workspace.EnabledSourceIDs(m.workspace.Scope),
		IncludeProposals: m.includeIdeas,
	})
}

func (m Model) openSearchResult(result searchsvc.Result) (tea.Model, tea.Cmd) {
	if result.Kind == searchsvc.KindRule {
		return m.openRuleSearchResult(result)
	}
	if m.session != nil {
		return m.openLiveSearchResult(result)
	}
	// Snapshot before closing search so back and preview-close both return to
	// this query and row rather than the bare browser.
	origin := m.snapshotBrowserLocation()
	m.searching = false
	m.searchInput.Blur()
	switch result.Kind {
	case searchsvc.KindPrep:
		m.pushBrowserLocationSnapshot(origin)
		m.focusPrep(result.TargetID())
		return m, nil
	case searchsvc.KindSession, searchsvc.KindTranscript:
		m.pushBrowserLocationSnapshot(origin)
		id := result.SessionID
		if id == "" {
			id = result.TargetID()
		}
		m.focusSession(id)
		return m, nil
	case searchsvc.KindRecon:
		return m.openReconAt(result.TargetID())
	}
	id := result.TargetID()
	if rec, ok := m.lookupAny(id); ok {
		if fivetools.IsReferenceID(rec.ID) {
			m.openPreviewFrom(detailHop{Kind: hopReference, Label: rec.Title, RecordID: rec.ID}, origin.search)
		} else {
			m.pushBrowserLocationSnapshot(origin)
			m.selectRecord(rec)
		}
	}
	return m, nil
}

func (m Model) openLiveSearchResult(result searchsvc.Result) (tea.Model, tea.Cmd) {
	hop, ok := m.searchResultPreviewHop(result)
	if !ok {
		m.status = "End live capture before reviewing reconciliation"
		return m, nil
	}
	m.searching = false
	m.searchInput.Blur()
	// No origin is recorded: D-043 returns live inspection to capture.
	m.openPreviewFrom(hop, searchLocation{})
	return m, nil
}

func (m Model) searchResultPreviewHop(result searchsvc.Result) (detailHop, bool) {
	switch result.Kind {
	case searchsvc.KindPrep:
		return detailHop{Kind: hopPrep, Label: result.DisplayTitle(), PlanID: result.TargetID()}, true
	case searchsvc.KindSession:
		return detailHop{Kind: hopSession, Label: result.DisplayTitle(), SessionID: result.TargetID()}, true
	case searchsvc.KindTranscript:
		return detailHop{Kind: hopHistory, Label: result.DisplayTitle(), SessionID: result.SessionID, EntryID: result.TargetID()}, true
	case searchsvc.KindRecon:
		return detailHop{}, false
	}
	record, ok := m.lookupAny(result.TargetID())
	if !ok {
		return detailHop{}, false
	}
	kind := hopWiki
	if fivetools.IsReferenceID(record.ID) {
		kind = hopReference
	}
	return detailHop{Kind: kind, Label: record.Title, RecordID: record.ID}, true
}

func (m Model) openReconAt(id string) (tea.Model, tea.Cmd) {
	for index, recon := range m.workspace.Reconciliations {
		if recon.ID == id {
			m.reconIndex = index
			return m.openReconciliation()
		}
	}
	m.status = "Reconciliation not found"
	return m, nil
}

func (m Model) visibleRecords() []domain.Record {
	return m.scopedRecords(true)
}

func (m Model) operationalRecords() []domain.Record {
	records := make([]domain.Record, 0, len(m.workspace.Records))
	enabled := m.workspace.EnabledSourceIDs(m.workspace.Scope)
	for _, record := range m.workspace.Records {
		if domain.RecordVisibleIn(record, m.workspace.Scope, enabled) {
			records = append(records, record)
		}
	}
	return records
}

func (m Model) operationalRecordByID(id string) *domain.Record {
	for _, record := range m.operationalRecords() {
		if record.ID == id {
			candidate := record
			return &candidate
		}
	}
	return nil
}

func (m Model) defaultSessionLocation() *domain.Record {
	var fallback *domain.Record
	for _, record := range m.operationalRecords() {
		if record.Type != domain.Location || record.Authority == domain.Proposal {
			continue
		}
		candidate := record
		if containsTag(record, "current-scene") {
			return &candidate
		}
		if fallback == nil {
			fallback = &candidate
		}
	}
	return fallback
}

func (m Model) sessionLocationRecord() *domain.Record {
	if m.session == nil || m.session.LocationID == "" {
		return nil
	}
	for _, record := range m.operationalRecords() {
		if record.ID == m.session.LocationID {
			candidate := record
			return &candidate
		}
	}
	return nil
}

func (m Model) findLocation(query string) *domain.Record {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}
	var best *domain.Record
	bestScore := 0
	for _, record := range m.operationalRecords() {
		if record.Type != domain.Location || record.Authority == domain.Proposal {
			continue
		}
		title := strings.ToLower(record.Title)
		score := 0
		switch {
		case title == needle:
			score = 4
		case strings.HasPrefix(title, needle):
			score = 3
		case strings.Contains(title, needle):
			score = 2
		}
		for _, alias := range record.Aliases {
			if strings.EqualFold(alias, query) {
				score = 4
			}
		}
		if score > bestScore {
			candidate := record
			best = &candidate
			bestScore = score
		}
	}
	return best
}

func (m Model) sessionPresent() []domain.Record {
	if m.session == nil || len(m.session.Links) == 0 {
		return nil
	}
	present := make([]domain.Record, 0, len(m.session.Links))
	seen := map[string]bool{}
	records := m.operationalRecords()
	for _, link := range m.session.Links {
		if link.RecordID == "" || seen[link.RecordID] {
			continue
		}
		for _, record := range records {
			if record.ID != link.RecordID {
				continue
			}
			// Location already has CURRENT SCENE; keep PRESENT for the cast.
			if record.Type == domain.Location {
				break
			}
			present = append(present, record)
			seen[record.ID] = true
			break
		}
	}
	return present
}

func (m Model) sessionPlannedNotes() *domain.PlannedNotes {
	if m.session == nil || m.session.PlannedNotesID == "" {
		return nil
	}
	return m.plannedByID(m.session.PlannedNotesID)
}

// sessionThreads lists up to five active threads for live play: first the
// ones the current scene carries through its location and cast, then the most
// urgent of the rest. Resolved threads and AI proposals never appear.
func (m Model) sessionThreads() []domain.Record {
	statuses := m.sessionThreadStatuses()
	threads := make([]domain.Record, 0, len(statuses))
	for _, status := range statuses {
		threads = append(threads, status.Record)
	}
	return threads
}

func (m Model) sessionThreadStatuses() []domain.ThreadStatus {
	if m.session == nil {
		return nil
	}
	ids := []string{m.session.LocationID}
	for _, link := range m.session.Links {
		ids = append(ids, link.RecordID)
	}
	scene := domain.ThreadsTouching(m.workspace, m.workspace.Scope, ids)
	out := make([]domain.ThreadStatus, 0, 5)
	seen := map[string]bool{}
	for _, group := range [][]domain.ThreadStatus{scene, domain.CampaignThreads(m.workspace, m.workspace.Scope, false)} {
		for _, status := range group {
			if len(out) == 5 {
				return out
			}
			if seen[status.Record.ID] {
				continue
			}
			seen[status.Record.ID] = true
			out = append(out, status)
		}
	}
	return out
}

func (m Model) sessionContextItems() []domain.Record {
	items := make([]domain.Record, 0)
	if location := m.sessionLocationRecord(); location != nil {
		items = append(items, *location)
	}
	items = append(items, m.sessionPresent()...)
	items = append(items, m.sessionThreads()...)
	return items
}

func (m Model) countByType(entityType domain.EntityType) int {
	count := 0
	for _, record := range m.operationalRecords() {
		if record.Type == entityType && record.Authority != domain.Proposal {
			count++
		}
	}
	return count
}

func containsTag(record domain.Record, tag string) bool {
	for _, value := range record.Tags {
		if strings.EqualFold(value, tag) {
			return true
		}
	}
	return false
}

func (m Model) View() tea.View {
	if m.width == 0 {
		view := tea.NewView("Loading Dungeon…")
		view.AltScreen = true
		view.MouseMode = tea.MouseModeCellMotion
		return view
	}
	if m.capture.Open {
		base := m
		base.capture.Open = false
		result := base.View()
		result.Content = m.renderQuickCapture(result.Content)
		result.WindowTitle = "Dungeon · Quick capture"
		return result
	}
	if m.commands.Open {
		base := m
		base.commands.Open = false
		result := base.View()
		result.Content = m.renderCommandPalette(result.Content)
		result.WindowTitle = "Dungeon · Command palette"
		return result
	}
	if m.helping {
		result := tea.NewView(m.renderHelpOverlay())
		result.AltScreen = true
		result.MouseMode = tea.MouseModeCellMotion
		result.WindowTitle = "Dungeon · Help"
		return result
	}
	if m.importing {
		result := tea.NewView(m.renderImport())
		result.AltScreen = true
		result.MouseMode = tea.MouseModeCellMotion
		result.WindowTitle = "Dungeon · Import"
		return result
	}
	if m.picking {
		result := tea.NewView(m.renderPicker())
		result.AltScreen = true
		result.MouseMode = tea.MouseModeCellMotion
		result.WindowTitle = "Dungeon · Library"
		return result
	}
	if m.session != nil {
		result := m.sessionView()
		if m.searching {
			result.Content = m.renderSearchOverlay()
		} else if m.preview != nil {
			result.Content = m.renderPreviewOverlay(result.Content)
		}
		if m.settingsOpen {
			result.Content = m.renderSettingsOverlay(result.Content)
		}
		return result
	}

	contentWidth := max(1, m.width)
	view := ""
	if m.editing {
		view = m.renderEditorOverlay()
	} else {
		header := m.renderHeader(contentWidth)
		bodyHeight := max(1, m.height-2)
		body := m.renderBrowserTree(m.layout.Browser.Root, contentWidth, bodyHeight, 0, 1, nil)
		help := "/ search   n new   Enter open   : commands"
		if trail := m.browserTrailLabel(); trail != "" {
			help = trail + "   " + help
		}
		footer := footerStyle.
			Width(contentWidth).
			Render(footerWithStatus(m.status, help, contentWidth))
		content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
		view = appStyle.
			Width(contentWidth).
			Height(max(1, m.height)).
			MaxHeight(max(1, m.height)).
			Render(content)
	}

	if m.searching {
		view = m.renderSearchOverlay()
	} else if m.namingFolder {
		view = m.renderFolderNameOverlay()
	} else if m.namingCollection {
		view = m.renderCollectionNameOverlay()
	} else if m.planning {
		view = m.renderPlannedNotesOverlay()
	} else if m.playingBack {
		view = m.renderPlaybackOverlay()
	} else if m.reconciling {
		view = m.renderReconciliationOverlay()
	}
	if m.preview != nil {
		view = m.renderPreviewOverlay(view)
	}
	if m.settingsOpen {
		view = m.renderSettingsOverlay(view)
	}

	result := tea.NewView(view)
	result.AltScreen = true
	result.MouseMode = tea.MouseModeCellMotion
	result.WindowTitle = "Dungeon · " + m.workspace.Scope.Campaign
	return result
}

func (m Model) sessionView() tea.View {
	width := max(1, m.width)
	inputHeight := m.sessionInputHeight()
	transcriptHeight := m.sessionTranscriptHeight()
	upperHeight := m.sessionUpperHeight()
	leftWidth := m.splitWidth(width)
	rightWidth := max(1, width-leftWidth)
	header := headerStyle.Width(width).Render(m.renderSessionHeader())
	campaignOn := m.layout.SessionLeafVisible(prefs.PaneCampaign)
	contextOn := m.layout.SessionLeafVisible(prefs.PaneContext)
	var upper string
	switch {
	case campaignOn && contextOn:
		scene := m.panelStyleFor(prefs.PaneCampaign).Width(leftWidth).Height(upperHeight).MaxHeight(upperHeight).Render(fitPanelBody(m.renderCampaignPane(panelInnerWidth(leftWidth), panelInnerHeight(upperHeight)), panelInnerWidth(leftWidth), panelInnerHeight(upperHeight)))
		context := m.panelStyleFor(prefs.PaneContext).Width(rightWidth).Height(upperHeight).MaxHeight(upperHeight).Render(fitPanelBody(m.renderContextPane(), panelInnerWidth(rightWidth), panelInnerHeight(upperHeight)))
		upper = lipgloss.JoinHorizontal(lipgloss.Top, scene, context)
	case contextOn:
		upper = m.panelStyleFor(prefs.PaneContext).Width(width).Height(upperHeight).MaxHeight(upperHeight).Render(fitPanelBody(m.renderContextPane(), panelInnerWidth(width), panelInnerHeight(upperHeight)))
	default:
		upper = m.panelStyleFor(prefs.PaneCampaign).Width(width).Height(upperHeight).MaxHeight(upperHeight).Render(fitPanelBody(m.renderCampaignPane(panelInnerWidth(width), panelInnerHeight(upperHeight)), panelInnerWidth(width), panelInnerHeight(upperHeight)))
	}
	transcript := m.panelStyleFor(prefs.PaneTranscript).Width(width).Height(transcriptHeight).MaxHeight(transcriptHeight).Render(m.renderTranscript(transcriptHeight))
	input := m.panelStyleFor(prefs.PaneInput).Width(width).Height(inputHeight).MaxHeight(inputHeight).Render(fitPanelBody(m.renderSessionInput(), panelInnerWidth(width), panelInnerHeight(inputHeight)))
	help := "Enter capture   Ctrl+N note   / search   Tab pane   ? commands"
	footer := footerStyle.Width(width).Render(footerWithStatus(m.status, help, width))
	content := lipgloss.JoinVertical(lipgloss.Left, header, upper, transcript, input, footer)
	view := appStyle.Width(width).Height(max(1, m.height)).MaxHeight(max(1, m.height)).Render(content)
	if m.preview != nil {
		view = m.renderPreviewOverlay(view)
	}
	result := tea.NewView(view)
	result.AltScreen = true
	result.MouseMode = tea.MouseModeCellMotion
	result.WindowTitle = "Dungeon · " + m.workspace.Scope.Campaign + " · Session"
	return result
}

func (m Model) renderSessionHeader() string {
	number := len(m.workspace.Sessions) + 1
	if m.session != nil {
		number = len(m.workspace.Sessions) + 1
	}
	left := "─ " + m.workspace.Scope.Campaign + " ─"
	right := fmt.Sprintf("Session %d ─● LIVE─", number)
	inner := max(1, m.width-headerStyle.GetHorizontalFrameSize())
	gap := max(1, inner-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat("─", gap) + right
}

func (m Model) renderCampaignPane(width, height int) string {
	return renderContentLines(m.sessionCampaignContentLines(width, height))
}

func (m Model) renderContextPane() string {
	return renderContentLines(m.contextContentLines())
}

func wrapWords(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(current+" "+word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func (m Model) renderReview() string {
	if m.review == nil {
		return sectionStyle.Render("ENTITY REVIEW") + "\n\n" + mutedStyle.Render("Type @ followed by an entity name to open its details here.")
	}
	record := *m.review
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("ENTITY REVIEW"))
	builder.WriteString("  ")
	builder.WriteString(typeStyle.Render(string(record.Type)))
	builder.WriteString("  ")
	builder.WriteString(authorityStyle(record.Authority).Render(record.Authority.Marker() + " " + record.Authority.Label()))
	builder.WriteString("\n")
	builder.WriteString(detailTitleStyle.Render(record.Title))
	builder.WriteString("\n")
	builder.WriteString(m.renderMarkdown(record.Summary, m.defaultMarkdownWidth()))
	builder.WriteString("\n\n")
	builder.WriteString(m.renderMarkdown(stripRedundantTitleHeading(record.Body, record.Title), m.defaultMarkdownWidth()))
	builder.WriteString("\n")
	builder.WriteString(labelStyle.Render("SOURCE"))
	builder.WriteString(" " + record.Source)
	return builder.String()
}

func (m Model) renderSessionInput() string {
	var builder strings.Builder
	focusMark := ""
	if m.sessionFocus() == prefs.PaneInput {
		focusMark = " · focused"
	}
	builder.WriteString(sectionStyle.Render("INPUT") + mutedStyle.Render(focusMark))
	if strings.TrimSpace(m.sessionInput.Value()) == "" {
		builder.WriteString("  " + m.sessionInput.View())
	} else {
		builder.WriteString("  " + m.sessionInput.View())
	}
	if len(m.suggestions) > 0 {
		builder.WriteString("\n")
		builder.WriteString(mutedStyle.Render("@ / $ / # suggestions  click / ↑↓ / Tab insert"))
		for index, item := range m.suggestions {
			if index >= 5 {
				break
			}
			prefix := "  "
			style := searchResultStyle
			if index == m.suggestion {
				prefix = "▸ "
				style = selectedSearchResultStyle
			}
			builder.WriteString("\n")
			builder.WriteString(style.Render(prefix + item.Label))
		}
	}
	if peek := m.renderPeekPanel(5); peek != "" {
		builder.WriteString("\n")
		builder.WriteString(peek)
	}
	return builder.String()
}

func (m Model) renderTranscript(transcriptHeight int) string {
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("SESSION TRANSCRIPT"))
	builder.WriteString("\n")
	if m.session == nil || len(m.session.Entries) == 0 {
		builder.WriteString(mutedStyle.Render("No captured entries yet."))
	} else {
		viewport := m.transcriptView
		inner := panelInnerHeight(transcriptHeight)
		viewport.SetWidth(max(1, m.width-4))
		viewport.SetHeight(max(1, inner-1))
		builder.WriteString(viewport.View())
	}
	return fitLines(builder.String(), panelInnerHeight(transcriptHeight))
}

func (m Model) renderHeader(width int) string {
	world := mutedStyle.Render(m.workspace.Scope.WorldName)
	campaign := titleStyle.Render(m.workspace.Scope.Campaign)
	left := campaign + "  " + world
	right := authorityStyle(domain.Canon).Render("● FACTS") + "  " +
		authorityStyle(domain.Proposal).Render("△ AI SEPARATE")
	innerWidth := max(1, width-headerStyle.GetHorizontalFrameSize())
	gap := max(1, innerWidth-lipgloss.Width(left)-lipgloss.Width(right))
	return headerStyle.
		Width(width).
		Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) renderDetailTrail(section, folder, title string) string {
	folders := make([]string, 0, 4)
	if folder = domain.NormalizeFolder(folder); folder != "" {
		folders = strings.Split(folder, "/")
	}
	return m.renderDetailTrailParts(section, folders, title)
}

func (m Model) renderDetailTrailParts(section string, folders []string, title string) string {
	parts := make([]string, 0, len(folders)+3)
	if campaign := strings.TrimSpace(m.workspace.Scope.Campaign); campaign != "" {
		parts = append(parts, campaign)
	}
	if section = strings.TrimSpace(section); section != "" {
		parts = append(parts, section)
	}
	for _, folder := range folders {
		if folder = strings.TrimSpace(folder); folder != "" {
			parts = append(parts, folder)
		}
	}
	if title = strings.TrimSpace(title); title != "" {
		parts = append(parts, title)
	}
	if len(parts) == 0 {
		return ""
	}
	return mutedStyle.Render("PATH  " + strings.Join(parts, " / "))
}

func (m Model) searchResultContext(result searchsvc.Result) string {
	parts := make([]string, 0, 3)
	appendPart := func(value string) {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}

	switch result.Kind {
	case searchsvc.KindRecord, searchsvc.KindReference:
		record := result.Record
		if record.ID == "" {
			if found, ok := m.lookupAny(result.TargetID()); ok {
				record = found
			}
		}
		appendPart(domain.RecordFolderPath(record, m.workspace.Sources))
		appendPart(record.Source)
		if campaign := strings.TrimSpace(record.Scope.Campaign); campaign != "" && campaign != m.workspace.Scope.Campaign {
			appendPart(campaign)
		}
	case searchsvc.KindPrep:
		if plan := m.plannedByID(result.TargetID()); plan != nil {
			appendPart(plan.LocationName)
			appendPart(plan.SourceID)
		}
	case searchsvc.KindSession, searchsvc.KindTranscript:
		sessionID := result.SessionID
		if sessionID == "" {
			sessionID = result.TargetID()
		}
		if session := m.sessionByID(sessionID); session != nil {
			appendPart(domain.SessionFolderPath(*session))
			if !session.StartedAt.IsZero() {
				appendPart(session.StartedAt.Local().Format("2006-01-02"))
			}
			appendPart(session.LocationName)
		}
	}
	return strings.Join(parts, " · ")
}

func (m Model) renderDetail() string {
	return m.renderDetailWidth(m.defaultMarkdownWidth())
}

func (m Model) renderDetailWidth(width int) string {
	record := m.selectedRecord()
	if record == nil {
		return mutedStyle.Render("No records in this campaign.")
	}
	if width < 16 {
		width = m.defaultMarkdownWidth()
	}

	var builder strings.Builder
	builder.WriteString(m.renderDetailTrail(m.currentNav().Label, domain.RecordFolderPath(*record, m.workspace.Sources), record.Title))
	builder.WriteString("\n")
	builder.WriteString(typeStyle.Render(string(record.Type)))
	builder.WriteString("  ")
	builder.WriteString(authorityStyle(record.Authority).Render(record.Authority.Marker() + " " + record.Authority.Label()))
	if record.Type == domain.Thread && record.Authority != domain.Proposal && !record.IsAIContent {
		builder.WriteString(mutedStyle.Render("  · " + string(domain.EffectiveThreadState(*record))))
	}
	builder.WriteString("\n")
	builder.WriteString(detailTitleStyle.Render(record.Title))
	builder.WriteString("\n")
	// Authority notices sit directly under the title because they change how
	// the body below them should be read. Everything else about provenance
	// waits for the DETAILS block at the end.
	if notice := entityAuthorityNotice(*record); notice != "" {
		builder.WriteString(notice)
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	if record.Summary != "" {
		builder.WriteString(m.renderMarkdown(record.Summary, width))
		builder.WriteString("\n\n")
	}
	if record.Body != "" {
		builder.WriteString(m.renderMarkdown(stripRedundantTitleHeading(record.Body, record.Title), width))
		builder.WriteString("\n\n")
	} else if record.Summary == "" {
		builder.WriteString(mutedStyle.Render("No description yet · e edits this entity"))
		builder.WriteString("\n\n")
	}
	builder.WriteString(m.renderEntityBriefSections(*record))
	return builder.String()
}

// entityAuthorityNotice states an explicit AI, draft, or reference-only state.
func entityAuthorityNotice(record domain.Record) string {
	switch {
	case record.IsAIContent:
		return proposalWarningStyle.Render("AI-GENERATED · NOT FACTUAL · REQUIRES DM APPROVAL")
	case record.Authority == domain.Proposal:
		return proposalWarningStyle.Render("AI PROPOSAL · NOT CANON UNTIL YOU ACCEPT IT")
	case record.Authority == domain.Draft:
		return draftNoticeStyle.Render("DRAFT ENTITY · EDITABLE · NOT YET CANON")
	case record.Authority == domain.Reference:
		return mutedStyle.Render("IMPORTED REFERENCE · READ-ONLY SOURCE MATERIAL · NOT CAMPAIGN CANON")
	case record.Authority == domain.Superseded:
		return mutedStyle.Render("SUPERSEDED · KEPT FOR HISTORY · NOT CURRENT")
	default:
		return ""
	}
}

func entityScopeLabel(scope domain.Scope) string {
	if scope.WorldID != "" && scope.CampaignID == "" {
		return "WORLD SHARED · " + scope.WorldName
	}
	return "CAMPAIGN · " + scope.Campaign
}

func (m Model) renderSearchOverlay() string {
	width := min(88, max(52, m.width-10))
	visibleRows := min(10, max(4, m.height-12))

	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("SEARCH"))
	builder.WriteString("  scope: ")
	builder.WriteString(filterStyle.Render(m.searchScope.Label()))
	builder.WriteString("  data: ")
	dataLabel := "facts only"
	if m.includeIdeas {
		dataLabel = "facts + AI proposals"
	}
	builder.WriteString(filterStyle.Render(dataLabel))
	builder.WriteString("\n")
	builder.WriteString(m.searchInput.View())
	builder.WriteString("\n\n")

	if len(m.results) == 0 {
		note := strings.TrimSpace(m.rulesNote)
		if m.searchScope == searchsvc.RulesReference && note != "" {
			builder.WriteString(mutedStyle.Render(note))
		} else {
			builder.WriteString(mutedStyle.Render("No matching results in this scope."))
		}
	} else {
		start := 0
		if m.selected >= visibleRows {
			start = m.selected - visibleRows + 1
		}
		end := min(len(m.results), start+visibleRows)
		for index := start; index < end; index++ {
			result := m.results[index]
			cursor := "  "
			style := searchResultStyle
			if index == m.selected {
				cursor = "▸ "
				style = selectedSearchResultStyle
			}
			authority := result.Authority()
			line := fmt.Sprintf("%s%-9s %s  %-12s %s",
				cursor,
				result.TypeLabel(),
				authority.Marker(),
				authority.Label(),
				result.DisplayTitle(),
			)
			if context := m.searchResultContextFor(result); context != "" {
				line += "  · " + context
			}
			line = truncateImportLine(line, max(1, width-4))
			builder.WriteString(style.Render(line))
			builder.WriteRune('\n')
		}
	}

	builder.WriteString("\n")
	closing := "Esc close"
	if m.session != nil {
		closing = "Esc back to capture"
	}
	footer := "Ctrl+S scope  Ctrl+A include AI  ↑/↓ select  Enter open  " + closing
	if m.session != nil {
		footer = "Ctrl+S scope  Ctrl+A include AI  ↑/↓ select  Enter inspect  " + closing
	}
	if m.searchScope == searchsvc.RulesReference {
		footer = "Ctrl+S scope  Ctrl+G 5e path  ↑/↓ select  Enter inspect  " + closing
	}
	builder.WriteString(helpStyle.Render(footer))
	overlay := searchPanelStyle.
		Width(width).
		Render(builder.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m Model) renderCollectionNameOverlay() string {
	width := min(72, max(40, m.width-12))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("NEW COLLECTION"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("named group of wiki records"))
	builder.WriteString("\n")
	builder.WriteString(m.collectionName.View())
	builder.WriteString("\n\n")
	builder.WriteString(helpStyle.Render("Enter save  Esc cancel"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m Model) renderEditorOverlay() string {
	label := "EDIT ENTITY"
	if m.creating {
		label = "NEW DRAFT ENTITY"
	}
	width := max(1, m.width)
	height := max(1, m.height)
	suggest := renderSuggestionList(m.suggestions, m.suggestion)
	peek := m.renderPeekPanel(5)
	reserve := 0
	if suggest != "" {
		reserve += lipgloss.Height(suggest) + 1
	}
	if peek != "" {
		reserve += lipgloss.Height(peek) + 1
	}
	sizeMarkdownTextAreaReserved(&m.editBody, width, height, reserve)
	chrome := headerStyle.Width(width).Render(
		searchTitleStyle.Render(label) + "  " +
			mutedStyle.Render("markdown") + "  " +
			filterStyle.Render(string(m.editType)),
	)
	if m.status != "" && (strings.Contains(m.status, "needs") || strings.Contains(m.status, "Type →") || strings.Contains(strings.ToLower(m.status), "error") || strings.Contains(m.status, "required") || strings.Contains(m.status, "Title")) {
		chrome = lipgloss.JoinVertical(lipgloss.Left, chrome, footerStyle.Width(width).Render(m.status))
	}
	body := m.editBody.View()
	extras := make([]string, 0, 2)
	if suggest != "" {
		extras = append(extras, suggest)
	}
	if peek != "" {
		extras = append(extras, peek)
	}
	if len(extras) > 0 {
		body = lipgloss.JoinVertical(lipgloss.Left, append([]string{body, ""}, extras...)...)
	}
	help := markdownEditorHelp(m.editType)
	return renderFullScreenEditor(width, height, chrome, body, help)
}

func (m Model) renderReconciliationOverlay() string {
	width := min(88, max(32, m.width-4))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("POST-SESSION INBOX"))
	if len(m.workspace.Reconciliations) == 0 {
		builder.WriteString("\n\n")
		builder.WriteString(mutedStyle.Render("No reconciliation records yet."))
	} else {
		recon := m.workspace.Reconciliations[clamp(m.reconIndex, 0, len(m.workspace.Reconciliations)-1)]
		builder.WriteString("  ")
		builder.WriteString(filterStyle.Render(recon.Title))
		builder.WriteString("\n")
		unresolved := m.unresolvedReconciliationItems()
		builder.WriteString(mutedStyle.Render(fmt.Sprintf("%d remaining · transcript evidence stays immutable", len(unresolved))))
		builder.WriteString("\n\n")
		if len(unresolved) == 0 {
			builder.WriteString(filterStyle.Render("Inbox complete"))
			builder.WriteString("\n")
			builder.WriteString(mutedStyle.Render("Every extracted item has an owner decision."))
		} else {
			maxRows := max(1, m.reconciliationOverlayHeight()-10)
			start := 0
			for position, index := range unresolved {
				if index == m.reconCursor && position >= maxRows {
					start = position - maxRows + 1
				}
			}
			visible := unresolved[start:min(len(unresolved), start+maxRows)]
			for _, index := range visible {
				item := recon.Items[index]
				prefix := "  "
				style := searchResultStyle
				if index == m.reconCursor {
					prefix = "▸ "
					style = selectedSearchResultStyle
				}
				line := fmt.Sprintf("%s[%s] %s", prefix, item.Status, item.Summary)
				builder.WriteString(style.Render(line))
				builder.WriteString("\n")
			}
			item := recon.Items[clamp(m.reconCursor, 0, len(recon.Items)-1)]
			builder.WriteString("\n")
			builder.WriteString(labelStyle.Render("EVIDENCE"))
			builder.WriteString("\n")
			builder.WriteString(m.reconciliationProvenance(item))
			builder.WriteString("\n\n")
			builder.WriteString(labelStyle.Render("MUTATION"))
			builder.WriteString("  ")
			builder.WriteString(filterStyle.Render(string(item.Mutation.Op)))
			builder.WriteString("\n")
			if m.reconEditing {
				builder.WriteString(m.reconEdit.View())
			} else if strings.TrimSpace(item.Mutation.Text) != "" {
				builder.WriteString(item.Mutation.Text)
			} else {
				builder.WriteString(mutedStyle.Render("No proposed wiki write."))
			}
		}
		builder.WriteString(m.renderSessionCaptures(recon.SessionID, width))
	}
	builder.WriteString("\n")
	if m.reconEditing {
		builder.WriteString(helpStyle.Render("Ctrl+S save mutation  Esc cancel"))
	} else {
		builder.WriteString(helpStyle.Render("a accept    e edit    d defer    x reject    Esc close"))
	}
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func nextEntityType(current domain.EntityType) domain.EntityType {
	types := []domain.EntityType{domain.NPC, domain.Character, domain.Location, domain.Faction, domain.Item, domain.Creature, domain.Thread, domain.Session, domain.Scene, domain.Event, domain.Note, domain.Rule}
	for index, entityType := range types {
		if entityType == current {
			return types[(index+1)%len(types)]
		}
	}
	return types[0]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func (m Model) splitWidth(total int) int {
	minWidth := 24
	if m.session == nil && m.usesCampaignTree() {
		minWidth = 14
	}
	maxWidth := max(minWidth+1, total-24)
	ratio := m.layout.VerticalRatio()
	return clamp(int(float64(total)*ratio), minWidth, maxWidth)
}

func (m Model) sessionInputHeight() int {
	if len(m.suggestions) == 0 {
		return 7
	}
	// label + compact editor + hint + suggestion rows
	return clamp(4+1+min(5, len(m.suggestions)), 8, 12)
}

func (m Model) sessionTranscriptHeight() int {
	available := max(8, m.height-2)
	inputHeight := m.sessionInputHeight()
	work := max(8, available-inputHeight)
	upper := m.sessionUpperHeight()
	return max(5, work-upper)
}

func (m Model) sessionUpperHeight() int {
	available := max(8, m.height-2)
	inputHeight := m.sessionInputHeight()
	work := max(8, available-inputHeight)
	upper := int(float64(work) * m.layout.SessionUpperRatio())
	return clamp(upper, 6, max(7, work-5))
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// panelInnerHeight is the content rows available inside panelStyle
// (rounded border + vertical padding).
func panelInnerHeight(panelHeight int) int {
	const chrome = 4 // top/bottom border + top/bottom padding
	return max(1, panelHeight-chrome)
}

func fitLines(content string, maxLines int) string {
	visible, _ := windowLines(content, maxLines, 0)
	return visible
}

func fitPanelBody(content string, innerWidth, maxLines int) string {
	return fitPanelBodyScroll(content, innerWidth, maxLines, 0)
}

type paneScroll struct {
	key          string
	offset       int
	contentWidth int
	contentRaw   string
	content      string
	contentLines []string
	contentValid bool
}

func windowLines(content string, maxLines, scroll int) (string, int) {
	if maxLines <= 0 {
		return "", 0
	}
	return windowLinesFromLines(content, strings.Split(content, "\n"), maxLines, scroll)
}

func windowLinesFromLines(content string, lines []string, maxLines, scroll int) (string, int) {
	if maxLines <= 0 {
		return "", 0
	}
	maxScroll := max(0, len(lines)-maxLines)
	scroll = clamp(scroll, 0, maxScroll)
	if len(lines) <= maxLines {
		return content, 0
	}
	return strings.Join(lines[scroll:scroll+maxLines], "\n"), scroll
}

func fitPanelBodyScroll(content string, innerWidth, maxLines, scroll int) string {
	visible, _ := windowLines(clampANSIWidth(content, innerWidth), maxLines, scroll)
	return visible
}

func (m Model) detailSelectionKey() string {
	return strconv.Itoa(m.navCursor) + "\x1f" + string(m.navKind) + "\x1f" + string(m.navType) + "\x1f" + m.selectedID + "\x1f" + m.selectedSessionID + "\x1f" + m.selectedPlanID + "\x1f" + m.selectedFolderPath + "\x1f" + m.selectedSourceID + "\x1f" + m.selectedHitName + "\x1f" + m.selectedChapter
}

func (m *Model) ensureDetailView() *paneScroll {
	if m.detailView == nil {
		m.detailView = &paneScroll{}
	}
	key := m.detailSelectionKey()
	if m.detailView.key != key {
		m.detailView.key = key
		m.detailView.offset = 0
		m.detailView.contentValid = false
		m.detailView.contentRaw = ""
		m.detailView.content = ""
		m.detailView.contentLines = nil
	}
	return m.detailView
}

func (m Model) syncDetailView() *paneScroll {
	if m.detailView == nil {
		return &paneScroll{}
	}
	key := m.detailSelectionKey()
	if m.detailView.key != key {
		m.detailView.key = key
		m.detailView.offset = 0
		m.detailView.contentValid = false
		m.detailView.contentRaw = ""
		m.detailView.content = ""
		m.detailView.contentLines = nil
	}
	return m.detailView
}

func (m Model) detailPaneSize() (innerWidth, innerHeight int) {
	for _, region := range m.browserRegions() {
		if region.Pane != prefs.PaneDetail {
			continue
		}
		w := region.MaxX - region.MinX + 1
		h := region.MaxY - region.MinY + 1
		return panelInnerWidth(w), panelInnerHeight(h)
	}
	return max(16, m.defaultMarkdownWidth()), max(1, panelInnerHeight(max(8, m.height-2)))
}

func (m Model) detailBodyContent(innerWidth int) string {
	if m.usesCampaignTree() {
		return m.renderTreeDetailWidth(innerWidth)
	}
	return m.renderDetailWidth(innerWidth)
}

func (m Model) detailBodyView(innerWidth int) (string, []string) {
	view := m.syncDetailView()
	content := m.detailBodyContent(innerWidth)
	if view != nil && view.contentValid && view.contentWidth == innerWidth && view.contentRaw == content {
		return view.content, view.contentLines
	}
	clamped := clampANSIWidth(content, innerWidth)
	lines := strings.Split(clamped, "\n")
	if view != nil {
		view.contentWidth = innerWidth
		view.contentRaw = content
		view.content = clamped
		view.contentLines = lines
		view.contentValid = true
	}
	return clamped, lines
}

func (m Model) detailMaxScroll() int {
	innerWidth, innerHeight := m.detailPaneSize()
	_, lines := m.detailBodyView(innerWidth)
	return max(0, len(lines)-innerHeight)
}

func (m Model) detailPageStep() int {
	_, innerHeight := m.detailPaneSize()
	return max(1, innerHeight-1)
}

func (m *Model) scrollDetail(delta int) {
	view := m.ensureDetailView()
	view.offset = clamp(view.offset+delta, 0, m.detailMaxScroll())
}

func (m *Model) scrollDetailTo(offset int) {
	view := m.ensureDetailView()
	view.offset = clamp(offset, 0, m.detailMaxScroll())
}

func (m *Model) moveDetailFocus(delta int) {
	if len(m.detailHops()) > 0 {
		m.moveDetailCursor(delta)
		return
	}
	m.scrollDetail(delta)
}
