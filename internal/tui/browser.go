package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

type browserRegion struct {
	Pane        prefs.Pane
	MinX        int
	MaxX        int
	MinY        int
	MaxY        int
	Rows        []domain.Record
	NavEntries  []navEntry
	SessionRows []domain.SessionRecord
	PlanRows    []domain.PlannedNotes
	Offset      int // Y offset of first record row within the pane content
}

func paneEntityType(pane prefs.Pane) (domain.EntityType, bool) {
	switch pane {
	case prefs.PaneNPC:
		return domain.NPC, true
	case prefs.PaneCharacter:
		return domain.Character, true
	case prefs.PaneLocation:
		return domain.Location, true
	case prefs.PaneFaction:
		return domain.Faction, true
	case prefs.PaneItem:
		return domain.Item, true
	case prefs.PaneCreature:
		return domain.Creature, true
	case prefs.PaneThread:
		return domain.Thread, true
	case prefs.PaneNote:
		return domain.Note, true
	case prefs.PaneEvent:
		return domain.Event, true
	case prefs.PaneList:
		return "", true
	default:
		return "", false
	}
}

func entityTypePane(entityType domain.EntityType) prefs.Pane {
	switch entityType {
	case domain.NPC:
		return prefs.PaneNPC
	case domain.Character:
		return prefs.PaneCharacter
	case domain.Location:
		return prefs.PaneLocation
	case domain.Faction:
		return prefs.PaneFaction
	case domain.Item:
		return prefs.PaneItem
	case domain.Creature:
		return prefs.PaneCreature
	case domain.Thread:
		return prefs.PaneThread
	case domain.Note:
		return prefs.PaneNote
	case domain.Event:
		return prefs.PaneEvent
	default:
		return prefs.PaneList
	}
}

func (m Model) recordsForPane(pane prefs.Pane) []domain.Record {
	if pane == prefs.PaneList && m.usesCampaignTree() {
		return m.listRecords()
	}
	entityType, ok := paneEntityType(pane)
	if !ok {
		return nil
	}
	if pane == prefs.PaneList {
		return m.scopedRecords(false)
	}
	out := make([]domain.Record, 0)
	for _, record := range m.scopedRecords(false) {
		if record.Type == entityType {
			out = append(out, record)
		}
	}
	return out
}

func (m Model) selectedRecord() *domain.Record {
	if m.selectedID == "" {
		return nil
	}
	for index := range m.workspace.Records {
		if m.workspace.Records[index].ID == m.selectedID {
			record := m.workspace.Records[index]
			return &record
		}
	}
	return nil
}

func (m *Model) selectRecord(record domain.Record) {
	m.selectedID = record.ID
	m.selectedSessionID = ""
	m.selectedPlanID = ""
	m.historyCursor = 0
	m.historyExpanded = ""
	m.typeFilter = record.Type
	if m.usesCampaignTree() {
		m.focusNavType(record.Type)
		// Keep current pane focus — do not steal focus onto the list.
		records := m.listRecords()
		for index, item := range records {
			if item.ID == record.ID {
				m.cursor = index
				return
			}
		}
		m.cursor = 0
		return
	}
	pane := entityTypePane(record.Type)
	if prefs.FindVisibleLeaf(m.layout.Browser.Root, pane) {
		m.layout.Focus = pane
	}
	records := m.recordsForPane(m.layout.Focus)
	for index, item := range records {
		if item.ID == record.ID {
			m.cursor = index
			return
		}
	}
	m.cursor = 0
}

func (m *Model) focusNavType(entityType domain.EntityType) {
	for index, entry := range m.navEntries() {
		if entry.Kind == NavType && entry.Type == entityType {
			m.navCursor = index
			m.navKind = NavType
			m.navType = entityType
			m.typeFilter = entityType
			return
		}
	}
}

