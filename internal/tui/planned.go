package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func (m Model) openPlannedNotes(create bool) (tea.Model, tea.Cmd) {
	title := textinput.New()
	title.Prompt = "Title: "
	title.CharLimit = 160
	title.SetWidth(max(20, m.width-10))
	body := newMarkdownTextArea(m.width, m.height-4)
	body.Placeholder = "Prep markdown · @Entity · #location Name"

	model := m
	model.planning = true
	model.deleteConfirm = false
	model.planTitle = title
	model.planBody = body
	model.planField = 0
	model.suggestions = nil
	model.suggestion = 0

	if create || m.planID == "" || m.plannedByID(m.planID) == nil {
		now := time.Now().UTC()
		model.planID = fmt.Sprintf("plan-%d", now.UnixNano())
		model.planTitle.SetValue("Prep " + now.Format("2006-01-02"))
		prior := domain.DefaultPriorSessionIDs(m.workspace.Sessions, m.workspace.Scope, 3)
		body := "# Next session\n\n#location \n\n- \n"
		if sits := domain.ResolvePriorSits(m.workspace.Sessions, m.workspace.Records, prior); len(sits) > 0 {
			if sits[0].LocationName != "" {
				body = "# Next session\n\n#location " + sits[0].LocationName + "\n\n- Follow up from " + sits[0].Title + "\n"
			} else {
				body = "# Next session\n\n#location \n\n- Follow up from " + sits[0].Title + "\n"
			}
		}
		model.planDraft = domain.PlannedNotes{
			ID:              model.planID,
			Title:           model.planTitle.Value(),
			Scope:           m.workspace.Scope,
			Body:            body,
			PriorSessionIDs: prior,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		model.planBody.SetValue(model.planDraft.Body)
	} else {
		notes := *m.plannedByID(m.planID)
		model.planDraft = notes
		model.planTitle.SetValue(notes.Title)
		model.planBody.SetValue(notes.Body)
	}
	return model, model.focusPlanEditor()
}

func (m Model) plannedByID(id string) *domain.PlannedNotes {
	for index := range m.workspace.PlannedNotes {
		if m.workspace.PlannedNotes[index].ID == id {
			return &m.workspace.PlannedNotes[index]
		}
	}
	return nil
}

func (m *Model) focusPlanEditor() tea.Cmd {
	m.planTitle.Blur()
	m.planBody.Blur()
	m.suggestions = nil
	m.suggestion = 0
	if m.planField == 0 {
		return m.planTitle.Focus()
	}
	return m.planBody.Focus()
}

func (m Model) updatePlannedNotes(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.planning = false
		m.suggestions = nil
		m.status = "Closed planned notes"
		return m, nil
	case "tab":
		if m.planField == 1 && m.acceptEditorSuggestion() {
			return m, nil
		}
		m.planField = (m.planField + 1) % 2
		return m, m.focusPlanEditor()
	case "shift+tab":
		m.planField = (m.planField + 1) % 2
		return m, m.focusPlanEditor()
	case "ctrl+s":
		return m.savePlannedNotes()
	case "ctrl+p":
		return m.cyclePlanPriorSits()
	case "ctrl+o":
		if m.openPeekPreview() {
			return m, nil
		}
	case "up":
		if m.planField == 1 && len(m.suggestions) > 0 {
			m.suggestion = clamp(m.suggestion-1, 0, len(m.suggestions)-1)
			m.refreshPeek()
			return m, nil
		}
	case "down":
		if m.planField == 1 && len(m.suggestions) > 0 {
			m.suggestion = clamp(m.suggestion+1, 0, len(m.suggestions)-1)
			m.refreshPeek()
			return m, nil
		}
	case "pgup":
		if m.peek != nil {
			m.scrollPeek(-1)
			return m, nil
		}
	case "pgdown":
		if m.peek != nil {
			m.scrollPeek(1)
			return m, nil
		}
	}
	var cmd tea.Cmd
	if m.planField == 0 {
		m.planTitle, cmd = m.planTitle.Update(msg)
		m.suggestions = nil
	} else {
		m.planBody, cmd = m.planBody.Update(msg)
		m.refreshEditorSuggestions()
	}
	return m, cmd
}

func (m Model) cyclePlanPriorSits() (tea.Model, tea.Cmd) {
	available := domain.DefaultPriorSessionIDs(m.workspace.Sessions, m.workspace.Scope, 3)
	if len(available) == 0 {
		m.status = "No ended sessions to attach"
		return m, nil
	}
	current := len(m.planDraft.PriorSessionIDs)
	next := (current + 1) % (len(available) + 1)
	if next == 0 {
		m.planDraft.PriorSessionIDs = nil
		m.status = "Prep is not attached to prior sits"
		return m, nil
	}
	m.planDraft.PriorSessionIDs = append([]string(nil), available[:next]...)
	m.status = fmt.Sprintf("Attached %d prior sit(s) · Ctrl+P cycles", next)
	return m, nil
}

