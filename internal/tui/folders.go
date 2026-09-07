package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func (m Model) sessionTreeRows() []domain.SessionTreeRow {
	collapsed := m.collapsedFolders
	if collapsed == nil {
		collapsed = map[string]bool{}
	}
	return domain.FlattenSessionTree(m.scopedSessions(), collapsed)
}

func (m *Model) bindSessionTreeRow(row domain.SessionTreeRow) {
	m.selectedID = ""
	m.selectedPlanID = ""
	m.selectedFolderPath = row.Path
	if row.Kind == domain.SessionTreeSession {
		m.selectedSessionID = row.Session.ID
		return
	}
	m.selectedSessionID = ""
}

func (m *Model) applySessionTreeCursor(index int) {
	rows := m.sessionTreeRows()
	if len(rows) == 0 {
		m.cursor = 0
		m.selectedSessionID = ""
		m.selectedFolderPath = ""
		return
	}
	m.cursor = clamp(index, 0, len(rows)-1)
	m.bindSessionTreeRow(rows[m.cursor])
}

func (m *Model) selectFirstSessionTreeRow() {
	rows := m.sessionTreeRows()
	for index, row := range rows {
		if row.Kind == domain.SessionTreeSession {
			m.cursor = index
			m.bindSessionTreeRow(row)
			return
		}
	}
	m.applySessionTreeCursor(0)
}

func (m *Model) syncSessionTreeCursor() {
	rows := m.sessionTreeRows()
	if len(rows) == 0 {
		m.applySessionTreeCursor(0)
		return
	}
	if m.selectedSessionID != "" {
		for index, row := range rows {
			if row.Kind == domain.SessionTreeSession && row.Session.ID == m.selectedSessionID {
				m.cursor = index
				m.bindSessionTreeRow(row)
				return
			}
		}
	}
	if m.selectedFolderPath != "" {
		for index, row := range rows {
			if row.Kind == domain.SessionTreeFolder && row.Path == m.selectedFolderPath {
				m.cursor = index
				m.bindSessionTreeRow(row)
				return
			}
		}
	}
	m.applySessionTreeCursor(m.cursor)
}

func (m *Model) toggleSelectedFolder() {
	if m.currentNav().Kind != NavSessions {
		m.toggleWikiFolder()
		return
	}
	rows := m.sessionTreeRows()
	if len(rows) == 0 {
		return
	}
	row := rows[clamp(m.cursor, 0, len(rows)-1)]
	if row.Kind != domain.SessionTreeFolder {
		return
	}
	if m.collapsedFolders == nil {
		m.collapsedFolders = map[string]bool{}
	}
	if m.collapsedFolders[row.Path] {
		delete(m.collapsedFolders, row.Path)
		m.status = "Expanded " + row.Label
	} else {
		m.collapsedFolders[row.Path] = true
		m.status = "Collapsed " + row.Label
	}
	m.selectedFolderPath = row.Path
	m.selectedSessionID = ""
	m.syncSessionTreeCursor()
}

func (m Model) recordFolderCollapsed(path string) bool {
	if path == "" {
		return false
	}
	if m.collapsedFolders[path] {
		return true
	}
	if m.expandedFolders[path] {
		return false
	}
	return true
}

func (m Model) recordTreeRows() []domain.RecordTreeRow {
	return m.recordTreeRowsFor(m.listRecords())
}

func (m Model) recordTreeRowsFor(records []domain.Record) []domain.RecordTreeRow {
	return domain.FlattenRecordTree(records, m.workspace.Sources, m.recordFolderCollapsed)
}

func (m *Model) bindRecordTreeRow(row domain.RecordTreeRow) {
	m.selectedSessionID = ""
	m.selectedPlanID = ""
	m.selectedFolderPath = row.Path
	if row.Kind == domain.RecordTreeRecord {
		m.selectRecord(row.Record)
		return
	}
	m.selectedID = ""
}

func (m *Model) applyRecordTreeCursor(index int) {
	rows := m.recordTreeRows()
	if len(rows) == 0 {
		m.cursor = 0
		m.selectedID = ""
		m.selectedFolderPath = ""
		return
	}
	m.cursor = clamp(index, 0, len(rows)-1)
	m.bindRecordTreeRow(rows[m.cursor])
}

func (m *Model) toggleWikiFolder() {
	rows := m.recordTreeRows()
	if len(rows) == 0 {
		return
	}
	row := rows[clamp(m.cursor, 0, len(rows)-1)]
	if row.Kind != domain.RecordTreeFolder || row.Path == "" {
		return
	}
	if m.collapsedFolders == nil {
		m.collapsedFolders = map[string]bool{}
	}
	if m.expandedFolders == nil {
		m.expandedFolders = map[string]bool{}
	}
	if m.recordFolderCollapsed(row.Path) {
		m.expandedFolders[row.Path] = true
		delete(m.collapsedFolders, row.Path)
		m.status = "Expanded " + row.Label
	} else {
		delete(m.expandedFolders, row.Path)
		m.collapsedFolders[row.Path] = true
		m.status = "Collapsed " + row.Label
	}
	m.selectedFolderPath = row.Path
	m.selectedID = ""
	m.applyRecordTreeCursor(m.cursor)
}

func (m *Model) expandRecordFolderPath(path string) {
	path = domain.NormalizeFolder(path)
	if path == "" {
		return
	}
	if m.expandedFolders == nil {
		m.expandedFolders = map[string]bool{}
	}
	if m.collapsedFolders == nil {
		m.collapsedFolders = map[string]bool{}
	}
	acc := ""
	for _, part := range strings.Split(path, "/") {
		if acc == "" {
			acc = part
		} else {
			acc += "/" + part
		}
		m.expandedFolders[acc] = true
		delete(m.collapsedFolders, acc)
	}
}

