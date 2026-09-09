package tui

import (
	"reflect"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
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
	}
	if m.detailView != nil {
		location.detailOffset = m.detailView.offset
	}
	return location
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
	location := m.snapshotBrowserLocation()
	if len(m.browserHistory) > 0 && reflect.DeepEqual(m.browserHistory[len(m.browserHistory)-1], location) {
		return
	}
	m.browserHistory = append(m.browserHistory, location)
	if len(m.browserHistory) > maxBrowserHistoryEntries {
		m.browserHistory = m.browserHistory[len(m.browserHistory)-maxBrowserHistoryEntries:]
	}
}

func (m *Model) clearBrowserHistory() {
	m.browserHistory = nil
}

func (m *Model) restoreBrowserLocation() {
	if len(m.browserHistory) == 0 {
		m.status = "No previous browser location"
		return
	}
	last := len(m.browserHistory) - 1
	location := m.browserHistory[last]
	m.browserHistory = m.browserHistory[:last]

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
	m.status = "Back to " + m.browserLocationLabel()
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