func (m Model) renderPlanLiveSits() string {
	if m.planDraft.ID == "" {
		return ""
	}
	sits := domain.SessionsSeededFrom(m.workspace.Sessions, m.planDraft.ID)
	if len(sits) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(labelStyle.Render("LIVE SITS"))
	for _, sit := range sits {
		state := "live"
		if sit.EndedAt != nil {
			state = sit.StartedAt.Local().Format("2006-01-02")
		}
		builder.WriteString(fmt.Sprintf("\n  %s · %s", sit.Title, state))
	}
	return builder.String()
}

func (m Model) savePlannedNotes() (tea.Model, tea.Cmd) {
	before := m.workspace.Clone()
	now := time.Now().UTC()
	notes := m.planDraft
	notes.ID = m.planID
	notes.Title = strings.TrimSpace(m.planTitle.Value())
	notes.Body = m.planBody.Value()
	notes.Scope = m.workspace.Scope
	notes.UpdatedAt = now
	if notes.CreatedAt.IsZero() {
		notes.CreatedAt = now
	}
	visible := make([]domain.Record, 0, len(m.workspace.Records))
	for _, record := range m.workspace.Records {
		if domain.RecordVisibleIn(record, m.workspace.Scope, m.workspace.EnabledSourceIDs(m.workspace.Scope)) {
			visible = append(visible, record)
		}
	}
	notes = notes.RefreshPlannedLinks(visible)
	if err := notes.Validate(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	updated := false
	for index := range m.workspace.PlannedNotes {
		if m.workspace.PlannedNotes[index].ID == notes.ID {
			m.workspace.PlannedNotes[index] = notes
			updated = true
			break
		}
	}
	if !updated {
		m.workspace.PlannedNotes = append(m.workspace.PlannedNotes, notes)
	}
	m.planID = notes.ID
	m.selectedPlanID = notes.ID
	m.planning = false
	m.suggestions = nil
	if err := m.persistWorkspace(); err != nil {
		m.replaceWorkspace(before)
		return m, nil
	}
	m.status = "Saved planned notes · s starts another live sit from this prep"
	return m, nil
}

func (m Model) renderPlannedNotesOverlay() string {
	width := max(1, m.width)
	height := max(1, m.height)
	suggest := ""
	if m.planField == 1 {
		suggest = renderSuggestionList(m.suggestions, m.suggestion)
	}
	peek := m.renderPeekPanel(5)
	prior := m.renderPlanPriorSits()
	live := m.renderPlanLiveSits()
	reserve := 3
	if suggest != "" {
		reserve += lipgloss.Height(suggest) + 1
	}
	if peek != "" {
		reserve += lipgloss.Height(peek) + 1
	}
	if prior != "" {
		reserve += lipgloss.Height(prior) + 1
	}
	if live != "" {
		reserve += lipgloss.Height(live) + 1
	}
	sizeMarkdownTextAreaReserved(&m.planBody, width, height, reserve)
	m.planTitle.SetWidth(max(20, width-10))

	var chrome strings.Builder
	chrome.WriteString(headerStyle.Width(width).Render(
		searchTitleStyle.Render("PLANNED NOTES") + "  " + mutedStyle.Render("prep · not a live session"),
	))
	chrome.WriteString("\n")
	chrome.WriteString(m.planTitle.View())
	if prior != "" {
		chrome.WriteString("\n")
		chrome.WriteString(prior)
	}
	if live := m.renderPlanLiveSits(); live != "" {
		chrome.WriteString("\n")
		chrome.WriteString(live)
	}
	if len(m.planDraft.Links) > 0 || m.planDraft.LocationName != "" {
		chrome.WriteString("\n")
		parts := []string{}
		if m.planDraft.LocationName != "" {
			parts = append(parts, "#location "+m.planDraft.LocationName)
		}
		for _, link := range m.planDraft.Links {
			parts = append(parts, "@"+link.Text)
		}
		chrome.WriteString(mutedStyle.Render(strings.Join(parts, "  ")))
	}

	body := m.planBody.View()
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
	help := "? help · Tab fields · Ctrl+S save · Ctrl+P prior sits · Esc · @Entity"
	return renderFullScreenEditor(width, height, chrome.String(), body, help)
}

func (m Model) renderPlanPriorSits() string {
	sits := m.planPriorSits()
	var builder strings.Builder
	builder.WriteString(labelStyle.Render("PRIOR SITS"))
	if len(sits) == 0 {
		builder.WriteString("  ")
		builder.WriteString(mutedStyle.Render("none · Ctrl+P attaches ended sessions"))
		return builder.String()
	}
	for _, sit := range sits {
		builder.WriteString("\n  ")
		builder.WriteString(sit.Summary())
	}
	return builder.String()
}

func (m Model) planPriorSits() []domain.PriorSit {
	return domain.ResolvePriorSits(m.workspace.Sessions, m.workspace.Records, m.planDraft.PriorSessionIDs)
}

func (m Model) activePlannedNotes() *domain.PlannedNotes {
	if m.selectedPlanID != "" {
		return m.plannedByID(m.selectedPlanID)
	}
	if m.planID != "" {
		return m.plannedByID(m.planID)
	}
	return nil
}
