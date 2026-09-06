package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	searchsvc "github.com/hbaldwin98/dungeon/internal/search"
)

func (m Model) scopedRecords(applyTypeFilter bool) []domain.Record {
	records := make([]domain.Record, 0, len(m.workspace.Records))
	for _, record := range m.workspace.Records {
		if !m.recordInListScope(record) {
			continue
		}
		if applyTypeFilter && m.typeFilter != "" && record.Type != m.typeFilter {
			continue
		}
		if m.tagFilter != "" && !containsTag(record, m.tagFilter) {
			continue
		}
		if m.collectionFilter != "" {
			col, ok := domain.FindCollection(m.workspace.Collections, m.collectionFilter)
			if !ok || !col.Has(record.ID) {
				continue
			}
		}
		records = append(records, record)
	}
	return records
}

func (m Model) recordInListScope(record domain.Record) bool {
	switch m.listScope {
	case searchsvc.CurrentWorld:
		return record.Scope.WorldID == m.workspace.Scope.WorldID
	case searchsvc.EntireLibrary:
		return true
	default: // CurrentCampaign
		return record.Scope.CampaignID == m.workspace.Scope.CampaignID ||
			(record.Scope.CampaignID == "" && record.Scope.WorldID == m.workspace.Scope.WorldID)
	}
}

func (m Model) availableTagFilters() []string {
	seen := map[string]string{}
	for _, record := range m.scopedRecordsIgnoringTag() {
		for _, tag := range record.Tags {
			key := strings.ToLower(tag)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; !ok {
				seen[key] = tag
			}
		}
	}
	out := make([]string, 0, len(seen))
	for _, tag := range seen {
		out = append(out, tag)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func (m Model) scopedRecordsIgnoringTag() []domain.Record {
	records := make([]domain.Record, 0, len(m.workspace.Records))
	for _, record := range m.workspace.Records {
		if !m.recordInListScope(record) {
			continue
		}
		entry := m.currentNav()
		if entry.Kind == NavType && entry.Type != "" && record.Type != entry.Type {
			continue
		}
		records = append(records, record)
	}
	return records
}

func (m *Model) cycleTagFilter() {
	tags := m.availableTagFilters()
	if len(tags) == 0 {
		m.tagFilter = ""
		m.status = "No tags in this section"
		return
	}
	if m.tagFilter == "" {
		m.tagFilter = tags[0]
	} else {
		found := -1
		for index, tag := range tags {
			if strings.EqualFold(tag, m.tagFilter) {
				found = index
				break
			}
		}
		if found < 0 || found+1 >= len(tags) {
			m.tagFilter = ""
		} else {
			m.tagFilter = tags[found+1]
		}
	}
	m.cursor = 0
	m.selectedID = ""
	records := m.listRecords()
	if len(records) > 0 {
		m.selectRecord(records[0])
	}
	if m.tagFilter == "" {
		m.status = "Tag filter: all"
	} else {
		m.status = "Tag filter: #" + m.tagFilter
	}
}

func (m *Model) cycleListScope() {
	m.listScope = (m.listScope + 1) % 3
	m.cursor = 0
	m.selectedID = ""
	records := m.listRecords()
	if len(records) > 0 {
		m.selectRecord(records[0])
	} else if m.usesCampaignTree() {
		m.ensureBrowserSelection()
	}
	m.status = "Scope filter: " + m.listScope.Label()
}

func (m Model) scopedCollections() []domain.Collection {
	return domain.ScopedCollections(m.workspace.Collections, m.workspace.Scope)
}

func (m Model) activeCollection() *domain.Collection {
	if m.collectionFilter == "" {
		return nil
	}
	for index := range m.workspace.Collections {
		if m.workspace.Collections[index].ID == m.collectionFilter {
			return &m.workspace.Collections[index]
		}
	}
	return nil
}

func (m *Model) cycleCollectionFilter() {
	cols := m.scopedCollections()
	if len(cols) == 0 {
		m.collectionFilter = ""
		m.status = "No collections yet · g to name one"
		return
	}
	if m.collectionFilter == "" {
		m.collectionFilter = cols[0].ID
	} else {
		found := -1
		for index, col := range cols {
			if col.ID == m.collectionFilter {
				found = index
				break
			}
		}
		if found < 0 || found+1 >= len(cols) {
			m.collectionFilter = ""
		} else {
			m.collectionFilter = cols[found+1].ID
		}
	}
	m.cursor = 0
	m.selectedID = ""
	records := m.listRecords()
	if len(records) > 0 {
		m.selectRecord(records[0])
	} else if m.usesCampaignTree() {
		m.ensureBrowserSelection()
	}
	if m.collectionFilter == "" {
		m.status = "Collection: all"
		return
	}
	if col := m.activeCollection(); col != nil {
		m.lastCollectionID = col.ID
		m.status = "Collection: " + col.Title
	}
}

func (m Model) targetCollection() *domain.Collection {
	if col := m.activeCollection(); col != nil {
		return col
	}
	if m.lastCollectionID == "" {
		return nil
	}
	for index := range m.workspace.Collections {
		if m.workspace.Collections[index].ID == m.lastCollectionID {
			return &m.workspace.Collections[index]
		}
	}
	return nil
}

func (m Model) openCollectionName() (tea.Model, tea.Cmd) {
	m.namingCollection = true
	m.collectionName.SetValue("")
	m.collectionName.SetWidth(max(24, m.width-16))
	m.collectionName.Focus()
	m.status = "Name a collection · Enter save · Esc cancel"
	return m, textinput.Blink
}

func (m Model) updateCollectionName(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.namingCollection = false
		m.collectionName.Blur()
		m.status = "Cancelled collection"
		return m, nil
	case "enter":
		return m.saveNamedCollection()
	}
	var cmd tea.Cmd
	m.collectionName, cmd = m.collectionName.Update(msg)
	return m, cmd
}

func (m Model) saveNamedCollection() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.collectionName.Value())
	if title == "" {
		m.status = "Collection title is required"
		return m, nil
	}
	now := time.Now().UTC()
	col := domain.Collection{
		ID:    fmt.Sprintf("col-%d", now.UnixNano()),
		Title: title,
		Scope: m.workspace.Scope,
	}
	if record := m.selectedRecord(); record != nil {
		col = col.Add(record.ID)
	}
	if err := col.Validate(); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.workspace.Collections = append(m.workspace.Collections, col)
	m.collectionFilter = col.ID
	m.lastCollectionID = col.ID
	m.namingCollection = false
	m.collectionName.Blur()
	m.persistWorkspace()
	m.status = "Collection “" + col.Title + "” · a adds the selected entity"
	return m, nil
}

func (m Model) toggleCollectionMembership() (tea.Model, tea.Cmd) {
	col := m.targetCollection()
	if col == nil {
		m.status = "Cycle to a collection with c, then a to add or remove"
		return m, nil
	}
	record := m.selectedRecord()
	if record == nil {
		m.status = "Select an entity to add"
		return m, nil
	}
	updated := col.Toggle(record.ID)
	for index := range m.workspace.Collections {
		if m.workspace.Collections[index].ID == updated.ID {
			m.workspace.Collections[index] = updated
			break
		}
	}
	m.persistWorkspace()
	if updated.Has(record.ID) {
		m.status = "Added " + record.Title + " to " + updated.Title
	} else {
		m.status = "Removed " + record.Title + " from " + updated.Title
		records := m.listRecords()
		if len(records) > 0 {
			m.selectRecord(records[0])
		}
	}
	return m, nil
}
