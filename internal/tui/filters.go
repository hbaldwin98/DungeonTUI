package tui

import (
	"sort"
	"strings"

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
