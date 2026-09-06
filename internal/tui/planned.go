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
		prior := []string{}
		if len(m.workspace.Sessions) > 0 {
			prior = append(prior, m.workspace.Sessions[len(m.workspace.Sessions)-1].ID)
		}
		model.planDraft = domain.PlannedNotes{
			ID:              model.planID,
			Title:           model.planTitle.Value(),
			Scope:           m.workspace.Scope,
			Body:            "# Next session\n\n#location \n\n- \n",
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

func (m Model) savePlannedNotes() (tea.Model, tea.Cmd) {
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
	notes = notes.RefreshPlannedLinks(m.workspace.Records)
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
	m.persistWorkspace()
	m.status = "Saved planned notes · s starts live from this prep"
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
	reserve := 3
	if suggest != "" {
		reserve += lipgloss.Height(suggest) + 1
	}
	if peek != "" {
		reserve += lipgloss.Height(peek) + 1
	}
	sizeMarkdownTextAreaReserved(&m.planBody, width, height, reserve)
	m.planTitle.SetWidth(max(20, width-10))

	var chrome strings.Builder
	chrome.WriteString(headerStyle.Width(width).Render(
		searchTitleStyle.Render("PLANNED NOTES") + "  " + mutedStyle.Render("prep · not a live session"),
	))
	chrome.WriteString("\n")
	chrome.WriteString(m.planTitle.View())
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
	help := "? help · Tab fields · Ctrl+S save · Esc · @Entity · #location"
	return renderFullScreenEditor(width, height, chrome.String(), body, help)
}

func (m Model) activePlannedNotes() *domain.PlannedNotes {
	if m.selectedPlanID != "" {
		if plan := m.plannedByID(m.selectedPlanID); plan != nil {
			return plan
		}
	}
	if m.planID != "" {
		if plan := m.plannedByID(m.planID); plan != nil {
			return plan
		}
	}
	if len(m.workspace.PlannedNotes) == 0 {
		return nil
	}
	return &m.workspace.PlannedNotes[len(m.workspace.PlannedNotes)-1]
}
