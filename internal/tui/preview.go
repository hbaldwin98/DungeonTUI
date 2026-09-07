package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

type previewBuf struct {
	Hop    detailHop
	Scroll int
}

func surface(style lipgloss.Style) lipgloss.Style {
	return style.Background(colorSurface)
}

func previewFill() lipgloss.Style {
	return surface(lipgloss.NewStyle().Foreground(colorText))
}

func (m *Model) openPreview(hop detailHop) {
	m.preview = &previewBuf{Hop: hop}
	m.status = "Preview · " + hop.Label + " · Enter open · Esc close"
}

func (m *Model) closePreview() {
	if m.preview == nil {
		return
	}
	m.preview = nil
	m.status = "Closed preview"
}

func (m Model) commitPreview() (tea.Model, tea.Cmd) {
	if m.preview == nil {
		return m, nil
	}
	hop := m.preview.Hop
	m.preview = nil
	return m.jumpToHop(hop)
}

func (m Model) followDetailHop() (tea.Model, tea.Cmd, bool) {
	hops := m.detailHops()
	if len(hops) == 0 {
		return m, nil, false
	}
	hop := hops[clamp(m.historyCursor, 0, len(hops)-1)]
	if hop.Kind == hopBroken {
		m.status = "Missing @" + hop.Label + " · e to edit and fix"
		return m, nil, true
	}
	if hop.Kind == hopWiki {
		if _, ok := m.lookupAny(hop.RecordID); !ok {
			m.status = "Missing @" + hop.Label + " · e to edit and fix"
			return m, nil, true
		}
	}
	if hop.Kind == hopReference {
		if _, ok := m.lookupAny(hop.RecordID); !ok {
			m.status = "Missing reference · import the adventure again"
			return m, nil, true
		}
	}
	m.openPreview(hop)
	return m, nil, true
}

func (m Model) jumpToHop(hop detailHop) (tea.Model, tea.Cmd) {
	switch hop.Kind {
	case hopBroken:
		m.status = "Missing @" + hop.Label + " · e to edit and fix"
		return m, nil
	case hopWiki:
		target, ok := m.lookupAny(hop.RecordID)
		if !ok {
			m.status = "Missing @" + hop.Label + " · e to edit and fix"
			return m, nil
		}
		if fivetools.IsReferenceID(target.ID) {
			m.status = "Adventure reference · preview only"
			return m, nil
		}
		m.selectRecord(target)
		m.layout.Focus = prefs.PaneDetail
		m.status = "Opened " + target.Title
		return m, nil
	case hopReference:
		if target, ok := m.lookupAny(hop.RecordID); ok && target.SourceID != "" {
			m.focusNavKind(NavSources)
			m.selectedSourceID = target.SourceID
			m.selectedHitName = target.Title
			m.syncSourceListCursor()
			m.layout.Focus = prefs.PaneDetail
			m.status = "Opened source · " + target.Source
			return m, nil
		}
		m.status = "Adventure reference · preview only"
		return m, nil
	case hopPrep:
		m.focusPrep(hop.PlanID)
		m.status = "Opened prep · " + hop.Label
		return m, nil
	case hopSession, hopHistory:
		m.focusSession(hop.SessionID)
		m.status = "Opened session · " + hop.Label
		return m, nil
	}
	return m, nil
}

func (m Model) updatePreview(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.preview == nil {
		return m, nil
	}
	switch msg.String() {
	case "esc", "q":
		m.closePreview()
		return m, nil
	case "enter":
		return m.commitPreview()
	case "j", "down", "pgdown":
		m.scrollPreview(1)
		return m, nil
	case "k", "up", "pgup":
		m.scrollPreview(-1)
		return m, nil
	case "home":
		m.preview.Scroll = 0
		return m, nil
	case "end":
		m.preview.Scroll = m.previewMaxScroll()
		return m, nil
	}
	return m, nil
}

func (m *Model) scrollPreview(delta int) {
	if m.preview == nil {
		return
	}
	m.preview.Scroll = clamp(m.preview.Scroll+delta, 0, m.previewMaxScroll())
}

func (m Model) previewWidth() int {
	return min(72, max(48, m.width-10))
}

func (m Model) previewBodyHeight() int {
	return min(14, max(8, m.height/2-6))
}

func (m Model) previewHitBox() (x, y, w, h int) {
	frame := m.previewFrame()
	w = lipgloss.Width(frame)
	h = lipgloss.Height(frame)
	x = max(0, (m.width-w)/2)
	y = max(0, (m.height-h)/2)
	return x, y, w, h
}

func (m Model) previewMaxScroll() int {
	if m.preview == nil {
		return 0
	}
	lines := strings.Split(m.previewBody(m.previewWidth()), "\n")
	return max(0, len(lines)-m.previewBodyHeight())
}

