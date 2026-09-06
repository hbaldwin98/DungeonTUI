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
)

func (m Model) openPlannedNotes(create bool) (tea.Model, tea.Cmd) {
	title := textinput.New()
	title.Prompt = "Title: "
	title.CharLimit = 160
	body := textarea.New()
	body.Prompt = "│ "
	body.SetWidth(max(40, m.width-14))
	body.SetHeight(max(8, m.height-14))
	body.Placeholder = "Markdown prep notes · @Entity · #location Name"

	model := m
	model.planning = true
	model.deleteConfirm = false
	model.planTitle = title
	model.planBody = body
	model.planField = 0

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
	if m.planField == 0 {
		return m.planTitle.Focus()
	}
	return m.planBody.Focus()
}

func (m Model) updatePlannedNotes(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.planning = false
		m.status = "Closed planned notes"
		return m, nil
	case "tab", "shift+tab":
		if msg.String() == "tab" {
			m.planField = (m.planField + 1) % 2
		} else {
			m.planField = (m.planField + 1) % 2
		}
		return m, m.focusPlanEditor()
	case "ctrl+s":
		return m.savePlannedNotes()
	}
	var cmd tea.Cmd
	if m.planField == 0 {
		m.planTitle, cmd = m.planTitle.Update(msg)
	} else {
		m.planBody, cmd = m.planBody.Update(msg)
	}
	return m, cmd
}

func (m Model) savePlannedNotes() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.planTitle.Value())
	if title == "" {
		m.status = "Planned notes need a title"
		return m, nil
	}
	now := time.Now().UTC()
	notes := m.planDraft
	notes.ID = m.planID
	notes.Title = title
	notes.Body = m.planBody.Value()
	notes.Scope = m.workspace.Scope
	if notes.CreatedAt.IsZero() {
		notes.CreatedAt = now
	}
	notes = notes.RefreshPlannedLinks(m.workspace.Records)
	notes.UpdatedAt = now
	if err := notes.Validate(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	replaced := false
	for index := range m.workspace.PlannedNotes {
		if m.workspace.PlannedNotes[index].ID == notes.ID {
			m.workspace.PlannedNotes[index] = notes
			replaced = true
			break
		}
	}
	if !replaced {
		m.workspace.PlannedNotes = append(m.workspace.PlannedNotes, notes)
	}
	m.planID = notes.ID
	m.planDraft = notes
	m.planning = false
	m.persistWorkspace()
	m.status = fmt.Sprintf("Saved planned notes · %d links · %s · press s to play", len(notes.Links), notes.Title)
	return m, nil
}

func (m Model) renderPlannedNotesOverlay() string {
	width := min(92, max(56, m.width-8))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("PLANNED SESSION NOTES"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("prep only · not a live transcript"))
	builder.WriteString("\n\n")
	builder.WriteString(m.planTitle.View())
	builder.WriteString("\n\n")
	builder.WriteString(m.planBody.View())
	builder.WriteString("\n")
	if len(m.planDraft.Links) > 0 || m.planDraft.LocationName != "" {
		builder.WriteString(mutedStyle.Render("Resolved on save: "))
		parts := make([]string, 0, len(m.planDraft.Links)+1)
		if m.planDraft.LocationName != "" {
			parts = append(parts, "loc "+m.planDraft.LocationName)
		}
		for _, link := range m.planDraft.Links {
			parts = append(parts, "@"+link.Text)
		}
		builder.WriteString(filterStyle.Render(strings.Join(parts, " · ")))
		builder.WriteString("\n")
	}
	builder.WriteString(helpStyle.Render("Tab fields  Ctrl+S save  Esc cancel  ·  @Entity  #location Name"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
	)
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
