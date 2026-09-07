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
	m.namingPicker = false
	m.preview = nil
	m.importing = false
	m.clearDestructiveConfirm("")
	if m.layout.ActiveWorldID != "" {
		for index, world := range m.workspace.Library {
			if world.ID == m.layout.ActiveWorldID {
				m.pickerCursor = index
				break
			}
		}
	}
	m.status = "Choose a world · Enter open · n new · e rename · d delete · q quit"
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
	m.attachReferences()
	m.rebuildSearch()
	m.refreshResults()
	m.persistPreferences()
	if m.resumeUnfinishedSession(scope) {
		m.status = "Resumed " + m.session.Title + " · Ctrl+E ends capture"
	} else {
		m.status = "Opened " + scope.Campaign + " · b back to library"
	}
}

func (m *Model) resumeUnfinishedSession(scope domain.Scope) bool {
	for index := len(m.workspace.Sessions) - 1; index >= 0; index-- {
		session := m.workspace.Sessions[index]
		if session.EndedAt == nil && session.Scope.WorldID == scope.WorldID && session.Scope.CampaignID == scope.CampaignID {
			m.session = &session
			m.sessionInput.Focus()
			m.configureTranscriptViewport()
			m.refreshTranscriptViewport()
			m.refreshSuggestions()
			return true
		}
	}
	m.session = nil
	return false
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
	if m.namingPicker {
		return m.updatePickerName(msg)
	}
	if m.deleteConfirm {
		switch msg.String() {
		case "y":
			return m.confirmPickerDelete()
		case "n", "esc":
			m.clearDestructiveConfirm("Cancelled")
			return m, nil
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		return m.openHelp()
	case "esc", "backspace":
		if m.pickerLevel == "campaign" {
			m.pickerLevel = "world"
			m.pickerWorldID = ""
			m.pickerCursor = 0
			m.status = pickerWorldHint()
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
	case "I", "i":
		return m.openImport()
	case "n":
		return m.createPickerItem()
	case "e":
		return m.openPickerRename()
	case "d":
		return m.armPickerDelete()
	}
	return m, nil
}

func pickerWorldHint() string {
	return "Choose a world · Enter open · n new · e rename · d delete · q quit"
}

func pickerCampaignHint(worldName string) string {
	return "Choose a campaign in " + worldName + " · Enter open · n new · e rename · d delete · Esc back"
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
		m.status = pickerCampaignHint(world.Name)
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
	before := m.workspace.Clone()
	if m.pickerLevel == "world" {
		id := nextAvailableID("world", func(candidate string) bool {
			_, exists := m.workspace.FindWorld(candidate)
			return exists
		})
		name := "New World"
		m.workspace.Library = append(m.workspace.Library, domain.WorldRef{
			ID:   id,
			Name: name,
			Campaigns: []domain.CampaignRef{
				{ID: id + "-campaign", Name: "New Campaign"},
			},
		})
		m.pickerCursor = len(m.workspace.Library) - 1
		if err := m.persistWorkspace(); err != nil {
			m.workspace = before
			return m, nil
		}
		return m.openPickerRename()
	}
	world, ok := m.workspace.FindWorld(m.pickerWorldID)
	if !ok {
		return m, nil
	}
	id := nextAvailableID(world.ID+"-campaign", func(candidate string) bool {
		for _, item := range m.workspace.Library {
			for _, campaign := range item.Campaigns {
				if campaign.ID == candidate {
					return true
				}
			}
		}
		return false
	})
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
	if err := m.persistWorkspace(); err != nil {
		m.workspace = before
		return m, nil
	}
	return m.openPickerRename()
}

func nextAvailableID(prefix string, exists func(string) bool) string {
	for suffix := time.Now().UTC().UnixNano(); ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", prefix, suffix)
		if !exists(candidate) {
			return candidate
		}
	}
}

func (m Model) selectedPickerWorld() (domain.WorldRef, bool) {
	if m.pickerLevel == "world" {
		if m.pickerCursor < 0 || m.pickerCursor >= len(m.workspace.Library) {
			return domain.WorldRef{}, false
		}
		return m.workspace.Library[m.pickerCursor], true
	}
	return m.workspace.FindWorld(m.pickerWorldID)
}

func (m Model) selectedPickerCampaign() (domain.CampaignRef, bool) {
	world, ok := m.workspace.FindWorld(m.pickerWorldID)
	if !ok || m.pickerCursor < 0 || m.pickerCursor >= len(world.Campaigns) {
		return domain.CampaignRef{}, false
	}
	return world.Campaigns[m.pickerCursor], true
}

func (m Model) openPickerRename() (tea.Model, tea.Cmd) {
	kind := "world"
	name := ""
	if m.pickerLevel == "campaign" {
		campaign, ok := m.selectedPickerCampaign()
		if !ok {
			m.status = "Nothing selected"
			return m, nil
		}
		kind = "campaign"
		name = campaign.Name
	} else {
		world, ok := m.selectedPickerWorld()
		if !ok {
			m.status = "Nothing selected"
			return m, nil
		}
		name = world.Name
	}
	m.namingPicker = true
	m.collectionName.Placeholder = kind + " name"
	m.collectionName.Prompt = "Name: "
	m.collectionName.SetValue(name)
	m.collectionName.SetWidth(max(24, min(48, m.width-16)))
	m.collectionName.Focus()
	m.status = "Rename " + kind + " · Enter save · Esc cancel"
	return m, textinput.Blink
}

func (m *Model) closePickerRename() {
	m.namingPicker = false
	m.collectionName.Blur()
	m.collectionName.Placeholder = "Collection name"
	m.collectionName.Prompt = "Name: "
	m.collectionName.SetValue("")
}

func (m Model) updatePickerName(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closePickerRename()
		m.status = "Cancelled rename"
		return m, nil
	case "enter":
		return m.savePickerRename()
	}
	var cmd tea.Cmd
	m.collectionName, cmd = m.collectionName.Update(msg)
	return m, cmd
}