func (m Model) previewBody(width int) string {
	if m.preview == nil {
		return ""
	}
	hop := m.preview.Hop
	switch hop.Kind {
	case hopWiki, hopReference:
		return m.previewWikiBody(hop, width)
	case hopPrep:
		return m.previewPrepBody(hop, width)
	case hopSession, hopHistory:
		return m.previewSessionBody(hop, width)
	default:
		return surface(mutedStyle).Render("Nothing to preview")
	}
}

func (m Model) previewWikiBody(hop detailHop, width int) string {
	record, ok := m.lookupAny(hop.RecordID)
	if !ok {
		return surface(mutedStyle).Render("Missing wiki record")
	}
	fill := previewFill()
	var builder strings.Builder
	kind := string(record.Type)
	if fivetools.IsReferenceID(record.ID) {
		kind = "reference"
	}
	builder.WriteString(surface(typeStyle).Render(kind))
	builder.WriteString(fill.Render("  "))
	builder.WriteString(surface(authorityStyle(record.Authority)).Render(record.Authority.Marker() + " " + record.Authority.Label()))
	builder.WriteString("\n")
	builder.WriteString(surface(titleStyle).Render(record.Title))
	builder.WriteString("\n")
	if record.Summary != "" {
		builder.WriteString(fill.Render(m.renderMarkdown(record.Summary, width)))
		builder.WriteString("\n")
	}
	if len(record.Tags) > 0 {
		builder.WriteString(surface(mutedStyle).Render("#" + strings.Join(record.Tags, "  #")))
		builder.WriteString("\n")
	}
	if strings.TrimSpace(record.Body) != "" {
		builder.WriteString("\n")
		builder.WriteString(fill.Render(m.renderMarkdown(stripRedundantTitleHeading(strings.TrimSpace(record.Body), record.Title), width)))
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func (m Model) previewPrepBody(hop detailHop, width int) string {
	plan := m.plannedByID(hop.PlanID)
	if plan == nil {
		return surface(mutedStyle).Render("Missing prep notes")
	}
	fill := previewFill()
	var builder strings.Builder
	builder.WriteString(surface(typeStyle).Render("PREP"))
	builder.WriteString("\n")
	builder.WriteString(surface(titleStyle).Render(plan.Title))
	builder.WriteString("\n")
	if plan.LocationName != "" {
		builder.WriteString(surface(mutedStyle).Render("Location · " + plan.LocationName))
		builder.WriteString("\n")
	}
	if len(plan.Links) > 0 {
		names := make([]string, 0, len(plan.Links))
		for _, link := range plan.Links {
			name := link.Text
			if target, ok := recordByID(m.workspace.Records, link.RecordID); ok {
				name = target.Title
			}
			names = append(names, name)
		}
		builder.WriteString(surface(mutedStyle).Render("Cast · " + strings.Join(names, ", ")))
		builder.WriteString("\n")
	}
	if strings.TrimSpace(plan.Body) != "" {
		builder.WriteString("\n")
		builder.WriteString(fill.Render(m.renderMarkdown(strings.TrimSpace(plan.Body), width)))
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func (m Model) previewSessionBody(hop detailHop, width int) string {
	session := m.sessionByID(hop.SessionID)
	if session == nil {
		return surface(mutedStyle).Render("Missing session")
	}
	fill := previewFill()
	var builder strings.Builder
	state := "live"
	if session.EndedAt != nil {
		state = "ended"
	}
	builder.WriteString(surface(typeStyle).Render("SESSION"))
	builder.WriteString(fill.Render("  "))
	builder.WriteString(surface(filterStyle).Render(state))
	builder.WriteString("\n")
	builder.WriteString(surface(titleStyle).Render(session.Title))
	builder.WriteString("\n")
	if !session.StartedAt.IsZero() {
		builder.WriteString(surface(mutedStyle).Render(session.StartedAt.Local().Format("2006-01-02")))
		builder.WriteString("\n")
	}
	if session.LocationName != "" {
		builder.WriteString(surface(mutedStyle).Render("Location · " + session.LocationName))
		builder.WriteString("\n")
	}
	folder := domain.SessionFolderPath(*session)
	if folder != "" {
		builder.WriteString(surface(mutedStyle).Render("Folder · " + folder))
		builder.WriteString("\n")
	}
	if len(session.Links) > 0 {
		names := make([]string, 0, len(session.Links))
		for _, link := range session.Links {
			name := link.Text
			if target, ok := recordByID(m.workspace.Records, link.RecordID); ok {
				name = target.Title
			}
			names = append(names, name)
		}
		builder.WriteString(surface(mutedStyle).Render("Cast · " + strings.Join(names, ", ")))
		builder.WriteString("\n")
	}
	if hop.Kind == hopHistory && m.selectedID != "" {
		for _, row := range domain.EntitySessionHistory(m.workspace, m.selectedID) {
			if row.SessionID != hop.SessionID {
				continue
			}
			if len(row.Events) == 0 {
				break
			}
			builder.WriteString("\n")
			builder.WriteString(surface(labelStyle).Render("HISTORY"))
			builder.WriteString("\n")
			for _, event := range row.Events {
				builder.WriteString(surface(mutedStyle).Render("· " + event.Summary))
				builder.WriteString("\n")
			}
			break
		}
	}
	beats := lastTranscriptBeats(session.Entries, 4)
	if len(beats) > 0 {
		builder.WriteString("\n")
		builder.WriteString(surface(labelStyle).Render("LAST BEATS"))
		builder.WriteString("\n")
		for _, beat := range beats {
			builder.WriteString(fill.Render(m.previewWrapped("· "+beat, width)))
			builder.WriteString("\n")
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func lastTranscriptBeats(entries []domain.TranscriptEntry, n int) []string {
	out := make([]string, 0, n)
	for i := len(entries) - 1; i >= 0 && len(out) < n; i-- {
		if entries[i].Undone {
			continue
		}
		text := strings.TrimSpace(entries[i].Text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (m Model) sessionByID(id string) *domain.SessionRecord {
	if id == "" {
		return nil
	}
	for index := range m.workspace.Sessions {
		if m.workspace.Sessions[index].ID == id {
			session := m.workspace.Sessions[index]
			return &session
		}
	}
	return nil
}

func (m Model) previewWrapped(text string, width int) string {
	if width < 12 {
		width = 12
	}
	var parts []string
	for _, para := range strings.Split(text, "\n") {
		if strings.TrimSpace(para) == "" {
			parts = append(parts, "")
			continue
		}
		parts = append(parts, wrapWords(para, width))
	}
	return strings.Join(parts, "\n")
}

func (m Model) previewFrame() string {
	if m.preview == nil {
		return ""
	}
	width := m.previewWidth()
	bodyH := m.previewBodyHeight()
	hop := m.preview.Hop
	kind := strings.ToUpper(string(hop.Kind))
	if hop.Kind == hopHistory {
		kind = "SESSION"
	}
	fill := previewFill()
	var builder strings.Builder
	builder.WriteString(surface(searchTitleStyle).Render("PREVIEW"))
	builder.WriteString(fill.Render("  "))
	builder.WriteString(surface(filterStyle).Render(kind))
	builder.WriteString(fill.Render("  "))
	builder.WriteString(surface(mutedStyle).Render(hop.Prefix + hop.Label))
	builder.WriteString("\n\n")

	bodyLines := strings.Split(m.previewBody(width), "\n")
	scroll := clamp(m.preview.Scroll, 0, m.previewMaxScroll())
	end := min(len(bodyLines), scroll+bodyH)
	visible := bodyLines[scroll:end]
	for len(visible) < bodyH {
		visible = append(visible, "")
	}
	for _, line := range visible {
		builder.WriteString(fill.Width(width).Render(line))
		builder.WriteString("\n")
	}
	builder.WriteString(surface(filterStyle).Render("[open]"))
	builder.WriteString(fill.Render("  "))
	builder.WriteString(surface(mutedStyle).Render("[close]"))
	if m.previewMaxScroll() > 0 {
		builder.WriteString(surface(mutedStyle).Render(fmt.Sprintf("  %d/%d", scroll+1, m.previewMaxScroll()+1)))
	}
	builder.WriteString("\n")
	builder.WriteString(surface(mutedStyle).Render("j/k scroll  Enter jump  Esc dismiss"))

	inner := fill.Width(width).Render(builder.String())
	return searchPanelStyle.
		BorderBackground(colorSurface).
		Width(width).
		Render(inner)
}

func (m Model) renderPreviewOverlay(background string) string {
	overlay := m.previewFrame()
	x, y, _, _ := m.previewHitBox()
	if background == "" {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
		)
	}
	base := lipgloss.NewLayer(background).X(0).Y(0).Z(0)
	float := lipgloss.NewLayer(overlay).X(x).Y(y).Z(1)
	return lipgloss.NewCompositor(base, float).Render()
}

func (m Model) updatePreviewClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	x, y, w, h := m.previewHitBox()
	if msg.X < x || msg.X >= x+w || msg.Y < y || msg.Y >= y+h {
		m.closePreview()
		return m, nil
	}
	innerX := x + 3
	footerY := y + h - 3
	if msg.Y == footerY || msg.Y == footerY-1 {
		rel := msg.X - innerX
		if rel >= 0 && rel < 6 {
			return m.commitPreview()
		}
		if rel >= 8 && rel < 15 {
			m.closePreview()
			return m, nil
		}
	}
	return m, nil
}
