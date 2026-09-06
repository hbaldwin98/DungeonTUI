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

	"github.com/hbaldwin98/dungeon/internal/domain"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
	"github.com/hbaldwin98/dungeon/internal/storage"
)

type Model struct {
	workspace      domain.Workspace
	search         searchsvc.Service
	cursor         int
	width          int
	height         int
	searching      bool
	searchInput    textinput.Model
	searchScope    searchsvc.Scope
	includeIdeas   bool
	results        []searchsvc.Result
	selected       int
	typeFilter     domain.EntityType
	store          storage.Store
	status         string
	editing        bool
	creating       bool
	editIndex      int
	editField      int
	editType       domain.EntityType
	editTitle      textinput.Model
	editSummary    textinput.Model
	editBody       textarea.Model
	session        *domain.SessionRecord
	sessionInput   textarea.Model
	review         *domain.Record
	reviewPinned   bool
	suggestions    []domain.Record
	suggestion     int
	previousInput  string
	transcriptView viewport.Model
	paneLayout     int
}

func New() Model {
	return newModel(demoWorkspace(), nil)
}

// NewPersistent loads the owner's workspace from the platform config
// directory. A first run starts with the small demo workspace and persists it
// on the first successful edit.
func NewPersistent() Model {
	path, err := storage.DefaultPath()
	if err != nil {
		return newModel(demoWorkspace(), nil)
	}
	store := storage.NewJSON(path)
	workspace, err := store.Load()
	if err != nil {
		model := newModel(demoWorkspace(), store)
		model.status = "New workspace · edit or create an entity to save"
		return model
	}
	return newModel(workspace, store)
}

