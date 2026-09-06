package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
)

func (m *Model) openPicker() {
	m.workspace.EnsureLibrary()
	m.picking = true
	m.pickerLevel = "world"
	m.pickerCursor = 0
	m.pickerWorldID = ""
	m.session = nil
	m.planning = false
	m.editing = false
	m.reconciling = false
	m.playingBack = false
	m.namingCollection = false
	m.namingFolder = false
	m.preview = nil
	m.clearDestructiveConfirm("")
	if m.layout.ActiveWorldID != "" {
		for index, world := range m.workspace.Library {
			if world.ID == m.layout.ActiveWorldID {
				m.pickerCursor = index
				break
			}
		}
	}
	m.status = "Choose a world · Enter open · n new · q quit"
}

func (m *Model) enterScope(scope domain.Scope) {
	m.workspace.Scope = scope
	m.picking = false
	m.layout.ActiveWorldID = scope.WorldID
	m.layout.ActiveCampaignID = scope.CampaignID
	m.selectedID = ""
	m.selectedSessionID = ""
	m.selectedPlanID = ""
	m.navKind = ""
	m.planID = ""
	m.ensureBrowserSelection()
	m.search = searchsvc.New(m.workspace.Records)
	m.refreshResults()
	m.persistPreferences()
	m.status = "Opened " + scope.Campaign + " · b back to library"
}

// enterDemoCampaign skips the picker for tests/harness that expect the browser.
func (m *Model) enterDemoCampaign() {
	m.workspace.EnsureLibrary()
	scope, ok := m.workspace.ScopeFor("ashen-realms", "ashen-crown")
	if !ok {
		scope = m.workspace.Scope
	}
	m.enterScope(scope)
}

func (m Model) updatePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "backspace":
		if m.pickerLevel == "campaign" {
			m.pickerLevel = "world"
			m.pickerWorldID = ""
			m.pickerCursor = 0
			m.status = "Choose a world · Enter open · n new · q quit"
			return m, nil
		}
		return m, nil
	case "j", "down":
		m.pickerCursor = clamp(m.pickerCursor+1, 0, max(0, m.pickerItemCount()-1))
		return m, nil
	case "k", "up":
		m.pickerCursor = clamp(m.pickerCursor-1, 0, max(0, m.pickerItemCount()-1))
		return m, nil
	case "enter":
		return m.activatePickerItem()
	case "n":
		return m.createPickerItem()
	}
	return m, nil
}

func (m Model) pickerItemCount() int {
	if m.pickerLevel == "campaign" {
		world, ok := m.workspace.FindWorld(m.pickerWorldID)
		if !ok {
			return 0
		}
		return len(world.Campaigns)
	}
	return len(m.workspace.Library)
}

func (m Model) activatePickerItem() (tea.Model, tea.Cmd) {
	if m.pickerLevel == "world" {
		if m.pickerCursor < 0 || m.pickerCursor >= len(m.workspace.Library) {
			return m, nil
		}
		world := m.workspace.Library[m.pickerCursor]
		m.pickerWorldID = world.ID
		m.pickerLevel = "campaign"
		m.pickerCursor = 0
		if m.layout.ActiveWorldID == world.ID && m.layout.ActiveCampaignID != "" {
			for index, campaign := range world.Campaigns {
				if campaign.ID == m.layout.ActiveCampaignID {
					m.pickerCursor = index
					break
				}
			}
		}
		m.status = "Choose a campaign in " + world.Name + " · Enter open · Esc back"
		return m, nil
	}
	world, ok := m.workspace.FindWorld(m.pickerWorldID)
	if !ok || m.pickerCursor < 0 || m.pickerCursor >= len(world.Campaigns) {
		return m, nil
	}
	campaign := world.Campaigns[m.pickerCursor]
	scope, ok := m.workspace.ScopeFor(world.ID, campaign.ID)
	if !ok {
		m.status = "Campaign not found"
		return m, nil
	}
	m.enterScope(scope)
	return m, nil
}

func (m Model) createPickerItem() (tea.Model, tea.Cmd) {
	if m.pickerLevel == "world" {
		id := fmt.Sprintf("world-%d", len(m.workspace.Library)+1)
		name := "New World"
		m.workspace.Library = append(m.workspace.Library, domain.WorldRef{
			ID:   id,
			Name: name,
			Campaigns: []domain.CampaignRef{
				{ID: id + "-campaign", Name: "New Campaign"},
			},
		})
		m.pickerCursor = len(m.workspace.Library) - 1
		m.persistWorkspace()
		m.status = "Created " + name + " · Enter to open its campaign"
		return m, nil
	}
	world, ok := m.workspace.FindWorld(m.pickerWorldID)
	if !ok {
		return m, nil
	}
	id := fmt.Sprintf("%s-campaign-%d", world.ID, len(world.Campaigns)+1)
	name := "New Campaign"
	for index := range m.workspace.Library {
		if m.workspace.Library[index].ID != world.ID {
			continue
		}
		m.workspace.Library[index].Campaigns = append(m.workspace.Library[index].Campaigns, domain.CampaignRef{
			ID: id, Name: name,
		})
		m.pickerCursor = len(m.workspace.Library[index].Campaigns) - 1
		break
	}
	m.persistWorkspace()
	m.status = "Created " + name + " · Enter to open"
	return m, nil
}

func (m Model) renderPicker() string {
	width := max(1, m.width)
	height := max(1, m.height)
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("DUNGEON"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render("Choose a world and campaign — like picking a vault"))
	builder.WriteString("\n\n")
	if m.pickerLevel == "world" {
		builder.WriteString(sectionStyle.Render("WORLDS"))
		builder.WriteString("\n\n")
		if len(m.workspace.Library) == 0 {
			builder.WriteString(mutedStyle.Render("No worlds yet · press n to create one"))
		}
		for index, world := range m.workspace.Library {
			prefix := "  "
			style := normalItemStyle
			if index == m.pickerCursor {
				prefix = "▸ "
				style = selectedItemStyle
			}
			line := fmt.Sprintf("%s%s  (%d campaigns)", prefix, world.Name, len(world.Campaigns))
			builder.WriteString(style.Render(line))
			builder.WriteString("\n")
		}
	} else {
		world, _ := m.workspace.FindWorld(m.pickerWorldID)
		builder.WriteString(sectionStyle.Render("CAMPAIGNS"))
		builder.WriteString("  ")
		builder.WriteString(mutedStyle.Render(world.Name))
		builder.WriteString("\n\n")
		if len(world.Campaigns) == 0 {
			builder.WriteString(mutedStyle.Render("No campaigns · press n to create one"))
		}
		for index, campaign := range world.Campaigns {
			prefix := "  "
			style := normalItemStyle
			if index == m.pickerCursor {
				prefix = "▸ "
				style = selectedItemStyle
			}
			builder.WriteString(style.Render(prefix + campaign.Name))
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n")
	help := "j/k move  Enter open  n new  Esc back  q quit"
	if m.status != "" {
		help = m.status + "  ·  " + help
	}
	body := panelStyle.Width(min(72, width-4)).Render(builder.String())
	footer := footerStyle.Width(width).Render(help)
	content := lipgloss.Place(width, max(1, height-1), lipgloss.Center, lipgloss.Center, body)
	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}
