package tui

import (
	"fmt"
	"reflect"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

const maxBrowserHistoryEntries = 32

type browserLocation struct {
	cursor             int
	selectedID         string
	selectedSessionID  string
	selectedPlanID     string
	selectedFolderPath string
	selectedSourceID   string
	selectedHitName    string
	selectedChapter    string
	navCursor          int
	navKind            NavKind
	navType            domain.EntityType
	typeFilter         domain.EntityType
	tagFilter          string
	listScope          searchsvc.Scope
	collectionFilter   string
	lastCollectionID   string
	focus              prefs.Pane
	historyCursor      int
	detailOffset       int
	collapsedFolders   map[string]bool
	expandedFolders    map[string]bool
	search             searchLocation
}

// searchLocation is the search overlay's position, so inspecting a result and
// coming back lands on the same query, scope, and row.
type searchLocation struct {
	open         bool
	query        string
	scope        searchsvc.Scope
	selected     int
	includeIdeas bool
}

func (m Model) snapshotBrowserLocation() browserLocation {
	location := browserLocation{
		cursor:             m.cursor,
		selectedID:         m.selectedID,
		selectedSessionID:  m.selectedSessionID,
		selectedPlanID:     m.selectedPlanID,
		selectedFolderPath: m.selectedFolderPath,
		selectedSourceID:   m.selectedSourceID,
		selectedHitName:    m.selectedHitName,
		selectedChapter:    m.selectedChapter,
		navCursor:          m.navCursor,
		navKind:            m.navKind,
		navType:            m.navType,
		typeFilter:         m.typeFilter,
		tagFilter:          m.tagFilter,
		listScope:          m.listScope,
		collectionFilter:   m.collectionFilter,
		lastCollectionID:   m.lastCollectionID,
		focus:              m.layout.Focus,
		historyCursor:      m.historyCursor,
		collapsedFolders:   cloneBoolMap(m.collapsedFolders),
		expandedFolders:    cloneBoolMap(m.expandedFolders),
		search:             m.snapshotSearchLocation(),
	}
	if m.detailView != nil {
		location.detailOffset = m.detailView.offset
	}
	return location
}

func (m Model) snapshotSearchLocation() searchLocation {
	return searchLocation{
		open:         m.searching,
		query:        m.searchInput.Value(),
		scope:        m.searchScope,
		selected:     m.selected,
		includeIdeas: m.includeIdeas,
	}
}

// restoreSearchLocation reopens the search overlay exactly where it was left.
// A location taken outside search restores nothing.
func (m *Model) restoreSearchLocation(location searchLocation) tea.Cmd {
	if !location.open {
		return nil
	}
	m.searching = true
	m.searchScope = location.scope
	m.includeIdeas = location.includeIdeas
	m.searchInput.SetValue(location.query)
	if location.scope == searchsvc.RulesReference {
		m.selected = location.selected
		return tea.Batch(m.searchInput.Focus(), m.scheduleRulesSearch())
	}
	m.rulesNote = ""
	m.refreshResults()
	m.selected = clamp(location.selected, 0, max(0, len(m.results)-1))
	return m.searchInput.Focus()
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	if len(source) == 0 {
		return map[string]bool{}
	}
	clone := make(map[string]bool, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func (m *Model) pushBrowserLocation() {
	m.pushBrowserLocationSnapshot(m.snapshotBrowserLocation())
}

// pushBrowserLocationSnapshot records a location captured earlier, for callers
// that must snapshot before they change state.
func (m *Model) pushBrowserLocationSnapshot(location browserLocation) {
	if len(m.browserHistory) > 0 && reflect.DeepEqual(m.browserHistory[len(m.browserHistory)-1], location) {
		return
	}
	// Navigating somewhere new abandons any forward trail, the way a browser
	// drops its forward history once you branch off.
	m.browserForward = nil
	m.browserHistory = boundedHistory(append(m.browserHistory, location))
}

func boundedHistory(entries []browserLocation) []browserLocation {
	if len(entries) > maxBrowserHistoryEntries {
		return entries[len(entries)-maxBrowserHistoryEntries:]
	}
	return entries
}

func (m *Model) clearBrowserHistory() {
	m.browserHistory = nil
	m.browserForward = nil
}

func (m *Model) restoreBrowserLocation() tea.Cmd {
	if len(m.browserHistory) == 0 {
		m.status = "No previous browser location"
		return nil
	}
	last := len(m.browserHistory) - 1
	location := m.browserHistory[last]
	current := m.snapshotBrowserLocation()
	m.browserHistory = m.browserHistory[:last]
	m.browserForward = boundedHistory(append(m.browserForward, current))
	return m.applyBrowserLocation(location, "Back to ")
}

// advanceBrowserLocation re-enters a location an earlier back step left.
func (m *Model) advanceBrowserLocation() tea.Cmd {
	if len(m.browserForward) == 0 {
		m.status = "No later browser location"
		return nil
	}
	last := len(m.browserForward) - 1
	location := m.browserForward[last]
	current := m.snapshotBrowserLocation()
	m.browserForward = m.browserForward[:last]
	m.browserHistory = boundedHistory(append(m.browserHistory, current))
	return m.applyBrowserLocation(location, "Forward to ")
}

func (m *Model) applyBrowserLocation(location browserLocation, prefix string) tea.Cmd {
	m.searching = false
	m.preview = nil

	m.cursor = location.cursor
	m.selectedID = location.selectedID
	m.selectedSessionID = location.selectedSessionID
	m.selectedPlanID = location.selectedPlanID
	m.selectedFolderPath = location.selectedFolderPath
	m.selectedSourceID = location.selectedSourceID
	m.selectedHitName = location.selectedHitName
	m.selectedChapter = location.selectedChapter
	m.navCursor = location.navCursor
	m.navKind = location.navKind
	m.navType = location.navType
	m.typeFilter = location.typeFilter
	m.tagFilter = location.tagFilter
	m.listScope = location.listScope
	m.collectionFilter = location.collectionFilter
	m.lastCollectionID = location.lastCollectionID
	m.layout.Focus = location.focus
	m.historyCursor = location.historyCursor
	m.collapsedFolders = cloneBoolMap(location.collapsedFolders)
	m.expandedFolders = cloneBoolMap(location.expandedFolders)

	if m.usesCampaignTree() {
		switch m.currentNav().Kind {
		case NavSessions:
			m.syncSessionTreeCursor()
		case NavPrep:
			m.syncPlannedNotesCursor()
		case NavSources:
			m.syncSourceListCursor()
		default:
			m.syncRecordTreeCursorFor(m.listRecords())
		}
	} else if record := m.selectedRecord(); record != nil {
		m.syncRecordTreeCursorFor(m.recordsForPane(entityTypePane(record.Type)))
	}
	m.ensureDetailView().offset = location.detailOffset
	m.status = prefix + m.browserLocationLabelFor(location)
	if gone := m.reconcileRestoredSelection(location); gone != "" {
		m.status = gone
	}
	return m.restoreSearchLocation(location.search)
}

// reconcileRestoredSelection replaces a restored target that no longer exists
// with the row now standing at the same position, so back and forward never
// land on nothing. It reports what happened, or "" when the target survives.
func (m *Model) reconcileRestoredSelection(location browserLocation) string {
	switch {
	case location.selectedID != "" && m.selectedRecord() == nil:
		m.selectedID = ""
		records := m.selectionRecords()
		if len(records) > 0 {
			m.selectRecord(records[clamp(location.cursor, 0, len(records)-1)])
		}
		return goneStatus("entity", m.browserLocationLabel(), m.selectedID != "")
	case location.selectedSessionID != "" && m.sessionByID(location.selectedSessionID) == nil:
		m.selectedSessionID = ""
		if ids := sessionRowIDs(m.sessionTreeRows()); len(ids) > 0 {
			m.selectedSessionID = ids[clamp(location.cursor, 0, len(ids)-1)]
		}
		m.syncSessionTreeCursor()
		return goneStatus("session", m.browserLocationLabel(), m.selectedSessionID != "")
	case location.selectedPlanID != "" && m.plannedByID(location.selectedPlanID) == nil:
		m.selectedPlanID = ""
		if plans := m.scopedPlannedNotes(); len(plans) > 0 {
			m.selectedPlanID = plans[clamp(location.cursor, 0, len(plans)-1)].ID
		}
		m.syncPlannedNotesCursor()
		return goneStatus("prep notes", m.browserLocationLabel(), m.selectedPlanID != "")
	}
	return ""
}

func goneStatus(kind, replacement string, found bool) string {
	if !found {
		return "That " + kind + " is gone · nothing left in this section"
	}
	return "That " + kind + " is gone · showing " + replacement
}

// browserTrailLabel is the compact back/forward affordance in the footer, so
// the trail's depth is visible without a permanent shortcut wall. It stays
// short because the footer already shares its line with the status.
func (m Model) browserTrailLabel() string {
	parts := make([]string, 0, 2)
	if depth := len(m.browserHistory); depth > 0 {
		parts = append(parts, fmt.Sprintf("← %d back", depth))
	}
	if depth := len(m.browserForward); depth > 0 {
		parts = append(parts, fmt.Sprintf("%d fwd →", depth))
	}
	return strings.Join(parts, "  ")
}

func (m *Model) syncPlannedNotesCursor() {
	plans := m.scopedPlannedNotes()
	if len(plans) == 0 {
		m.cursor = 0
		m.selectedPlanID = ""
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
}

// browserLocationLabelFor names a restored location, preferring the search it
// was taken from so returning to a query reads as returning to that query.
func (m Model) browserLocationLabelFor(location browserLocation) string {
	if location.search.open {
		if query := strings.TrimSpace(location.search.query); query != "" {
			return "search · " + query
		}
		return "search"
	}
	return m.browserLocationLabel()
}

func (m Model) browserLocationLabel() string {
	if record := m.selectedRecord(); record != nil {
		return record.Title
	}
	if session := m.sessionByID(m.selectedSessionID); session != nil {
		return session.Title
	}
	if plan := m.plannedByID(m.selectedPlanID); plan != nil {
		return plan.Title
	}
	if m.selectedHitName != "" {
		return m.selectedHitName
	}
	if m.selectedFolderPath != "" {
		return m.selectedFolderPath
	}
	if label := m.currentNav().Label; label != "" {
		return label
	}
	return "browser"
}

// selectionRecords is the record list the browser selection moves through.
func (m Model) selectionRecords() []domain.Record {
	if m.usesCampaignTree() {
		return m.listRecords()
	}
	return m.recordsForPane(m.layout.Focus)
}

// neighborID names the row that should take a removed row's place: the row
// after it, else the row before it, else nothing. Keeping the rule this
// explicit is what makes selection after a delete predictable.
func neighborID(ids []string, removed string) string {
	for index, id := range ids {
		if id != removed {
			continue
		}
		if index+1 < len(ids) {
			return ids[index+1]
		}
		if index > 0 {
			return ids[index-1]
		}
		return ""
	}
	return ""
}

func recordIDs(records []domain.Record) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func sessionRowIDs(rows []domain.SessionTreeRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Session.ID != "" {
			ids = append(ids, row.Session.ID)
		}
	}
	return ids
}
