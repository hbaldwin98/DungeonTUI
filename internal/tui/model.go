package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/dice"
	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
	"github.com/hbaldwin98/dungeon/internal/storage"
)

type Model struct {
	workspace       domain.Workspace
	search          searchsvc.Service
	cursor          int
	width           int
	height          int
	searching       bool
	searchInput     textinput.Model
	searchScope     searchsvc.Scope
	includeIdeas    bool
	results         []searchsvc.Result
	selected        int
	typeFilter      domain.EntityType
	store           storage.Store
	prefs           prefs.Store
	status          string
	editing         bool
	creating        bool
	editIndex       int
	editField       int
	editType        domain.EntityType
	editTitle       textinput.Model
	editSummary     textinput.Model
	editBody        textarea.Model
	session         *domain.SessionRecord
	sessionInput    textarea.Model
	review          *domain.Record
	reviewPinned    bool
	suggestions     []domain.Record
	suggestion      int
	previousInput   string
	transcriptView  viewport.Model
	paneLayout      int
	paneSplit       int
	horizontalSplit int
	draggingSplit   bool
	dragAxis        string
	rollRNG         dice.RNG
}

func New() Model {
	return newModel(demoWorkspace(), nil, nil)
}

// NewPersistent loads the owner's workspace from the platform config
// directory. A first run starts with the small demo workspace and persists it
// on the first successful edit. Layout preferences load from a sibling file.
func NewPersistent() Model {
	path, err := storage.DefaultPath()
	if err != nil {
		return newModel(demoWorkspace(), nil, nil)
	}
	store := storage.NewJSON(path)
	prefStore := prefs.NewJSON(prefs.BesideWorkspace(path))
	workspace, err := store.Load()
	if err != nil {
		model := newModel(demoWorkspace(), store, prefStore)
		model.applyPreferences()
		model.status = "New workspace · edit or create an entity to save"
		return model
	}
	model := newModel(workspace, store, prefStore)
	model.applyPreferences()
	return model
}

func newModel(workspace domain.Workspace, store storage.Store, prefStore prefs.Store) Model {
	input := textinput.New()
	input.Placeholder = "Search titles, aliases, notes, and sources"
	input.Prompt = "> "
	input.CharLimit = 120

	model := Model{
		workspace:    workspace,
		store:        store,
		prefs:        prefStore,
		search:       searchsvc.New(workspace.Records),
		searchInput:  input,
		searchScope:  searchsvc.CurrentCampaign,
		includeIdeas: false,
	}
	model.sessionInput = textarea.New()
	model.sessionInput.Prompt = "│ "
	model.sessionInput.Placeholder = "Start a session to capture play…"
	model.sessionInput.SetHeight(5)
	model.transcriptView = viewport.New()
	model.transcriptView.SoftWrap = true
	model.transcriptView.MouseWheelEnabled = true
	model.paneLayout = 0
	model.rollRNG = dice.DefaultRNG()
	model.refreshResults()
	return model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.session != nil {
			m.configureTranscriptViewport()
		}
		return m, nil
	case tea.KeyPressMsg:
		if m.editing {
			return m.updateEditor(msg)
		}
		if m.session != nil {
			return m.updateSession(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.visibleRecords())-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "/":
			return m.openSearch()
		case "n":
			return m.openEditor(true)
		case "e":
			return m.openEditor(false)
		case "t":
			m.cycleTypeFilter()
		case "s":
			return m.startSession()
		}
	case tea.MouseClickMsg:
		if m.session != nil {
			return m.updateSessionMouseClick(msg)
		}
		return m.updateMouseClick(msg)
	case tea.MouseWheelMsg:
		if m.session != nil {
			var cmd tea.Cmd
			m.transcriptView, cmd = m.transcriptView.Update(msg)
			return m, cmd
		}
		return m.updateMouseWheel(msg)
	case tea.MouseMotionMsg:
		if m.draggingSplit {
			if m.dragAxis == "horizontal" {
				m.horizontalSplit = clamp(msg.Y-1, 6, max(7, m.height-m.sessionInputHeight()-7))
				m.configureTranscriptViewport()
			} else {
				m.paneSplit = clamp(msg.X, 24, max(25, m.width-24))
			}
		}
		return m, nil
	case tea.MouseReleaseMsg:
		wasDragging := m.draggingSplit
		m.draggingSplit = false
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
	session := domain.SessionRecord{
		ID:        fmt.Sprintf("session-%d", started.UnixNano()),
		Title:     "Session " + started.Format("2006-01-02 15:04"),
		Scope:     m.workspace.Scope,
		StartedAt: started,
	}
	if location := m.defaultSessionLocation(); location != nil {
		session.LocationID = location.ID
		session.LocationName = location.Title
	}
	m.session = &session
	m.sessionInput.SetValue("")
	m.sessionInput.SetWidth(max(30, m.width-8))
	m.sessionInput.SetHeight(5)
	m.sessionInput.Focus()
	m.configureTranscriptViewport()
	m.refreshTranscriptViewport()
	m.status = "Session started · Ctrl+E ends capture"
	return m, nil
}

