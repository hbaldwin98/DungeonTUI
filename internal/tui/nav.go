package tui

import (
	"fmt"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/ingest/fivetools"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

// NavKind identifies a first-class branch in the in-campaign tree.
type NavKind string

const (
	NavHome     NavKind = "home"
	NavSessions NavKind = "sessions"
	NavPrep     NavKind = "prep"
	NavSources  NavKind = "sources"
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
		{Kind: NavSources, Label: "Sources"},
		{Kind: NavType, Type: domain.NPC, Label: "NPCs"},
		{Kind: NavType, Type: domain.Character, Label: "Characters"},
		{Kind: NavType, Type: domain.Location, Label: "Locations"},
		{Kind: NavType, Type: domain.Faction, Label: "Factions"},
		{Kind: NavType, Type: domain.Item, Label: "Items"},
		{Kind: NavType, Type: domain.Creature, Label: "Creatures"},
		{Kind: NavType, Type: domain.Thread, Label: "Threads"},
		{Kind: NavType, Type: domain.Event, Label: "Timeline"},
		{Kind: NavType, Type: domain.Rule, Label: "Rules"},
		{Kind: NavType, Type: domain.Note, Label: "Knowledge"},
		{Kind: NavHome, Label: "Home"},
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
	m.selectedSourceID = ""
	m.selectedHitName = ""
	m.selectedChapter = ""
	switch entry.Kind {
	case NavHome:
		m.typeFilter = ""
	case NavType:
		m.typeFilter = entry.Type
		rows := m.recordTreeRows()
		if len(rows) > 0 {
			m.applyRecordTreeCursor(0)
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
	case NavSources:
		m.typeFilter = ""
		m.selectFirstSourceRow()
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

type sourceRowKind string

const (
	sourceRowBook    sourceRowKind = "book"
	sourceRowChapter sourceRowKind = "chapter"
	sourceRowHit     sourceRowKind = "hit"
)

type sourceListRow struct {
	Kind    sourceRowKind
	Doc     domain.SourceDocument
	Chapter string
	Path    string
	Hit     fivetools.AdventureHit
	Count   int
}

func sourceBookPath(id string) string {
	return "srcbook:" + id
}

func sourceChapterPath(id, chapter string) string {
	return "srcbook:" + id + "/" + chapter
}

func (m Model) sourceFolderCollapsed(path string, kind sourceRowKind) bool {
	if path == "" {
		return false
	}
	if m.collapsedFolders[path] {
		return true
	}
	if m.expandedFolders[path] {
		return false
	}
	return kind == sourceRowChapter
}

func (m Model) sourceListRows() []sourceListRow {
	docs := m.scopedReferenceSources()
	out := make([]sourceListRow, 0)
	for _, doc := range docs {
		book, ok := m.adventureBookBySource(doc.ID)
		chapters := adventureChapterGroups(book)
		bookPath := sourceBookPath(doc.ID)
		out = append(out, sourceListRow{
			Kind:  sourceRowBook,
			Doc:   doc,
			Path:  bookPath,
			Count: len(chapters),
		})
		if !ok || m.sourceFolderCollapsed(bookPath, sourceRowBook) {
			continue
		}
		for _, chapter := range chapters {
			chPath := sourceChapterPath(doc.ID, chapter.Name)
			out = append(out, sourceListRow{
				Kind:    sourceRowChapter,
				Doc:     doc,
				Chapter: chapter.Name,
				Path:    chPath,
				Hit:     chapter.Hit,
				Count:   len(chapter.Kids),
			})
			if m.sourceFolderCollapsed(chPath, sourceRowChapter) {
				continue
			}
			for _, hit := range chapter.Kids {
				out = append(out, sourceListRow{
					Kind:    sourceRowHit,
					Doc:     doc,
					Chapter: chapter.Name,
					Path:    chPath,
					Hit:     hit,
				})
			}
		}
	}
	return out
}

type adventureChapterGroup struct {
	Name string
	Hit  fivetools.AdventureHit
	Kids []fivetools.AdventureHit
}

func adventureChapterGroups(book fivetools.AdventureBook) []adventureChapterGroup {
	if book.ID == "" && len(book.Hits) == 0 && len(book.TOC) == 0 {
		return nil
	}
	order := append([]string{}, book.TOC...)
	index := map[string]int{}
	groups := make([]adventureChapterGroup, 0, len(order))
	ensure := func(name string) int {
		name = strings.TrimSpace(name)
		if name == "" {
			name = "Untitled"
		}
		key := strings.ToLower(name)
		if at, ok := index[key]; ok {
			return at
		}
		index[key] = len(groups)
		groups = append(groups, adventureChapterGroup{Name: name})
		return len(groups) - 1
	}
	for _, name := range order {
		ensure(name)
	}
	for _, hit := range book.Hits {
		chapter := strings.TrimSpace(hit.Chapter)
		if chapter == "" {
			chapter = hit.Name
		}
		at := ensure(chapter)
		if strings.EqualFold(hit.Name, chapter) {
			if groups[at].Hit.Name == "" {
				groups[at].Hit = hit
			}
			continue
		}
		groups[at].Kids = append(groups[at].Kids, hit)
	}
	return groups
}

func (m *Model) bindSourceListRow(row sourceListRow) {
	m.selectedSourceID = row.Doc.ID
	m.selectedChapter = row.Chapter
	m.selectedFolderPath = row.Path
	switch row.Kind {
	case sourceRowHit:
		m.selectedHitName = row.Hit.Name
	case sourceRowChapter:
		m.selectedHitName = row.Chapter
	default:
		m.selectedHitName = ""
		m.selectedChapter = ""
	}
	m.selectedID = ""
	m.selectedSessionID = ""
	m.selectedPlanID = ""
}

func (m *Model) selectFirstSourceRow() {
	rows := m.sourceListRows()
	if len(rows) == 0 {
		return
	}
	for index, row := range rows {
		if row.Kind == sourceRowChapter {
			m.cursor = index
			m.bindSourceListRow(row)
			return
		}
	}
	m.bindSourceListRow(rows[0])
	m.cursor = 0
}

func (m *Model) syncSourceListCursor() {
	rows := m.sourceListRows()
	if len(rows) == 0 {
		return
	}
	for index, row := range rows {
		if row.Doc.ID != m.selectedSourceID {
			continue
		}
		switch {
		case m.selectedHitName != "" && row.Kind == sourceRowHit && strings.EqualFold(row.Hit.Name, m.selectedHitName):
			m.cursor = index
			return
		case m.selectedHitName != "" && row.Kind == sourceRowChapter && strings.EqualFold(row.Chapter, m.selectedHitName):
			m.cursor = index
			return
		case m.selectedHitName == "" && m.selectedChapter != "" && row.Kind == sourceRowChapter && strings.EqualFold(row.Chapter, m.selectedChapter):
			m.cursor = index
			return
		case m.selectedHitName == "" && m.selectedChapter == "" && row.Kind == sourceRowBook:
			m.cursor = index
			return
		}
	}
	m.cursor = clamp(m.cursor, 0, len(rows)-1)
	m.bindSourceListRow(rows[m.cursor])
}

func (m *Model) toggleSourceFolder() {
	rows := m.sourceListRows()
	if len(rows) == 0 {
		return
	}
	row := rows[clamp(m.cursor, 0, len(rows)-1)]
	if row.Kind != sourceRowBook && row.Kind != sourceRowChapter {
		return
	}
	if m.collapsedFolders == nil {
		m.collapsedFolders = map[string]bool{}
	}
	if m.expandedFolders == nil {
		m.expandedFolders = map[string]bool{}
	}
	if m.sourceFolderCollapsed(row.Path, row.Kind) {
		delete(m.collapsedFolders, row.Path)
		m.expandedFolders[row.Path] = true
	} else {
		delete(m.expandedFolders, row.Path)
		m.collapsedFolders[row.Path] = true
	}
	m.bindSourceListRow(row)
	m.syncSourceListCursor()
}

func (m *Model) expandSourceChapter(sourceID, chapter string) {
	if sourceID == "" {
		return
	}
	if m.expandedFolders == nil {
		m.expandedFolders = map[string]bool{}
	}
	if m.collapsedFolders == nil {
		m.collapsedFolders = map[string]bool{}
	}
	bookPath := sourceBookPath(sourceID)
	delete(m.collapsedFolders, bookPath)
	m.expandedFolders[bookPath] = true
	if chapter == "" {
		return
	}
	chPath := sourceChapterPath(sourceID, chapter)
	delete(m.collapsedFolders, chPath)
	m.expandedFolders[chPath] = true
}

func (m Model) selectedSourceHit() (fivetools.AdventureHit, bool) {
	if m.selectedHitName == "" {
		return fivetools.AdventureHit{}, false
	}
	book, ok := m.adventureBookBySource(m.selectedSourceID)
	if !ok {
		return fivetools.AdventureHit{}, false
	}
	return book.Lookup(m.selectedHitName)
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

func (m Model) renderListPane(maxRows int) string {
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
	case NavHome:
		return m.renderHomeActions()
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
	case NavSources:
		rows := m.sourceListRows()
		if len(rows) == 0 {
			builder.WriteString(mutedStyle.Render("  — no adventures enabled · I to import"))
			return builder.String()
		}
		visible := max(1, maxRows-2)
		start, end := visibleWindow(len(rows), m.cursor, visible)
		for index := start; index < end; index++ {
			row := rows[index]
			cursor := "  "
			style := normalItemStyle
			selected := m.layout.Focus == prefs.PaneList && index == m.cursor
			if !selected {
				switch row.Kind {
				case sourceRowHit:
					selected = row.Doc.ID == m.selectedSourceID && strings.EqualFold(row.Hit.Name, m.selectedHitName)
				case sourceRowChapter:
					selected = row.Doc.ID == m.selectedSourceID && m.selectedHitName == row.Chapter && m.selectedChapter == row.Chapter
				default:
					selected = row.Doc.ID == m.selectedSourceID && m.selectedHitName == "" && m.selectedChapter == ""
				}
			}
			if selected {
				cursor = "▸ "
				style = selectedItemStyle
			}
			indent := 0
			var line string
			switch row.Kind {
			case sourceRowChapter:
				indent = 2
				glyph := "▾"
				if m.sourceFolderCollapsed(row.Path, row.Kind) {
					glyph = "▸"
				}
				line = fmt.Sprintf("%s%s %s", cursor, glyph, row.Chapter)
				if row.Count > 0 {
					line += fmt.Sprintf(" · %d", row.Count)
				}
			case sourceRowHit:
				indent = 4
				line = cursor + row.Hit.Name
			default:
				glyph := "▾"
				if m.sourceFolderCollapsed(row.Path, row.Kind) {
					glyph = "▸"
				}
				line = fmt.Sprintf("%s%s %s", cursor, glyph, row.Doc.Title)
				if row.Count > 0 {
					line += fmt.Sprintf(" · %d chapters", row.Count)
				}
			}
			builder.WriteString(style.PaddingLeft(indent).Render(line))
			builder.WriteRune('\n')
		}
	default:
		rows := m.recordTreeRows()
		if len(rows) == 0 {
			builder.WriteString(mutedStyle.Render("  —"))
			return builder.String()
		}
		visible := max(1, maxRows-2)
		start, end := visibleWindow(len(rows), m.cursor, visible)
		for index := start; index < end; index++ {
			row := rows[index]
			cursor := "  "
			style := normalItemStyle
			selected := (m.layout.Focus == prefs.PaneList && index == m.cursor) ||
				(row.Kind == domain.RecordTreeRecord && row.Record.ID == m.selectedID) ||
				(row.Kind == domain.RecordTreeFolder && m.selectedID == "" && row.Path == m.selectedFolderPath)
			if selected {
				cursor = "▸ "
				style = selectedItemStyle
			}
			var rest string
			if row.Kind == domain.RecordTreeFolder {
				glyph := "▾"
				if m.recordFolderCollapsed(row.Path) {
					glyph = "▸"
				}
				rest = fmt.Sprintf("%s %s · %d", glyph, row.Label, row.Count)
			} else {
				rest = fmt.Sprintf("%s %s", row.Record.Authority.Marker(), row.Label)
			}
			builder.WriteString(style.PaddingLeft(row.Depth * 2).Render(cursor + rest))
			builder.WriteRune('\n')
		}
	}
	return builder.String()
}

func (m Model) renderTreeDetail() string {
	return m.renderTreeDetailWidth(m.defaultMarkdownWidth())
}

func (m Model) renderTreeDetailWidth(width int) string {
	entry := m.currentNav()
	switch entry.Kind {
	case NavHome:
		return m.renderCampaignHome()
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
			builder.WriteString(m.renderDetailTrail("Sessions", domain.SessionFolderPath(session), session.Title))
			builder.WriteString("\n")
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
			builder.WriteString(mutedStyle.Render("List Enter playback · detail j/k select · Enter preview · Enter again follow · m files · s live · d deletes"))
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
			builder.WriteString(m.renderDetailTrail("Prep", "", plan.Title))
			builder.WriteString("\n")
			builder.WriteString(titleStyle.Render(plan.Title))
			if plan.LocationName != "" {
				builder.WriteString("\n")
				builder.WriteString("Location: " + plan.LocationName)
			}
			builder.WriteString("\n\n")
			builder.WriteString(m.renderCastHops())
			builder.WriteString("\n")
			builder.WriteString(m.renderRunSheet(plan, width))
			builder.WriteString("\n\n")
			builder.WriteString(mutedStyle.Render("List Enter/e edits · detail j/k select · Enter preview · Enter again follow · s starts another live sit"))
			return builder.String()
		}
		return mutedStyle.Render("Select prep notes · p to draft")
	case NavSources:
		return m.renderAdventureReader(width)
	default:
		if m.selectedID == "" && m.selectedFolderPath != "" {
			return m.renderWikiFolderDetail(m.selectedFolderPath)
		}
		return m.renderDetailWidth(width)
	}
}

func (m Model) renderAdventureReader(width int) string {
	if m.selectedSourceID == "" {
		return mutedStyle.Render("Select an adventure")
	}
	title := m.selectedSourceID
	for _, doc := range m.scopedReferenceSources() {
		if doc.ID == m.selectedSourceID {
			title = doc.Title
			break
		}
	}
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("SOURCE"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("reference"))
	builder.WriteString("\n")
	book, ok := m.adventureBookBySource(m.selectedSourceID)
	if !ok {
		builder.WriteString(m.renderDetailTrail("Sources", "", title))
		builder.WriteString("\n\n")
		builder.WriteString(titleStyle.Render(title))
		builder.WriteString("\n\n")
		builder.WriteString(mutedStyle.Render("Cached adventure text is missing · import it again from 5e.tools"))
		return builder.String()
	}
	hit, hitOK := m.selectedSourceHit()
	if hitOK {
		folders := []string{title}
		if hit.Chapter != "" {
			folders = append(folders, hit.Chapter)
		}
		builder.WriteString(m.renderDetailTrailParts("Sources", folders, hit.Name))
		builder.WriteString("\n\n")
		builder.WriteString(titleStyle.Render(hit.Name))
		builder.WriteString("\n")
		loc := title
		if hit.Heading != "" && !strings.EqualFold(hit.Heading, hit.Name) {
			loc += " · " + hit.Heading
		}
		builder.WriteString(mutedStyle.Render(loc))
		if len(hit.Aliases) > 0 {
			builder.WriteString("\n")
			builder.WriteString(mutedStyle.Render("also  " + strings.Join(hit.Aliases, " · ")))
		}
		builder.WriteString("\n\n")
		body := strings.TrimSpace(hit.Body)
		if body == "" {
			builder.WriteString(mutedStyle.Render("(empty)"))
		} else {
			builder.WriteString(m.renderMarkdown(body, width))
		}
		return builder.String()
	}
	builder.WriteString(m.renderDetailTrail("Sources", "", title))
	builder.WriteString("\n\n")
	builder.WriteString(titleStyle.Render(title))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(fmt.Sprintf("%d names · j/k the list to read one", len(book.Hits))))
	builder.WriteString("\n\n")
	if len(book.TOC) > 0 {
		builder.WriteString(labelStyle.Render("CONTENTS"))
		builder.WriteString("\n")
		for _, name := range book.TOC {
			builder.WriteString(mutedStyle.Render("  " + name))
			builder.WriteRune('\n')
		}
	}
	return builder.String()
}