func (m *Model) ensureBrowserSelection() {
	if m.usesCampaignTree() {
		if m.navKind == "" {
			chosen := 2 // NPCs after Sessions / Prep
			for index, entry := range m.navEntries() {
				if entry.Kind != NavType {
					continue
				}
				m.navCursor = index
				m.navKind = entry.Kind
				m.navType = entry.Type
				m.typeFilter = entry.Type
				if len(m.listRecords()) > 0 {
					chosen = index
					break
				}
			}
			m.navKind = ""
			m.setNavCursor(chosen)
		}
		// Default focus stays on the campaign tree, even when the list has items.
		if m.layout.Focus == prefs.PaneInput || m.layout.Focus == "" || m.layout.Focus == prefs.PaneNPC {
			m.layout.Focus = prefs.PaneNav
		}
		return
	}
	if m.selectedRecord() != nil {
		if m.layout.Focus == prefs.PaneInput || m.layout.Focus == "" {
			m.layout.Focus = entityTypePane(m.selectedRecord().Type)
		}
		return
	}
	for _, pane := range prefs.VisibleLeaves(m.layout.Browser.Root) {
		records := m.recordsForPane(pane)
		if len(records) > 0 {
			m.selectRecord(records[0])
			return
		}
	}
}

func (m *Model) setBrowserFocus(pane prefs.Pane) {
	m.layout.Focus = pane
	if pane == prefs.PaneNav {
		return
	}
	if pane == prefs.PaneList && m.usesCampaignTree() {
		switch m.currentNav().Kind {
		case NavSessions:
			sessions := m.scopedSessions()
			if len(sessions) == 0 {
				return
			}
			for index, session := range sessions {
				if session.ID == m.selectedSessionID {
					m.cursor = index
					return
				}
			}
			m.cursor = clamp(m.cursor, 0, len(sessions)-1)
			m.selectedSessionID = sessions[m.cursor].ID
		case NavPrep:
			plans := m.scopedPlannedNotes()
			if len(plans) == 0 {
				return
			}
			for index, plan := range plans {
				if plan.ID == m.selectedPlanID {
					m.cursor = index
					return
				}
			}
			m.cursor = clamp(m.cursor, 0, len(plans)-1)
			m.selectedPlanID = plans[m.cursor].ID
		default:
			records := m.listRecords()
			if len(records) == 0 {
				return
			}
			for index, record := range records {
				if record.ID == m.selectedID {
					m.cursor = index
					return
				}
			}
			m.selectRecord(records[clamp(m.cursor, 0, len(records)-1)])
		}
		return
	}
	if _, ok := paneEntityType(pane); ok {
		if entityType, typed := paneEntityType(pane); typed && pane != prefs.PaneList {
			m.typeFilter = entityType
		}
		records := m.recordsForPane(pane)
		if len(records) == 0 {
			return
		}
		for index, record := range records {
			if record.ID == m.selectedID {
				m.cursor = index
				return
			}
		}
		m.selectRecord(records[clamp(m.cursor, 0, len(records)-1)])
	}
}

func (m *Model) cycleBrowserFocus() {
	m.cycleBrowserFocusBy(1)
}

func (m *Model) cycleBrowserFocusBack() {
	m.cycleBrowserFocusBy(-1)
}

func (m *Model) cycleBrowserFocusBy(delta int) {
	panes := m.browserFocusOrder()
	if len(panes) == 0 {
		return
	}
	current := m.layout.Focus
	for index, pane := range panes {
		if pane == current {
			next := (index + delta) % len(panes)
			if next < 0 {
				next += len(panes)
			}
			m.setBrowserFocus(panes[next])
			return
		}
	}
	m.setBrowserFocus(panes[0])
}

func (m Model) browserFocusOrder() []prefs.Pane {
	var panes []prefs.Pane
	for _, pane := range prefs.VisibleLeaves(m.layout.Browser.Root) {
		if pane == prefs.PaneNav || pane == prefs.PaneDetail {
			panes = append(panes, pane)
			continue
		}
		if _, ok := paneEntityType(pane); ok {
			panes = append(panes, pane)
		}
	}
	return panes
}