func (m Model) updateSession(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	model := m
	if msg.String() == "ctrl+up" || msg.String() == "ctrl+down" || msg.String() == "pgup" || msg.String() == "pgdown" {
		var cmd tea.Cmd
		model.transcriptView, cmd = model.transcriptView.Update(msg)
		return model, cmd
	}
	if msg.Code == tea.KeyEnter && msg.Mod&tea.ModShift != 0 {
		previous := model.sessionInput.Value()
		model.sessionInput.SetValue(previous + "\n")
		model.previousInput = previous
		return model, nil
	}
	switch msg.String() {
	case "ctrl+e":
		return model.endSession()
	case "ctrl+p":
		model.paneLayout = (model.paneLayout + 1) % 3
		model.persistPreferences()
		return model, nil
	case "enter", "ctrl+enter", "\r", "\n":
		return model.submitTranscript()
	case "shift+enter":
		previous := model.sessionInput.Value()
		model.sessionInput.SetValue(previous + "\n")
		model.previousInput = previous
		return model, nil
	case "ctrl+z":
		if model.sessionInput.Value() != "" && model.previousInput != "" {
			model.sessionInput.SetValue(model.previousInput)
			model.refreshSuggestions()
			return model, nil
		}
		for index := len(model.session.Entries) - 1; index >= 0; index-- {
			if !model.session.Entries[index].Undone {
				model.session.Entries[index].Undone = true
				model.refreshTranscriptViewport()
				model.status = "Undid transcript entry · Ctrl+Y restores it"
				model.persistWorkspace()
				return model, nil
			}
		}
	case "ctrl+y":
		for index := len(model.session.Entries) - 1; index >= 0; index-- {
			if model.session.Entries[index].Undone {
				model.session.Entries[index].Undone = false
				model.refreshTranscriptViewport()
				model.status = "Restored transcript entry"
				model.persistWorkspace()
				return model, nil
			}
		}
	case "tab":
		if len(model.suggestions) > 0 {
			model.acceptSuggestion()
			return model, nil
		}
	case "up":
		if len(model.suggestions) > 0 {
			model.suggestion = clamp(model.suggestion-1, 0, len(model.suggestions)-1)
			return model, nil
		}
	case "down":
		if len(model.suggestions) > 0 {
			model.suggestion = clamp(model.suggestion+1, 0, len(model.suggestions)-1)
			return model, nil
		}
	case "esc":
		model.sessionInput.Blur()
		return model, nil
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

func (m *Model) refreshSuggestions() {
	m.suggestions = nil
	m.suggestion = 0
	value := m.sessionInput.Value()
	index := strings.LastIndex(value, "@")
	if index < 0 || (index > 0 && value[index-1] != ' ' && value[index-1] != '\n') {
		m.configureTranscriptViewport()
		return
	}
	query := strings.ToLower(strings.TrimSpace(value[index+1:]))
	if query == "" {
		m.configureTranscriptViewport()
		return
	}
	for _, record := range m.campaignRecords() {
		if strings.Contains(strings.ToLower(record.Title), query) {
			m.suggestions = append(m.suggestions, record)
		}
		if len(m.suggestions) == 5 {
			break
		}
	}
	m.configureTranscriptViewport()
}

func (m *Model) acceptSuggestion() {
	if len(m.suggestions) == 0 {
		return
	}
	value := m.sessionInput.Value()
	index := strings.LastIndex(value, "@")
	if index < 0 {
		return
	}
	record := m.suggestions[clamp(m.suggestion, 0, len(m.suggestions)-1)]
	m.sessionInput.SetValue(value[:index] + "@" + record.Title + " ")
	m.review = &record
	m.suggestions = nil
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

func (m *Model) handleSessionCommand(text string) {
	if strings.HasPrefix(text, "$") {
		if record, ok := parseEntityCommand(text, m.workspace.Scope, m.session); ok {
			m.workspace.Records = append(m.workspace.Records, record)
			m.search = searchsvc.New(m.workspace.Records)
			m.refreshResults()
			m.review = &record
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
		m.search = searchsvc.New(m.workspace.Records)
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
	m.session.EndedAt = &ended
	m.sessionInput.Blur()
	m.status = fmt.Sprintf("Session ended · %d transcript entries", len(m.session.Entries))
	// Always retain the ended session in the workspace, even when no store is
	// attached (demo / harness). Persistence remains best-effort afterward.
	updated := false
	for index := range m.workspace.Sessions {
		if m.workspace.Sessions[index].ID == m.session.ID {
			m.workspace.Sessions[index] = *m.session
			updated = true
			break
		}
	}
	if !updated {
		m.workspace.Sessions = append(m.workspace.Sessions, *m.session)
	}
	if m.store != nil {
		m.persistWorkspace()
	}
	m.session = nil
	return m, nil
}

func (m *Model) persistWorkspace() {
	if m.store == nil {
		return
	}
	if m.session != nil {
		updated := false
		for index := range m.workspace.Sessions {
			if m.workspace.Sessions[index].ID == m.session.ID {
				m.workspace.Sessions[index] = *m.session
				updated = true
				break
			}
		}
		if !updated {
			m.workspace.Sessions = append(m.workspace.Sessions, *m.session)
		}
	}
	if err := m.store.Save(m.workspace); err != nil {
		m.status = "Saved in memory; persistence failed: " + err.Error()
	}
}

func (m *Model) applyPreferences() {
	if m.prefs == nil {
		return
	}
	layout, err := m.prefs.Load()
	if err != nil {
		return
	}
	m.paneLayout = clamp(layout.PaneLayout, 0, 2)
	m.paneSplit = layout.PaneSplit
	m.horizontalSplit = layout.HorizontalSplit
}

func (m *Model) persistPreferences() {
	if m.prefs == nil {
		return
	}
	err := m.prefs.Save(prefs.Layout{
		PaneLayout:      m.paneLayout,
		PaneSplit:       m.paneSplit,
		HorizontalSplit: m.horizontalSplit,
	})
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
	for _, record := range m.campaignRecords() {
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
	return best
}

func (m Model) resolveLinks(text string) []domain.EntityLink {
	var links []domain.EntityLink
	for index := strings.Index(text, "@"); index >= 0; {
		remaining := text[index+1:]
		if remaining == "" {
			break
		}
		for end := len(remaining); end > 0; end-- {
			if record := m.resolveReference("@" + remaining[:end]); record != nil {
				links = append(links, domain.EntityLink{Text: remaining[:end], RecordID: record.ID})
				break
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
		return m, nil
	case "ctrl+s":
		m.searchScope = (m.searchScope + 1) % 3
		m.selected = 0
		m.refreshResults()
		return m, nil
	case "ctrl+a":
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
			selectedID := m.results[m.selected].Record.ID
			for index, record := range m.visibleRecords() {
				if record.ID == selectedID {
					m.cursor = index
					break
				}
			}
		}
		m.searching = false
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.selected = 0
	m.refreshResults()
	return m, cmd
}

func (m Model) openSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.selected = 0
	m.searchInput.Focus()
	return m, textinput.Blink
}

func (m Model) updateMouseClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}

	if m.searching {
		return m.updateSearchClick(msg)
	}

	if msg.Y == m.height-1 && msg.X <= 10 {
		return m.openSearch()
	}

	listWidth := m.splitWidth(m.width)
	if abs(msg.X-listWidth) <= 1 {
		m.draggingSplit = true
		m.dragAxis = "vertical"
		return m, nil
	}
	if msg.X >= 0 && msg.X < listWidth {
		const firstRecordRow = 5
		index := msg.Y - firstRecordRow
		if index >= 0 && index < len(m.visibleRecords()) {
			m.cursor = index
		}
	}
	return m, nil
}

func (m *Model) cycleTypeFilter() {
	types := []domain.EntityType{"", domain.NPC, domain.Character, domain.Location, domain.Faction, domain.Item, domain.Thread, domain.Session, domain.Event, domain.Note, domain.Rule}
	for index, entityType := range types {
		if entityType == m.typeFilter {
			m.typeFilter = types[(index+1)%len(types)]
			m.cursor = 0
			return
		}
	}
}

func (m Model) openEditor(create bool) (tea.Model, tea.Cmd) {
	title := textinput.New()
	title.Prompt = "Title: "
	title.CharLimit = 160
	summary := textinput.New()
	summary.Prompt = "Summary: "
	summary.CharLimit = 240
	body := textarea.New()
	body.Prompt = "Body: "
	body.SetWidth(max(30, m.width-12))
	body.SetHeight(6)
	model := m
	model.editing = true
	model.creating = create
	model.editIndex = m.cursor
	model.editField = 0
	model.editType = domain.NPC
	model.editTitle = title
	model.editSummary = summary
	model.editBody = body
	if create {
		model.editTitle.SetValue("New NPC")
		model.editBody.SetValue("")
	} else {
		records := m.visibleRecords()
		if len(records) == 0 || m.cursor >= len(records) {
			return m, nil
		}
		record := records[m.cursor]
		model.editType = record.Type
		model.editTitle.SetValue(record.Title)
		model.editSummary.SetValue(record.Summary)
		model.editBody.SetValue(record.Body)
	}
	return model, model.focusEditor()
}

func (m *Model) focusEditor() tea.Cmd {
	m.editTitle.Blur()
	m.editSummary.Blur()
	m.editBody.Blur()
	switch m.editField {
	case 0:
		return m.editTitle.Focus()
	case 1:
		return m.editSummary.Focus()
	default:
		return m.editBody.Focus()
	}
}

func (m Model) updateEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	model := m
	switch msg.String() {
	case "esc":
		model.editing = false
		return model, nil
	case "tab", "shift+tab":
		if msg.String() == "tab" {
			model.editField = (model.editField + 1) % 3
		} else {
			model.editField = (model.editField + 2) % 3
		}
		return model, model.focusEditor()
	case "ctrl+t":
		model.editType = nextEntityType(model.editType)
		return model, nil
	case "ctrl+s", "ctrl+enter":
		return model.saveEditor()
	}
	var cmd tea.Cmd
	switch model.editField {
	case 0:
		model.editTitle, cmd = model.editTitle.Update(msg)
	case 1:
		model.editSummary, cmd = model.editSummary.Update(msg)
	default:
		model.editBody, cmd = model.editBody.Update(msg)
	}
	return model, cmd
}

func (m Model) saveEditor() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.editTitle.Value())
	if title == "" {
		m.status = "Title is required"
		return m, nil
	}
	records := m.visibleRecords()
	scope := m.workspace.Scope
	record := domain.Record{
		ID:        fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(title, " ", "-")), time.Now().UnixNano()),
		Type:      m.editType,
		Title:     title,
		Summary:   strings.TrimSpace(m.editSummary.Value()),
		Body:      m.editBody.Value(),
		Authority: domain.Draft,
		Scope:     scope,
		Source:    "DM draft",
	}
	if !m.creating {
		if m.editIndex < 0 || m.editIndex >= len(records) {
			m.status = "Entity no longer exists"
			m.editing = false
			return m, nil
		}
		record = records[m.editIndex]
		record.Title = title
		record.Summary = strings.TrimSpace(m.editSummary.Value())
		record.Body = m.editBody.Value()
	}
	if err := record.Validate(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	if m.creating {
		m.workspace.Records = append(m.workspace.Records, record)
		m.cursor = max(0, len(m.visibleRecords())-1)
	} else {
		for index := range m.workspace.Records {
			if m.workspace.Records[index].ID == record.ID {
				m.workspace.Records[index] = record
				break
			}
		}
	}
	m.search = searchsvc.New(m.workspace.Records)
	m.refreshResults()
	m.editing = false
	m.status = "Saved draft: " + record.Title
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
		selectedID := m.results[m.selected].Record.ID
		for recordIndex, record := range m.visibleRecords() {
			if record.ID == selectedID {
				m.cursor = recordIndex
				m.searching = false
				m.searchInput.Blur()
				break
			}
		}
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

	if m.searching {
		m.selected = clamp(m.selected+delta, 0, max(0, len(m.results)-1))
	} else {
		m.cursor = clamp(m.cursor+delta, 0, max(0, len(m.visibleRecords())-1))
	}
	return m, nil
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
		m.sessionInput.Focus()
	}
	return m, nil
}

