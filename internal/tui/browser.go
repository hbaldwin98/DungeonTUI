package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

type browserRegion struct {
	Pane   prefs.Pane
	MinX   int
	MaxX   int
	MinY   int
	MaxY   int
	Rows   []domain.Record
	Offset int // Y offset of first record row within the pane content
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
	m.typeFilter = record.Type
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

func (m *Model) ensureBrowserSelection() {
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
	panes := m.browserFocusOrder()
	if len(panes) == 0 {
		return
	}
	current := m.layout.Focus
	for index, pane := range panes {
		if pane == current {
			m.setBrowserFocus(panes[(index+1)%len(panes)])
			return
		}
	}
	m.setBrowserFocus(panes[0])
}

func (m Model) browserFocusOrder() []prefs.Pane {
	var panes []prefs.Pane
	for _, pane := range prefs.VisibleLeaves(m.layout.Browser.Root) {
		if _, ok := paneEntityType(pane); ok || pane == prefs.PaneDetail {
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
	if pane == prefs.PaneDetail {
		body = fitLines(m.renderDetail(), innerHeight)
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
				Offset: originY + 4, // border + pad + title + blank
			})
		}
	}
	if pane == prefs.PaneDetail && regions != nil {
		*regions = append(*regions, browserRegion{
			Pane: pane,
			MinX: originX,
			MaxX: originX + width - 1,
			MinY: originY,
			MaxY: originY + height - 1,
		})
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
	if visible[1].IsLeaf() && visible[1].Pane == prefs.PaneDetail {
		return clamp(int(float64(m.width)*root.Ratio), 12, max(12, m.width-12))
	}
	return m.splitWidth(m.width)
}