func (m Model) savePickerRename() (tea.Model, tea.Cmd) {
	before := m.workspace.Clone()
	name := strings.TrimSpace(m.collectionName.Value())
	if name == "" {
		m.status = "Name is required"
		return m, nil
	}
	var err error
	if m.pickerLevel == "campaign" {
		campaign, ok := m.selectedPickerCampaign()
		if !ok {
			m.closePickerRename()
			m.status = "Nothing selected"
			return m, nil
		}
		err = m.workspace.RenameCampaign(m.pickerWorldID, campaign.ID, name)
	} else {
		world, ok := m.selectedPickerWorld()
		if !ok {
			m.closePickerRename()
			m.status = "Nothing selected"
			return m, nil
		}
		err = m.workspace.RenameWorld(world.ID, name)
	}
	m.closePickerRename()
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	if err := m.persistWorkspace(); err != nil {
		m.workspace = before
		return m, nil
	}
	m.status = "Renamed to “" + name + "”"
	return m, nil
}

func (m Model) armPickerDelete() (tea.Model, tea.Cmd) {
	if m.pickerLevel == "campaign" {
		campaign, ok := m.selectedPickerCampaign()
		if !ok {
			m.status = "Nothing selected"
			return m, nil
		}
		m.deleteConfirm = true
		m.confirmKind = "delete-campaign"
		m.status = "Delete campaign “" + campaign.Name + "” and its wiki, sessions, and prep?  y confirm · n/Esc cancel"
		return m, nil
	}
	world, ok := m.selectedPickerWorld()
	if !ok {
		m.status = "Nothing selected"
		return m, nil
	}
	m.deleteConfirm = true
	m.confirmKind = "delete-world"
	m.status = "Delete world “" + world.Name + "” and every campaign inside it?  y confirm · n/Esc cancel"
	return m, nil
}

func (m Model) confirmPickerDelete() (tea.Model, tea.Cmd) {
	before := m.workspace.Clone()
	kind := m.confirmKind
	m.clearDestructiveConfirm("")
	var err error
	label := ""
	switch kind {
	case "delete-campaign":
		campaign, ok := m.selectedPickerCampaign()
		if !ok {
			m.status = "Nothing selected"
			return m, nil
		}
		label = campaign.Name
		err = m.workspace.DeleteCampaign(m.pickerWorldID, campaign.ID)
	case "delete-world":
		world, ok := m.selectedPickerWorld()
		if !ok {
			m.status = "Nothing selected"
			return m, nil
		}
		label = world.Name
		err = m.workspace.DeleteWorld(world.ID)
	default:
		return m, nil
	}
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.pickerCursor = clamp(m.pickerCursor, 0, max(0, m.pickerItemCount()-1))
	m.syncPickerLayout()
	if err := m.persistWorkspace(); err != nil {
		m.workspace = before
		m.syncPickerLayout()
		return m, nil
	}
	m.status = "Deleted “" + label + "”"
	return m, nil
}

func (m *Model) syncPickerLayout() {
	changed := false
	if m.layout.ActiveWorldID != "" {
		if _, ok := m.workspace.FindWorld(m.layout.ActiveWorldID); !ok {
			m.layout.ActiveWorldID = ""
			m.layout.ActiveCampaignID = ""
			changed = true
		} else if m.layout.ActiveCampaignID != "" {
			if _, ok := m.workspace.ScopeFor(m.layout.ActiveWorldID, m.layout.ActiveCampaignID); !ok {
				m.layout.ActiveCampaignID = ""
				changed = true
			}
		}
	}
	if changed {
		m.persistPreferences()
	}
}

func (m Model) renderPicker() string {
	width := max(1, m.width)
	height := max(1, m.height)
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("DUNGEON"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render("Choose a world and campaign — like picking a vault"))
	builder.WriteString("\n\n")
	if m.namingPicker {
		kind := "WORLD"
		if m.pickerLevel == "campaign" {
			kind = "CAMPAIGN"
		}
		builder.WriteString(sectionStyle.Render("RENAME " + kind))
		builder.WriteString("\n\n")
		builder.WriteString(m.collectionName.View())
		builder.WriteString("\n")
	} else if m.pickerLevel == "world" {
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
	help := "j/k move  Enter open  n new  e rename  d delete  I import  Esc back  q quit"
	if m.namingPicker {
		help = "Enter save  Esc cancel"
	}
	if m.status != "" {
		help = m.status + "  ·  " + help
	}
	body := panelStyle.Width(min(72, width-4)).Render(builder.String())
	footer := footerStyle.Width(width).Render(help)
	content := lipgloss.Place(width, max(1, height-1), lipgloss.Center, lipgloss.Center, body)
	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}