func (m Model) applyHit(hit hitTarget) (tea.Model, tea.Cmd) {
	switch hit.Action {
	case hitSelectRecord:
		if hit.Record != nil {
			record := *hit.Record
			m.review = &record
			m.status = "Reviewing " + record.Title
		}
	case hitAcceptSuggestion:
		m.suggestion = hit.Suggestion
		m.acceptSuggestion()
		m.status = "Inserted @" + m.suggestionsTitle(hit)
	case hitPinReview:
		if m.review != nil {
			m.reviewPinned = !m.reviewPinned
			if m.reviewPinned {
				m.status = "Pinned review: " + m.review.Title
			} else {
				m.status = "Unpinned review"
			}
		}
	case hitClearReview:
		m.review = nil
		m.reviewPinned = false
		m.status = "Cleared entity review"
	case hitCampaignType:
		for _, record := range m.campaignRecords() {
			if record.Type == hit.EntityType && record.Authority != domain.Proposal {
				candidate := record
				m.review = &candidate
				m.status = "Reviewing " + record.Title
				break
			}
		}
	case hitFocusInput:
		m.sessionInput.Focus()
	}
	return m, nil
}

func (m Model) suggestionsTitle(hit hitTarget) string {
	if hit.Record != nil {
		return hit.Record.Title
	}
	if hit.Suggestion >= 0 && hit.Suggestion < len(m.suggestions) {
		return m.suggestions[hit.Suggestion].Title
	}
	return "entity"
}

