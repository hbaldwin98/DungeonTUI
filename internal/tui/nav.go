package tui

import (
	"fmt"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
)

// NavKind identifies a first-class branch in the in-campaign tree.
type NavKind string

const (
	NavSessions NavKind = "sessions"
	NavPrep     NavKind = "prep"
	NavType     NavKind = "type"
)

// navEntry is one row in the campaign navigation tree.
type navEntry struct {
	Kind  NavKind
	Type  domain.EntityType
	Label string
}

func campaignTreeEntries() []navEntry {
	return []navEntry{
		{Kind: NavSessions, Label: "Sessions"},
		{Kind: NavPrep, Label: "Prep"},
		{Kind: NavType, Type: domain.NPC, Label: "NPCs"},
		{Kind: NavType, Type: domain.Character, Label: "Characters"},
		{Kind: NavType, Type: domain.Location, Label: "Locations"},
		{Kind: NavType, Type: domain.Faction, Label: "Factions"},
		{Kind: NavType, Type: domain.Item, Label: "Items"},
		{Kind: NavType, Type: domain.Creature, Label: "Creatures"},
		{Kind: NavType, Type: domain.Thread, Label: "Threads"},
		{Kind: NavType, Type: domain.Event, Label: "Timeline"},
		{Kind: NavType, Type: domain.Note, Label: "Knowledge"},
	}
}

func (m Model) navEntries() []navEntry {
	return campaignTreeEntries()
}

func (m Model) currentNav() navEntry {
	entries := m.navEntries()
	if len(entries) == 0 {
		return navEntry{Kind: NavType, Type: domain.NPC, Label: "NPCs"}
	}
	return entries[clamp(m.navCursor, 0, len(entries)-1)]
}

func (m *Model) setNavCursor(index int) {
	entries := m.navEntries()
	if len(entries) == 0 {
		return
	}
	m.navCursor = clamp(index, 0, len(entries)-1)
	entry := entries[m.navCursor]
	m.navKind = entry.Kind
	m.navType = entry.Type
	m.cursor = 0
	m.selectedID = ""
	m.selectedSessionID = ""
	m.selectedPlanID = ""
	switch entry.Kind {
	case NavType:
		m.typeFilter = entry.Type
		records := m.listRecords()
		if len(records) > 0 {
			m.selectRecord(records[0])
		}
	case NavSessions:
		m.typeFilter = ""
		sessions := m.scopedSessions()
		if len(sessions) > 0 {
			m.selectedSessionID = sessions[0].ID
		}
	case NavPrep:
		m.typeFilter = ""
		plans := m.scopedPlannedNotes()
		if len(plans) > 0 {
			m.selectedPlanID = plans[0].ID
		}
	}
}

func (m *Model) moveNavCursor(delta int) {
	m.setNavCursor(m.navCursor + delta)
}

func (m Model) scopedSessions() []domain.SessionRecord {
	out := make([]domain.SessionRecord, 0)
	for _, session := range m.workspace.Sessions {
		if session.Scope.WorldID == m.workspace.Scope.WorldID &&
			session.Scope.CampaignID == m.workspace.Scope.CampaignID {
			out = append(out, session)
		}
	}
	return out
}

func (m Model) scopedPlannedNotes() []domain.PlannedNotes {
	out := make([]domain.PlannedNotes, 0)
	for _, plan := range m.workspace.PlannedNotes {
		if plan.Scope.WorldID == m.workspace.Scope.WorldID &&
			plan.Scope.CampaignID == m.workspace.Scope.CampaignID {
			out = append(out, plan)
		}
	}
	return out
}

func (m Model) listRecords() []domain.Record {
	entry := m.currentNav()
	if entry.Kind != NavType {
		return nil
	}
	out := make([]domain.Record, 0)
	for _, record := range m.scopedRecords(false) {
		if record.Type == entry.Type {
			out = append(out, record)
		}
	}
	return out
}

func (m Model) usesCampaignTree() bool {
	return prefs.FindVisibleLeaf(m.layout.Browser.Root, prefs.PaneNav)
}

func (m Model) renderNavTree() string {
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("CAMPAIGN"))
	if m.layout.Focus == prefs.PaneNav {
		builder.WriteString(mutedStyle.Render(" · focused"))
	}
	builder.WriteString("\n\n")
	for index, entry := range m.navEntries() {
		cursor := "  "
		style := normalItemStyle
		if index == m.navCursor {
			cursor = "▸ "
			if m.layout.Focus == prefs.PaneNav {
				style = selectedItemStyle
			}
		}
		builder.WriteString(style.Render(cursor + entry.Label))
		builder.WriteRune('\n')
	}
	return builder.String()
}