func newModel(workspace domain.Workspace, store storage.Store) Model {
	input := textinput.New()
	input.Placeholder = "Search titles, aliases, notes, and sources"
	input.Prompt = "> "
	input.CharLimit = 120

	model := Model{
		workspace:    workspace,
		store:        store,
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
	for _, record := range m.visibleRecords() {
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
	m.handleSessionCommand(text)
	now := time.Now().UTC()
	m.session.Entries = append(m.session.Entries, domain.TranscriptEntry{
		ID:        fmt.Sprintf("entry-%d", now.UnixNano()),
		Text:      text,
		CreatedAt: now,
		Links:     links,
		Revision:  1,
	})
	m.sessionInput.SetValue("")
	m.refreshTranscriptViewport()
	m.status = "Captured transcript entry"
	if m.store != nil {
		m.persistWorkspace()
	}
	return m, nil
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
	}
	m.transcriptView.SetContent(builder.String())
	m.transcriptView.GotoBottom()
}

func (m *Model) configureTranscriptViewport() {
	available := max(8, m.height-2)
	transcriptHeight := min(7, max(5, available/4))
	m.transcriptView.SetWidth(max(1, m.width-4))
	m.transcriptView.SetHeight(max(1, transcriptHeight-2))
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
	if strings.HasPrefix(text, "#location ") {
		m.status = "Current location: " + strings.TrimSpace(strings.TrimPrefix(text, "#location "))
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
	return domain.Record{
		ID:        fmt.Sprintf("generated-%d", now.UnixNano()),
		Type:      entityType,
		Title:     title,
		Summary:   "A context-aware local generator draft.",
		Body:      "Generated during the active session. Review and edit before treating this as campaign truth.",
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
	for _, record := range m.visibleRecords() {
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

	listWidth := min(40, max(28, m.width/3))
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
	// The lower portion is always the transcript/input region. Clicking there
	// returns focus to capture, while clicking the context pane selects the
	// corresponding entity for review.
	available := max(8, m.height-2)
	inputHeight := 7
	if len(m.suggestions) > 0 {
		inputHeight = 9
	}
	transcriptHeight := min(7, max(5, available/4))
	upperHeight := max(6, available-inputHeight-transcriptHeight)
	if msg.Y > upperHeight+1 {
		m.sessionInput.Focus()
		return m, nil
	}
	if m.paneLayout == 2 || msg.X < m.width/2 {
		return m, nil
	}
	threads := make([]domain.Record, 0)
	for _, record := range m.visibleRecords() {
		if record.Type == domain.Thread || record.Type == domain.NPC || record.Type == domain.Character {
			threads = append(threads, record)
		}
	}
	index := msg.Y - 3
	if index >= 0 && index < len(threads) {
		record := threads[index]
		m.review = &record
	}
	return m, nil
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
	records := make([]domain.Record, 0, len(m.workspace.Records))
	for _, record := range m.workspace.Records {
		if record.Scope.CampaignID == m.workspace.Scope.CampaignID ||
			(record.Scope.CampaignID == "" && record.Scope.WorldID == m.workspace.Scope.WorldID) {
			if m.typeFilter != "" && record.Type != m.typeFilter {
				continue
			}
			records = append(records, record)
		}
	}
	return records
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

	contentWidth := max(60, m.width)
	header := m.renderHeader(contentWidth)
	bodyHeight := max(10, m.height-2)
	listWidth := min(40, max(28, contentWidth/3))
	detailWidth := max(30, contentWidth-listWidth)

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
	help := "/ search  n new draft  e edit  t filter type  s start session  mouse: click/scroll  q quit"
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
	width := max(60, m.width)
	available := max(8, m.height-2)
	inputHeight := 7
	if len(m.suggestions) > 0 {
		inputHeight = 9
	}
	transcriptHeight := min(7, max(5, available/4))
	upperHeight := max(6, available-inputHeight-transcriptHeight)
	leftWidth := max(28, width/2)
	rightWidth := max(28, width-leftWidth)
	header := headerStyle.Width(width).Render(m.renderSessionHeader())
	scene := panelStyle.Width(leftWidth).Height(upperHeight).MaxHeight(upperHeight).Render(m.renderCampaignPane())
	context := panelStyle.Width(rightWidth).Height(upperHeight).MaxHeight(upperHeight).Render(m.renderContextPane())
	upper := lipgloss.JoinHorizontal(lipgloss.Top, scene, context)
	if m.paneLayout == 1 {
		upper = panelStyle.Width(width).Height(upperHeight).MaxHeight(upperHeight).Render(m.renderContextPane())
	} else if m.paneLayout == 2 {
		upper = panelStyle.Width(width).Height(upperHeight).MaxHeight(upperHeight).Render(m.renderCampaignPane())
	}
	transcript := panelStyle.Width(width).Height(transcriptHeight).MaxHeight(transcriptHeight).Render(m.renderTranscript(inputHeight))
	input := panelStyle.Width(width).Height(inputHeight).MaxHeight(inputHeight).Render(m.renderSessionInput())
	help := "Enter/Ctrl+Enter capture   Shift+Enter newline   Ctrl+P panes   wheel scroll log   Ctrl+E end"
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
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("CAMPAIGN"))
	builder.WriteString("\n\n")
	sections := []string{"Session", "World", "NPCs", "Locations", "Factions", "Threads", "Timeline", "Players", "Secrets"}
	for index, section := range sections {
		marker := "  "
		if index == 0 {
			marker = "▸ "
		}
		count := ""
		switch section {
		case "NPCs":
			count = "42"
		case "Locations":
			count = "18"
		case "Factions":
			count = "7"
		case "Threads":
			count = "11"
		}
		builder.WriteString(marker + section)
		if count != "" {
			builder.WriteString(strings.Repeat(" ", max(1, 14-len(section)-len(count))) + count)
		}
		builder.WriteRune('\n')
	}
	return builder.String()
}

func (m Model) renderContextPane() string {
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("CURRENT SCENE"))
	builder.WriteString("\n\n")
	builder.WriteString(detailTitleStyle.Render("Greywatch Monastery"))
	builder.WriteString("\n")
	builder.WriteString("Party entered through the collapsed\n")
	builder.WriteString("eastern transept.\n\n")
	builder.WriteString(labelStyle.Render("PRESENT"))
	builder.WriteString("\n")
	builder.WriteString("◆ Captain Vale\n◆ Father Merrow\n\n")
	builder.WriteString(labelStyle.Render("ACTIVE THREADS"))
	builder.WriteString("\n")
	groups := []struct {
		label string
		types []domain.EntityType
	}{
		{label: "", types: []domain.EntityType{domain.Thread}},
	}
	for _, group := range groups {
		if group.label != "" {
			builder.WriteString(labelStyle.Render(group.label))
			builder.WriteString("\n")
		}
		count := 0
		for _, record := range m.visibleRecords() {
			if !containsType(group.types, record.Type) {
				continue
			}
			marker := "  "
			if m.review != nil && m.review.ID == record.ID {
				marker = "▸ "
			}
			builder.WriteString(marker + record.Title + "\n")
			count++
			if count == 4 {
				break
			}
		}
		if count == 0 {
			builder.WriteString(mutedStyle.Render("  —") + "\n")
		}
		builder.WriteString("\n")
	}
	if m.review != nil {
		builder.WriteString(labelStyle.Render("SELECTED"))
		builder.WriteString("\n")
		builder.WriteString(detailTitleStyle.Render(m.review.Title))
		builder.WriteString("\n")
		builder.WriteString(m.review.Summary)
		builder.WriteString("\n")
	}
	return builder.String()
}

func containsType(types []domain.EntityType, wanted domain.EntityType) bool {
	for _, entityType := range types {
		if entityType == wanted {
			return true
		}
	}
	return false
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
	var builder strings.Builder
	label := sectionStyle.Render("INPUT")
	if strings.TrimSpace(m.sessionInput.Value()) == "" && len(m.suggestions) == 0 {
		builder.WriteString(label + "  " + mutedStyle.Render("> Type a transcript entry, $entity command, or #command; press Enter to submit"))
		return builder.String()
	}
	builder.WriteString(label + "  " + m.sessionInput.View())
	if len(m.suggestions) > 0 {
		builder.WriteString("\n")
		builder.WriteString(mutedStyle.Render("@ suggestions  ↑/↓ choose  Tab insert"))
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
		viewport.SetWidth(max(1, m.width-4))
		viewport.SetHeight(max(1, transcriptHeight-2))
		builder.WriteString(viewport.View())
	}
	return builder.String()
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