func (m *Model) refreshResults() {
	m.results = m.search.Find(searchsvc.Filter{
		Query:            m.searchInput.Value(),
		Scope:            m.searchScope,
		WorldID:          m.workspace.Scope.WorldID,
		CampaignID:       m.workspace.Scope.CampaignID,
		IncludeProposals: m.includeIdeas,
	})
}

func (m Model) visibleRecords() []domain.Record {
	return m.scopedRecords(true)
}

func (m Model) campaignRecords() []domain.Record {
	return m.scopedRecords(false)
}

func (m Model) scopedRecords(applyTypeFilter bool) []domain.Record {
	records := make([]domain.Record, 0, len(m.workspace.Records))
	for _, record := range m.workspace.Records {
		if record.Scope.CampaignID == m.workspace.Scope.CampaignID ||
			(record.Scope.CampaignID == "" && record.Scope.WorldID == m.workspace.Scope.WorldID) {
			if applyTypeFilter && m.typeFilter != "" && record.Type != m.typeFilter {
				continue
			}
			records = append(records, record)
		}
	}
	return records
}

func (m Model) defaultSessionLocation() *domain.Record {
	var fallback *domain.Record
	for _, record := range m.campaignRecords() {
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
	for _, record := range m.campaignRecords() {
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
	for _, record := range m.campaignRecords() {
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
	present := make([]domain.Record, 0)
	for _, record := range m.campaignRecords() {
		if record.Authority == domain.Proposal {
			continue
		}
		if record.Type == domain.NPC || record.Type == domain.Character {
			present = append(present, record)
		}
		if len(present) == 6 {
			break
		}
	}
	return present
}

func (m Model) sessionThreads() []domain.Record {
	threads := make([]domain.Record, 0)
	for _, record := range m.campaignRecords() {
		if record.Type != domain.Thread || record.Authority == domain.Proposal {
			continue
		}
		threads = append(threads, record)
		if len(threads) == 5 {
			break
		}
	}
	return threads
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
	for _, record := range m.campaignRecords() {
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
	if m.session != nil {
		return m.sessionView()
	}

	contentWidth := max(1, m.width)
	header := m.renderHeader(contentWidth)
	bodyHeight := max(1, m.height-2)
	listWidth := m.splitWidth(contentWidth)
	detailWidth := max(1, contentWidth-listWidth)

	list := panelStyle.
		Width(listWidth).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(m.renderList())
	detail := panelStyle.
		Width(detailWidth).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(m.renderDetail())
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
	help := "/ search  n new draft  e edit  t filter type  s start session  drag gutters  q quit"
	if m.status != "" {
		help = m.status + "  ·  " + help
	}
	footer := footerStyle.
		Width(contentWidth).
		Render(help)
	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	view := appStyle.
		Width(contentWidth).
		Height(max(1, m.height)).
		MaxHeight(max(1, m.height)).
		Render(content)

	if m.searching {
		view = m.renderSearchOverlay()
	} else if m.editing {
		view = m.renderEditorOverlay()
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
	scene := panelStyle.Width(leftWidth).Height(upperHeight).MaxHeight(upperHeight).Render(fitLines(m.renderCampaignPane(), panelInnerHeight(upperHeight)))
	context := panelStyle.Width(rightWidth).Height(upperHeight).MaxHeight(upperHeight).Render(fitLines(m.renderContextPane(), panelInnerHeight(upperHeight)))
	upper := lipgloss.JoinHorizontal(lipgloss.Top, scene, context)
	if m.paneLayout == 1 {
		upper = panelStyle.Width(width).Height(upperHeight).MaxHeight(upperHeight).Render(fitLines(m.renderContextPane(), panelInnerHeight(upperHeight)))
	} else if m.paneLayout == 2 {
		upper = panelStyle.Width(width).Height(upperHeight).MaxHeight(upperHeight).Render(fitLines(m.renderCampaignPane(), panelInnerHeight(upperHeight)))
	}
	transcript := panelStyle.Width(width).Height(transcriptHeight).MaxHeight(transcriptHeight).Render(m.renderTranscript(transcriptHeight))
	input := panelStyle.Width(width).Height(inputHeight).MaxHeight(inputHeight).Render(fitLines(m.renderSessionInput(), panelInnerHeight(inputHeight)))
	help := "Enter capture   click entities/suggestions   [pin]/[clear] review   Ctrl+P panes   wheel scroll   Ctrl+E end"
	footer := footerStyle.Width(width).Render(help)
	content := lipgloss.JoinVertical(lipgloss.Left, header, upper, transcript, input, footer)
	view := appStyle.Width(width).Height(max(1, m.height)).MaxHeight(max(1, m.height)).Render(content)
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

func (m Model) renderCampaignPane() string {
	return renderContentLines(m.campaignContentLines())
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
	builder.WriteString(record.Summary)
	builder.WriteString("\n\n")
	builder.WriteString(record.Body)
	builder.WriteString("\n")
	builder.WriteString(labelStyle.Render("SOURCE"))
	builder.WriteString(" " + record.Source)
	return builder.String()
}

func (m Model) renderSessionInput() string {
	if strings.TrimSpace(m.sessionInput.Value()) == "" && len(m.suggestions) == 0 {
		return sectionStyle.Render("INPUT") + "  " + mutedStyle.Render("> Type a transcript entry, $entity command, or #command; press Enter to submit")
	}
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("INPUT") + "  " + m.sessionInput.View())
	if len(m.suggestions) > 0 {
		builder.WriteString("\n")
		builder.WriteString(mutedStyle.Render("@ suggestions  click / ↑↓ / Tab insert"))
		for index, record := range m.suggestions {
			if index >= 3 {
				break
			}
			prefix := "  "
			style := searchResultStyle
			if index == m.suggestion {
				prefix = "▸ "
				style = selectedSearchResultStyle
			}
			builder.WriteString("\n")
			builder.WriteString(style.Render(prefix + string(record.Type) + "  " + record.Title))
		}
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

func (m Model) renderList() string {
	var builder strings.Builder
	section := "CAMPAIGN RECORDS"
	if m.typeFilter != "" {
		section = string(m.typeFilter) + " RECORDS"
	}
	builder.WriteString(sectionStyle.Render(section))
	builder.WriteString("\n\n")

	for index, record := range m.visibleRecords() {
		cursor := "  "
		style := normalItemStyle
		if index == m.cursor {
			cursor = "▸ "
			style = selectedItemStyle
		}
		line := fmt.Sprintf("%s%s %-8s %s", cursor, record.Authority.Marker(), record.Type, record.Title)
		builder.WriteString(style.Render(line))
		builder.WriteRune('\n')
	}

	return builder.String()
}

func (m Model) renderDetail() string {
	records := m.visibleRecords()
	if len(records) == 0 {
		return mutedStyle.Render("No records in this campaign.")
	}
	if m.cursor >= len(records) {
		m.cursor = len(records) - 1
	}
	record := records[m.cursor]

	var builder strings.Builder
	builder.WriteString(typeStyle.Render(string(record.Type)))
	builder.WriteString("  ")
	builder.WriteString(authorityStyle(record.Authority).Render(record.Authority.Marker() + " " + record.Authority.Label()))
	builder.WriteString("\n")
	builder.WriteString(detailTitleStyle.Render(record.Title))
	builder.WriteString("\n\n")
	builder.WriteString(record.Summary)
	builder.WriteString("\n\n")
	builder.WriteString(record.Body)
	builder.WriteString("\n\n")
	builder.WriteString(labelStyle.Render("SCOPE"))
	builder.WriteString("  " + record.Scope.Label() + "\n")
	builder.WriteString(labelStyle.Render("SOURCE"))
	builder.WriteString(" " + record.Source + "\n")

	if record.IsAIContent {
		builder.WriteString("\n")
		builder.WriteString(proposalWarningStyle.Render("AI-GENERATED · NOT FACTUAL · REQUIRES DM APPROVAL"))
	}

	if record.Authority == domain.Draft {
		builder.WriteString("\n")
		builder.WriteString(draftNoticeStyle.Render("DRAFT ENTITY · EDITABLE · NOT YET CANON"))
	}
	return builder.String()
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
		builder.WriteString(mutedStyle.Render("No matching records in this scope."))
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
			line := fmt.Sprintf("%s%-9s %s  %-12s %s",
				cursor,
				result.Record.Type,
				result.Record.Authority.Marker(),
				result.Record.Authority.Label(),
				result.Record.Title,
			)
			builder.WriteString(style.Render(line))
			builder.WriteRune('\n')
		}
	}

	builder.WriteString("\n")
	builder.WriteString(helpStyle.Render("Ctrl+S scope  Ctrl+A include AI  ↑/↓ select  Enter open  Esc close"))
	overlay := searchPanelStyle.
		Width(width).
		Render(builder.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
	)
}

func (m Model) renderEditorOverlay() string {
	width := min(88, max(52, m.width-10))
	label := "EDIT ENTITY"
	if m.creating {
		label = "NEW DRAFT ENTITY"
	}
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render(label))
	builder.WriteString("  type: ")
	builder.WriteString(filterStyle.Render(string(m.editType)))
	builder.WriteString("\n\n")
	builder.WriteString(m.editTitle.View())
	builder.WriteString("\n")
	builder.WriteString(m.editSummary.View())
	builder.WriteString("\n")
	builder.WriteString(m.editBody.View())
	builder.WriteString("\n")
	builder.WriteString(helpStyle.Render("Tab/Shift+Tab next field  Ctrl+S save draft  Esc cancel"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
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
	if m.paneSplit > 0 {
		return clamp(m.paneSplit, 24, max(25, total-24))
	}
	return clamp(total/3, 28, max(29, total-28))
}

func (m Model) sessionInputHeight() int {
	if len(m.suggestions) > 0 {
		return 9
	}
	return 7
}

func (m Model) sessionTranscriptHeight() int {
	available := max(8, m.height-2)
	if m.horizontalSplit > 0 {
		return max(5, available-m.sessionInputHeight()-m.sessionUpperHeight())
	}
	return min(8, max(6, available/4))
}

func (m Model) sessionUpperHeight() int {
	available := max(8, m.height-2)
	if m.horizontalSplit > 0 {
		return clamp(m.horizontalSplit, 6, max(7, available-m.sessionInputHeight()-5))
	}
	return max(6, available-m.sessionInputHeight()-min(8, max(6, available/4)))
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
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}