func (m Model) renderListPane() string {
	entry := m.currentNav()
	title := strings.ToUpper(entry.Label)
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render(title))
	filters := []string{}
	if m.listScope != searchsvc.CurrentCampaign {
		filters = append(filters, m.listScope.Label())
	}
	if m.tagFilter != "" {
		filters = append(filters, "#"+m.tagFilter)
	}
	if col := m.activeCollection(); col != nil {
		filters = append(filters, col.Title)
	}
	if m.layout.Focus == prefs.PaneList {
		filters = append(filters, "focused")
	}
	if len(filters) > 0 {
		builder.WriteString(mutedStyle.Render(" · " + strings.Join(filters, " · ")))
	}
	builder.WriteString("\n\n")

	switch entry.Kind {
	case NavSessions:
		sessions := m.scopedSessions()
		if len(sessions) == 0 {
			builder.WriteString(mutedStyle.Render("  — no sessions yet"))
			return builder.String()
		}
		for index, session := range sessions {
			cursor := "  "
			style := normalItemStyle
			if session.ID == m.selectedSessionID || (m.layout.Focus == prefs.PaneList && index == m.cursor) {
				cursor = "▸ "
				style = selectedItemStyle
			}
			state := "live"
			if session.EndedAt != nil {
				state = "ended"
			}
			line := fmt.Sprintf("%s%s · %s", cursor, session.Title, state)
			builder.WriteString(style.Render(line))
			builder.WriteRune('\n')
		}
	case NavPrep:
		plans := m.scopedPlannedNotes()
		if len(plans) == 0 {
			builder.WriteString(mutedStyle.Render("  — no prep notes yet · p to draft"))
			return builder.String()
		}
		for index, plan := range plans {
			cursor := "  "
			style := normalItemStyle
			if plan.ID == m.selectedPlanID || (m.layout.Focus == prefs.PaneList && index == m.cursor) {
				cursor = "▸ "
				style = selectedItemStyle
			}
			line := fmt.Sprintf("%s%s", cursor, plan.Title)
			builder.WriteString(style.Render(line))
			builder.WriteRune('\n')
		}
	default:
		records := m.listRecords()
		if len(records) == 0 {
			builder.WriteString(mutedStyle.Render("  —"))
			return builder.String()
		}
		for index, record := range records {
			cursor := "  "
			style := normalItemStyle
			if record.ID == m.selectedID || (m.layout.Focus == prefs.PaneList && index == m.cursor) {
				cursor = "▸ "
				style = selectedItemStyle
			}
			line := fmt.Sprintf("%s%s %s", cursor, record.Authority.Marker(), record.Title)
			builder.WriteString(style.Render(line))
			builder.WriteRune('\n')
		}
	}
	return builder.String()
}

func (m Model) renderTreeDetail() string {
	entry := m.currentNav()
	switch entry.Kind {
	case NavSessions:
		for _, session := range m.scopedSessions() {
			if session.ID != m.selectedSessionID {
				continue
			}
			state := "Live"
			if session.EndedAt != nil {
				state = "Ended"
			}
			var builder strings.Builder
			builder.WriteString(sectionStyle.Render("SESSION"))
			builder.WriteString("\n\n")
			builder.WriteString(titleStyle.Render(session.Title))
			builder.WriteString("\n")
			builder.WriteString(mutedStyle.Render(state))
			if session.LocationName != "" {
				builder.WriteString("\n")
				builder.WriteString("Location: " + session.LocationName)
			}
			builder.WriteString(fmt.Sprintf("\nEntries: %d", len(session.Entries)))
			if session.PlannedNotesID != "" {
				if plan := m.plannedByID(session.PlannedNotesID); plan != nil {
					builder.WriteString("\nPrep: " + plan.Title)
				} else {
					builder.WriteString("\nSeeded from prep notes")
				}
			}
			if len(session.Links) > 0 {
				builder.WriteString("\n\n")
				builder.WriteString(labelStyle.Render("CAST"))
				builder.WriteString("\n")
				for _, link := range session.Links {
					title := link.Text
					for _, record := range m.workspace.Records {
						if record.ID == link.RecordID {
							title = record.Title
							break
						}
					}
					builder.WriteString("  · " + title + "\n")
				}
			}
			builder.WriteString("\n\n")
			builder.WriteString(mutedStyle.Render("Enter playback · s starts live · d deletes this session"))
			return builder.String()
		}
		return mutedStyle.Render("Select a session")
	case NavPrep:
		for _, plan := range m.scopedPlannedNotes() {
			if plan.ID != m.selectedPlanID {
				continue
			}
			var builder strings.Builder
			builder.WriteString(sectionStyle.Render("PREP NOTES"))
			builder.WriteString("\n\n")
			builder.WriteString(titleStyle.Render(plan.Title))
			if plan.LocationName != "" {
				builder.WriteString("\n")
				builder.WriteString("Location: " + plan.LocationName)
			}
			if len(plan.Links) > 0 {
				builder.WriteString(fmt.Sprintf("\nLinks: %d", len(plan.Links)))
			}
			if len(plan.PriorSessionIDs) > 0 {
				sits := domain.ResolvePriorSits(m.workspace.Sessions, m.workspace.Records, plan.PriorSessionIDs)
				builder.WriteString("\n")
				builder.WriteString(labelStyle.Render("PRIOR SITS"))
				for _, sit := range sits {
					builder.WriteString("\n  " + sit.Title)
					if sit.LocationName != "" {
						builder.WriteString(" · " + sit.LocationName)
					}
				}
			}
			if live := domain.SessionsSeededFrom(m.workspace.Sessions, plan.ID); len(live) > 0 {
				builder.WriteString("\n")
				builder.WriteString(labelStyle.Render("LIVE SITS"))
				for _, sit := range live {
					state := "live"
					if sit.EndedAt != nil {
						state = sit.StartedAt.Local().Format("2006-01-02")
					}
					builder.WriteString(fmt.Sprintf("\n  %s · %s", sit.Title, state))
				}
			}
			builder.WriteString("\n\n")
			body := strings.TrimSpace(plan.Body)
			if body == "" {
				builder.WriteString(mutedStyle.Render("(empty)"))
			} else {
				builder.WriteString(body)
			}
			builder.WriteString("\n\n")
			builder.WriteString(mutedStyle.Render("Enter/e edits · s starts another live sit from this prep"))
			return builder.String()
		}
		return mutedStyle.Render("Select prep notes · p to draft")
	default:
		return m.renderDetail()
	}
}
