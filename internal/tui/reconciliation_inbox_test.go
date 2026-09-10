package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

func reconciliationInboxModel() Model {
	m := newModel(demoWorkspace(), nil, nil)
	m.width = 80
	m.height = 24
	ended := time.Now().UTC()
	session := domain.SessionRecord{
		ID: "sit-review", Title: "The Broken Seal", Scope: m.workspace.Scope,
		EndedAt: &ended,
		Entries: []domain.TranscriptEntry{
			{ID: "entry-accepted", Text: "Already filed."},
			{ID: "entry-pending", Text: "The seal cracked under the chapel."},
			{ID: "entry-deferred", Text: "Ask Vale about the key."},
		},
	}
	m.workspace.Sessions = append(m.workspace.Sessions, session)
	m.workspace.Reconciliations = []domain.ReconciliationRecord{{
		ID: "recon-review", SessionID: session.ID, Title: "Review The Broken Seal",
		Items: []domain.ReconciliationItem{
			{ID: "accepted", EntryID: "entry-accepted", Status: domain.ReconApproved},
			{ID: "pending", EntryID: "entry-pending", Status: domain.ReconPending},
			{ID: "deferred", EntryID: "entry-deferred", Status: domain.ReconDeferred},
		},
	}}
	return m
}

func TestReconciliationInboxResumesAtFirstUnresolvedAndShowsEvidence(t *testing.T) {
	m := reconciliationInboxModel()
	updated, _ := m.openReconciliation()
	m = updated.(Model)
	if m.reconCursor != 1 {
		t.Fatalf("cursor=%d", m.reconCursor)
	}
	view := testANSI.ReplaceAllString(m.View().Content, "")
	for _, want := range []string{"POST-SESSION INBOX", "2 remaining", "From The Broken Seal", "seal cracked", "a accept", "d defer"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in narrow inbox:\n%s", want, view)
		}
	}
}

func TestReconciliationInboxActionsAdvanceAndComplete(t *testing.T) {
	m := reconciliationInboxModel()
	updated, _ := m.openReconciliation()
	m = updated.(Model)
	updated, _ = m.updateReconciliation(tea.KeyPressMsg{Code: 'x'})
	m = updated.(Model)
	if m.reconCursor != 2 || m.workspace.Reconciliations[0].Items[1].Status != domain.ReconRejected {
		t.Fatalf("reject did not advance: cursor=%d items=%#v", m.reconCursor, m.workspace.Reconciliations[0].Items)
	}
	updated, _ = m.updateReconciliation(tea.KeyPressMsg{Code: 'a'})
	m = updated.(Model)
	if len(m.unresolvedReconciliationItems()) != 0 || !strings.Contains(m.status, "inbox complete") {
		t.Fatalf("completion = unresolved %v status %q", m.unresolvedReconciliationItems(), m.status)
	}
}

func TestReconciliationInboxMouseUsesSameDeferAction(t *testing.T) {
	m := reconciliationInboxModel()
	updated, _ := m.openReconciliation()
	m = updated.(Model)
	height := m.reconciliationOverlayHeight()
	top := max(0, (m.height-height)/2)
	updated, _ = m.Update(tea.MouseClickMsg{X: m.width / 2, Y: top + height - 2, Button: tea.MouseLeft})
	m = updated.(Model)
	if m.workspace.Reconciliations[0].Items[1].Status != domain.ReconDeferred {
		t.Fatalf("mouse defer status=%q", m.workspace.Reconciliations[0].Items[1].Status)
	}
}

func TestEndedSessionOpensUnfinishedInboxBeforePlayback(t *testing.T) {
	m := reconciliationInboxModel()
	m.setNavCursor(0)
	m.selectedSessionID = "sit-review"
	for index, row := range m.sessionTreeRows() {
		if row.Session.ID == m.selectedSessionID {
			m.cursor = index
			break
		}
	}
	updated, _ := m.activateSessionSelection()
	m = updated.(Model)
	if !m.reconciling || m.playingBack || m.reconCursor != 1 {
		t.Fatalf("ended session route: recon=%t playback=%t cursor=%d", m.reconciling, m.playingBack, m.reconCursor)
	}
}
