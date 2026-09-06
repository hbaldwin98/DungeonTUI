package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
	"github.com/hbaldwin98/dungeon/internal/storage"
)

type Model struct {
	workspace    domain.Workspace
	search       searchsvc.Service
	cursor       int
	width        int
	height       int
	searching    bool
	searchInput  textinput.Model
	searchScope  searchsvc.Scope
	includeIdeas bool
	results      []searchsvc.Result
	selected     int
	typeFilter   domain.EntityType
	store        storage.Store
	status       string
	editing      bool
	creating     bool
	editIndex    int
	editField    int
	editType     domain.EntityType
	editTitle    textinput.Model
	editSummary  textinput.Model
	editBody     textarea.Model
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
		return m, nil
	case tea.KeyPressMsg:
		if m.editing {
			return m.updateEditor(msg)
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
		}
	case tea.MouseClickMsg:
		return m.updateMouseClick(msg)
	case tea.MouseWheelMsg:
		return m.updateMouseWheel(msg)
	}

	return m, nil
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
	help := "/ search  n new draft  e edit  t filter type  j/k navigate  mouse: click/scroll  q quit"
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