func (m Model) panelStyleForBrowser(pane prefs.Pane) lipgloss.Style {
	if m.layout.Focus == pane {
		return focusedPanelStyle
	}
	return panelStyle
}

func (m Model) renderBrowserTree(node prefs.Node, width, height, originX, originY int, regions *[]browserRegion) string {
	if height < 1 || width < 1 {
		return strings.Repeat(" ", width)
	}
	if node.IsLeaf() {
		if !node.IsVisible() {
			return ""
		}
		return m.renderBrowserLeaf(node.Pane, width, height, originX, originY, regions)
	}
	visible := visibleBrowserChildren(node)
	if len(visible) == 0 {
		return strings.Repeat(" ", max(0, width*height))
	}
	if len(visible) == 1 {
		return m.renderBrowserTree(visible[0], width, height, originX, originY, regions)
	}
	first := visible[0]
	second := visible[1]
	ratio := node.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	if node.Axis == prefs.AxisHorizontal {
		topHeight := clamp(int(float64(height)*ratio), 3, max(3, height-3))
		bottomHeight := max(1, height-topHeight)
		top := m.renderBrowserTree(first, width, topHeight, originX, originY, regions)
		bottom := m.renderBrowserTree(second, width, bottomHeight, originX, originY+topHeight, regions)
		return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	}
	leftWidth := clamp(int(float64(width)*ratio), 12, max(12, width-12))
	rightWidth := max(1, width-leftWidth)
	left := m.renderBrowserTree(first, leftWidth, height, originX, originY, regions)
	right := m.renderBrowserTree(second, rightWidth, height, originX+leftWidth, originY, regions)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func visibleBrowserChildren(node prefs.Node) []prefs.Node {
	if node.IsLeaf() {
		if node.IsVisible() {
			return []prefs.Node{node}
		}
		return nil
	}
	var out []prefs.Node
	for _, child := range node.Children {
		if child.IsLeaf() {
			if child.IsVisible() {
				out = append(out, child)
			}
			continue
		}
		if len(visibleBrowserChildren(child)) > 0 {
			out = append(out, child)
		}
	}
	return out
}

func (m Model) renderBrowserLeaf(pane prefs.Pane, width, height, originX, originY int, regions *[]browserRegion) string {
	style := m.panelStyleForBrowser(pane).Width(width).Height(height).MaxHeight(height)
	innerHeight := panelInnerHeight(height)
	var body string
	switch pane {
	case prefs.PaneDetail:
		if m.usesCampaignTree() {
			body = fitLines(m.renderTreeDetail(), innerHeight)
		} else {
			body = fitLines(m.renderDetail(), innerHeight)
		}
		if regions != nil {
			*regions = append(*regions, browserRegion{
				Pane: pane,
				MinX: originX,
				MaxX: originX + width - 1,
				MinY: originY,
				MaxY: originY + height - 1,
			})
		}
	case prefs.PaneNav:
		body = fitLines(m.renderNavTree(), innerHeight)
		if regions != nil {
			*regions = append(*regions, browserRegion{
				Pane:       pane,
				MinX:       originX,
				MaxX:       originX + width - 1,
				MinY:       originY,
				MaxY:       originY + height - 1,
				NavEntries: m.navEntries(),
				Offset:     originY + 4,
			})
		}
	case prefs.PaneList:
		if m.usesCampaignTree() {
			body = fitLines(m.renderListPane(), innerHeight)
			region := browserRegion{
				Pane:   pane,
				MinX:   originX,
				MaxX:   originX + width - 1,
				MinY:   originY,
				MaxY:   originY + height - 1,
				Offset: originY + 4,
			}
			switch m.currentNav().Kind {
			case NavSessions:
				region.SessionRows = m.scopedSessions()
			case NavPrep:
				region.PlanRows = m.scopedPlannedNotes()
			default:
				region.Rows = m.listRecords()
			}
			if regions != nil {
				*regions = append(*regions, region)
			}
		} else {
			records := m.recordsForPane(pane)
			body = fitLines(m.renderTypeSection(pane, records), innerHeight)
			if regions != nil {
				*regions = append(*regions, browserRegion{
					Pane:   pane,
					MinX:   originX,
					MaxX:   originX + width - 1,
					MinY:   originY,
					MaxY:   originY + height - 1,
					Rows:   records,
					Offset: originY + 4,
				})
			}
		}
	default:
		records := m.recordsForPane(pane)
		body = fitLines(m.renderTypeSection(pane, records), innerHeight)
		if regions != nil {
			*regions = append(*regions, browserRegion{
				Pane:   pane,
				MinX:   originX,
				MaxX:   originX + width - 1,
				MinY:   originY,
				MaxY:   originY + height - 1,
				Rows:   records,
				Offset: originY + 4, // border + pad + title + blank
			})
		}
	}
	return style.Render(body)
}

func (m Model) renderTypeSection(pane prefs.Pane, records []domain.Record) string {
	title := strings.ToUpper(string(pane))
	if entityType, ok := paneEntityType(pane); ok && pane != prefs.PaneList {
		title = string(entityType)
	}
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render(title))
	focusMark := ""
	if m.layout.Focus == pane {
		focusMark = " · focused"
	}
	builder.WriteString(mutedStyle.Render(focusMark))
	builder.WriteString("\n\n")
	if len(records) == 0 {
		builder.WriteString(mutedStyle.Render("  —"))
		return builder.String()
	}
	for index, record := range records {
		cursor := "  "
		style := normalItemStyle
		if record.ID == m.selectedID || (m.layout.Focus == pane && index == m.cursor) {
			cursor = "▸ "
			style = selectedItemStyle
		}
		line := fmt.Sprintf("%s%s %s", cursor, record.Authority.Marker(), record.Title)
		builder.WriteString(style.Render(line))
		builder.WriteRune('\n')
	}
	return builder.String()
}

