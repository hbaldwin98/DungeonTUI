package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/hbaldwin98/DungeonTUI/internal/app"
	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

func (m Model) unresolvedReconciliationItems() []int {
	if m.reconIndex < 0 || m.reconIndex >= len(m.workspace.Reconciliations) {
		return nil
	}
	var indices []int
	for index, item := range m.workspace.Reconciliations[m.reconIndex].Items {
		if domain.ReconciliationUnresolved(item.Status) {
			indices = append(indices, index)
		}
	}
	return indices
}

func (m *Model) selectFirstUnresolvedReconciliationItem() {
	indices := m.unresolvedReconciliationItems()
	if len(indices) > 0 {
		m.reconCursor = indices[0]
	} else {
		m.reconCursor = 0
	}
}

func (m *Model) moveReconciliationCursor(delta int) {
	indices := m.unresolvedReconciliationItems()
	if len(indices) == 0 {
		return
	}
	position := 0
	for index, itemIndex := range indices {
		if itemIndex == m.reconCursor {
			position = index
			break
		}
	}
	m.reconCursor = indices[clamp(position+delta, 0, len(indices)-1)]
}

func (m *Model) advanceReconciliationCursor() {
	indices := m.unresolvedReconciliationItems()
	if len(indices) == 0 {
		m.reconCursor = 0
		return
	}
	for _, index := range indices {
		if index > m.reconCursor {
			m.reconCursor = index
			return
		}
	}
	m.reconCursor = indices[0]
}

func (m Model) unresolvedReconciliationForSession(sessionID string) int {
	for index, recon := range m.workspace.Reconciliations {
		if recon.SessionID != sessionID {
			continue
		}
		for _, item := range recon.Items {
			if domain.ReconciliationUnresolved(item.Status) {
				return index
			}
		}
	}
	return -1
}

func (m Model) reconciliationResultStatus(action string) string {
	remaining := len(m.unresolvedReconciliationItems())
	if remaining == 0 {
		return action + " · inbox complete · transcript unchanged"
	}
	return fmt.Sprintf("%s · %d remaining · transcript unchanged", action, remaining)
}

func (m Model) deferReconciliationItem() (tea.Model, tea.Cmd) {
	if len(m.unresolvedReconciliationItems()) == 0 {
		return m, nil
	}
	current := m.reconCursor
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.DeferRecon(ws, m.reconIndex, current)
	})
	if err != nil {
		m.status = "Defer rolled back: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	indices := m.unresolvedReconciliationItems()
	if len(indices) > 1 {
		for position, index := range indices {
			if index == current {
				m.reconCursor = indices[(position+1)%len(indices)]
				break
			}
		}
	}
	m.status = m.reconciliationResultStatus("Deferred")
	return m, nil
}

func (m Model) reconciliationProvenance(item domain.ReconciliationItem) string {
	if m.reconIndex < 0 || m.reconIndex >= len(m.workspace.Reconciliations) {
		return "Evidence unavailable"
	}
	recon := m.workspace.Reconciliations[m.reconIndex]
	for _, session := range m.workspace.Sessions {
		if session.ID != recon.SessionID {
			continue
		}
		parts := []string{"From " + session.Title}
		for _, entry := range session.Entries {
			if entry.ID == item.EntryID {
				excerpt := strings.TrimSpace(entry.Text)
				if len(excerpt) > 120 {
					excerpt = excerpt[:119] + "…"
				}
				if excerpt != "" {
					parts = append(parts, "“"+excerpt+"”")
				}
				break
			}
		}
		for _, record := range m.workspace.Records {
			if record.ID == item.RecordID {
				parts = append(parts, fmt.Sprintf("Target: %s · %s · %s", record.Title, record.Type, record.Authority))
				break
			}
		}
		return strings.Join(parts, "\n")
	}
	return "Session evidence unavailable"
}

func (m Model) updateReconciliationMouse(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft || m.reconEditing {
		return m, nil
	}
	indices := m.unresolvedReconciliationItems()
	width := min(88, max(32, m.width-4))
	left := max(0, (m.width-(width+2))/2)
	top := max(0, (m.height-m.reconciliationOverlayHeight())/2)
	row := msg.Y - top - 5
	if row >= 0 && row < len(indices) {
		m.reconCursor = indices[row]
		return m, nil
	}
	if msg.Y == top+m.reconciliationOverlayHeight()-2 {
		relative := msg.X - left
		switch {
		case relative < width/4:
			return m.approveReconciliationItem(&m.workspace.Reconciliations[m.reconIndex])
		case relative < width/2:
			return m.startReconEdit(&m.workspace.Reconciliations[m.reconIndex])
		case relative < width*3/4:
			return m.deferReconciliationItem()
		default:
			return m.rejectReconciliationItem(&m.workspace.Reconciliations[m.reconIndex])
		}
	}
	return m, nil
}

func (m Model) reconciliationOverlayHeight() int {
	return min(max(14, 10+len(m.unresolvedReconciliationItems())), max(14, m.height-2))
}
