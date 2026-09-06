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
	m.selectedFolderPath = ""
	switch entry.Kind {
	case NavType:
		m.typeFilter = entry.Type
		records := m.listRecords()
		if len(records) > 0 {
			m.selectRecord(records[0])
		}
	case NavSessions:
		m.typeFilter = ""
		m.selectFirstSessionTreeRow()
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

func sessionRowIndent(row domain.SessionTreeRow) int {
	indent := row.Depth * 2
	if row.Kind == domain.SessionTreeSession {
		indent += 2
	}
	return indent
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
		rows := m.sessionTreeRows()
		if len(rows) == 0 {
			builder.WriteString(mutedStyle.Render("  — no sessions yet · s starts live · m files into folders"))
			return builder.String()
		}
		for index, row := range rows {
			cursor := "  "
			style := normalItemStyle
			selected := (m.layout.Focus == prefs.PaneList && index == m.cursor) ||
				(row.Kind == domain.SessionTreeSession && row.Session.ID == m.selectedSessionID) ||
				(row.Kind == domain.SessionTreeFolder && m.selectedSessionID == "" && row.Path == m.selectedFolderPath)
			if selected {
				cursor = "▸ "
				style = selectedItemStyle
			}
			var rest string
			if row.Kind == domain.SessionTreeFolder {
				glyph := "▾"
				if m.collapsedFolders[row.Path] {
					glyph = "▸"
				}
				rest = fmt.Sprintf("%s %s · %d", glyph, row.Label, row.Count)
			} else {
				state := "live"
				if row.Session.EndedAt != nil {
					state = "ended"
				}
				rest = fmt.Sprintf("%s · %s", row.Label, state)
			}
			builder.WriteString(style.PaddingLeft(sessionRowIndent(row)).Render(cursor + rest))
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
		if m.selectedSessionID == "" {
			return m.renderSessionFolderDetail(m.selectedFolderPath)
		}
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
			if folder := domain.NormalizeFolder(session.Folder); folder != "" {
				builder.WriteString("\n")
				builder.WriteString("Folder: " + strings.ReplaceAll(folder, "/", " / "))
			}
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
			builder.WriteString("\n\n")
			builder.WriteString(m.renderCastHops())
			builder.WriteString("\n")
			builder.WriteString(mutedStyle.Render("List Enter playback · detail Enter follows · m files · s live · d deletes"))
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
			builder.WriteString("\n\n")
			builder.WriteString(m.renderCastHops())
			builder.WriteString("\n")
			body := strings.TrimSpace(plan.Body)
			if body == "" {
				builder.WriteString(mutedStyle.Render("(empty)"))
			} else {
				builder.WriteString(m.renderProseWithMentions(body))
			}
			builder.WriteString("\n\n")
			builder.WriteString(mutedStyle.Render("List Enter/e edits · detail Enter follows · s starts another live sit"))
			return builder.String()
		}
		return mutedStyle.Render("Select prep notes · p to draft")
	default:
		return m.renderDetail()
	}
}