func (m Model) browserRegions() []browserRegion {
	var regions []browserRegion
	bodyHeight := max(1, m.height-2)
	_ = m.renderBrowserTree(m.layout.Browser.Root, max(1, m.width), bodyHeight, 0, 1, &regions)
	return regions
}

func (m Model) browserDetailGutterX() int {
	root := m.layout.Browser.Root
	if root.IsLeaf() || root.Axis != prefs.AxisVertical {
		return m.splitWidth(m.width)
	}
	visible := visibleBrowserChildren(root)
	if len(visible) < 2 {
		return m.splitWidth(m.width)
	}
	// Prefer the nested list|detail gutter when present (campaign tree).
	if !visible[1].IsLeaf() && visible[1].Axis == prefs.AxisVertical {
		inner := visibleBrowserChildren(visible[1])
		if len(inner) >= 2 && inner[1].IsLeaf() && inner[1].Pane == prefs.PaneDetail {
			left := clamp(int(float64(m.width)*root.Ratio), 10, max(10, m.width-20))
			remain := max(1, m.width-left)
			return left + clamp(int(float64(remain)*visible[1].Ratio), 12, max(12, remain-12))
		}
	}
	if visible[1].IsLeaf() && visible[1].Pane == prefs.PaneDetail {
		return clamp(int(float64(m.width)*root.Ratio), 12, max(12, m.width-12))
	}
	return m.splitWidth(m.width)
}

func (m Model) browserNavGutterX() int {
	root := m.layout.Browser.Root
	if root.IsLeaf() || root.Axis != prefs.AxisVertical {
		return -1
	}
	visible := visibleBrowserChildren(root)
	if len(visible) < 2 {
		return -1
	}
	if visible[0].IsLeaf() && visible[0].Pane == prefs.PaneNav {
		return m.splitWidth(m.width)
	}
	return -1
}