func visibleWindow(n, cursor, maxRows int) (start, end int) {
	if maxRows < 1 {
		maxRows = 1
	}
	if n <= maxRows {
		return 0, n
	}
	start = cursor - maxRows/3
	if start < 0 {
		start = 0
	}
	if start > n-maxRows {
		start = n - maxRows
	}
	return start, start + maxRows
}

func (m Model) renderWikiFolderDetail(path string) string {
	path = domain.NormalizeFolder(path)
	rows := m.recordTreeRows()
	count := 0
	label := path
	for _, row := range rows {
		if row.Kind == domain.RecordTreeFolder && row.Path == path {
			count = row.Count
			label = row.Label
			break
		}
	}
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("FOLDER"))
	builder.WriteString("\n\n")
	builder.WriteString(titleStyle.Render(label))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(strings.ReplaceAll(path, "/", " / ")))
	builder.WriteString(fmt.Sprintf("\n%d records · Enter expand/collapse", count))
	builder.WriteString("\n\n")
	builder.WriteString(mutedStyle.Render("Imported sourcebooks stay in folders so the campaign wiki stays scannable."))
	return builder.String()
}

func (m Model) sessionIDsToFile() []string {
	if m.selectedSessionID != "" {
		return []string{m.selectedSessionID}
	}
	if m.selectedFolderPath == "" {
		return nil
	}
	ids := make([]string, 0)
	for _, session := range m.scopedSessions() {
		if domain.FolderPrefix(domain.SessionFolderPath(session), m.selectedFolderPath) {
			ids = append(ids, session.ID)
		}
	}
	return ids
}

func (m Model) folderForNewSit() string {
	if session := m.selectedSession(); session != nil {
		return domain.NormalizeFolder(session.Folder)
	}
	path := domain.NormalizeFolder(m.selectedFolderPath)
	if domain.IsDerivedFolder(path) {
		return ""
	}
	return path
}

func (m Model) openFolderName() (tea.Model, tea.Cmd) {
	if !m.usesCampaignTree() || m.currentNav().Kind != NavSessions {
		m.status = "Folders live on Sessions · m files a sit or group"
		return m, nil
	}
	if len(m.sessionIDsToFile()) == 0 {
		m.status = "Select a sit or folder, then m to file it"
		return m, nil
	}
	m.namingFolder = true
	m.collectionName.Placeholder = "Greywatch/Crypt · empty unfiles"
	m.collectionName.Prompt = "Folder: "
	prefill := ""
	if session := m.selectedSession(); session != nil {
		prefill = domain.NormalizeFolder(session.Folder)
	} else if !domain.IsDerivedFolder(m.selectedFolderPath) {
		prefill = m.selectedFolderPath
	}
	m.collectionName.SetValue(prefill)
	m.collectionName.SetWidth(max(24, m.width-16))
	m.collectionName.Focus()
	m.status = "File into a folder · Enter save · Esc cancel"
	return m, textinput.Blink
}

func (m *Model) closeFolderName() {
	m.namingFolder = false
	m.collectionName.Blur()
	m.collectionName.Placeholder = "Collection name"
	m.collectionName.Prompt = "Name: "
	m.collectionName.SetValue("")
}

func (m Model) updateFolderName(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeFolderName()
		m.status = "Cancelled folder"
		return m, nil
	case "enter":
		return m.saveSessionFolder()
	}
	var cmd tea.Cmd
	m.collectionName, cmd = m.collectionName.Update(msg)
	return m, cmd
}

func (m Model) saveSessionFolder() (tea.Model, tea.Cmd) {
	ids := m.sessionIDsToFile()
	if len(ids) == 0 {
		m.closeFolderName()
		m.status = "Nothing to file"
		return m, nil
	}
	folder := strings.TrimSpace(m.collectionName.Value())
	m.workspace.Sessions = domain.ApplySessionFolder(m.workspace.Sessions, ids, folder)
	m.closeFolderName()
	m.persistWorkspace()
	m.syncSessionTreeCursor()
	if domain.NormalizeFolder(folder) == "" {
		m.status = fmt.Sprintf("Unfiled %d sit(s) · grouped by month", len(ids))
		return m, nil
	}
	m.status = fmt.Sprintf("Filed %d sit(s) under %s", len(ids), domain.NormalizeFolder(folder))
	return m, nil
}

func (m Model) renderFolderNameOverlay() string {
	width := min(72, max(40, m.width-12))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("SESSION FOLDER"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("slash path · empty returns to month groups"))
	builder.WriteString("\n")
	builder.WriteString(m.collectionName.View())
	builder.WriteString("\n\n")
	builder.WriteString(helpStyle.Render("Enter save  Esc cancel"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
	)
}

func (m Model) renderSessionFolderDetail(path string) string {
	path = domain.NormalizeFolder(path)
	if path == "" {
		return mutedStyle.Render("Select a session")
	}
	var sits []domain.SessionRecord
	for _, session := range m.scopedSessions() {
		if domain.FolderPrefix(domain.SessionFolderPath(session), path) {
			sits = append(sits, session)
		}
	}
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("FOLDER"))
	builder.WriteString("\n\n")
	builder.WriteString(titleStyle.Render(strings.ReplaceAll(path, "/", " / ")))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(fmt.Sprintf("%d sit(s)", len(sits))))
	if len(sits) > 0 {
		builder.WriteString("\n")
		for _, session := range sits {
			state := "live"
			if session.EndedAt != nil {
				state = "ended"
			}
			builder.WriteString(fmt.Sprintf("\n  %s · %s", session.Title, state))
		}
	}
	builder.WriteString("\n\n")
	builder.WriteString(mutedStyle.Render("Enter collapse · m files this group · s starts live here"))
	return builder.String()
}
