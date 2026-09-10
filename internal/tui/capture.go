package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/app"
	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

// captureRows is the visible height of the compact capture editor.
const captureRows = 3

// captureEvidenceRows caps the capture titles listed in post-session review.
const captureEvidenceRows = 3

// captureOverlay is transient capture state. Opening and closing it touches
// nothing else in the model, so cancelling returns to the exact prior UI.
type captureOverlay struct {
	Open    bool
	Input   textarea.Model
	Context domain.CaptureContext
}

func (m Model) canQuickCapture() bool {
	// Only overlays that own a text field are excluded, so ':' -style literal
	// typing and in-place renaming keep their exact semantics.
	if m.capture.Open || m.helping || m.settingsOpen || m.reconEditing {
		return false
	}
	if m.namingPicker || m.namingFolder || m.namingCollection {
		return false
	}
	return m.workspace.Scope.WorldID != ""
}

// captureContext collects whatever provenance the current surface knows.
// Every field is optional; capture never waits on missing context.
func (m Model) captureContext() domain.CaptureContext {
	context := domain.CaptureContext{Origin: m.captureOrigin()}
	if session := m.captureSession(); session != nil {
		context.SessionID = session.ID
		context.SessionTitle = session.Title
		context.LocationID = session.LocationID
		context.LocationName = session.LocationName
	}
	if context.LocationName == "" {
		context.LocationName = m.campaignHome().CurrentLocation
	}
	if record, ok := m.captureEntity(); ok {
		context.EntityID = record.ID
		context.EntityTitle = record.Title
	}
	return context
}

func (m Model) captureOrigin() string {
	switch {
	case m.session != nil:
		return "live play"
	case m.reconciling:
		return "post-session review"
	case m.playingBack:
		return "playback"
	case m.searching:
		return "search"
	case m.planning:
		return "prep"
	case m.editing:
		return "entity editor"
	case m.preview != nil:
		return "preview"
	default:
		return "browser"
	}
}

func (m Model) captureSession() *domain.SessionRecord {
	if m.session != nil {
		return m.session
	}
	if m.playingBack || m.reconciling {
		return m.selectedSession()
	}
	return nil
}

func (m Model) captureEntity() (domain.Record, bool) {
	if m.preview != nil {
		if record, ok := m.lookupAny(m.preview.Hop.RecordID); ok {
			return record, true
		}
	}
	if m.session != nil && m.review != nil {
		return *m.review, true
	}
	if record := m.selectedRecord(); record != nil {
		return *record, true
	}
	return domain.Record{}, false
}

func (m Model) openQuickCapture() (tea.Model, tea.Cmd) {
	if !m.canQuickCapture() {
		if m.workspace.Scope.WorldID == "" {
			m.status = "Open a world or campaign before capturing"
		}
		return m, nil
	}
	input := textarea.New()
	input.Prompt = "│ "
	input.Placeholder = "An unclassified thought…"
	input.ShowLineNumbers = false
	input.CharLimit = 0
	input.SetHeight(captureRows)
	input.SetWidth(captureInputWidth(m.width))
	m.capture = captureOverlay{Open: true, Input: input, Context: m.captureContext()}
	return m, m.capture.Input.Focus()
}

func captureInputWidth(width int) int {
	return max(16, min(64, width-8))
}

func (m *Model) closeQuickCapture() {
	m.capture = captureOverlay{}
}

func (m Model) updateQuickCapture(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeQuickCapture()
		m.status = "Capture discarded"
		return m, nil
	case "enter", "ctrl+enter", "ctrl+s", "\r", "\n":
		return m.submitQuickCapture()
	case "shift+enter":
		m.capture.Input.SetValue(m.capture.Input.Value() + "\n")
		return m, nil
	}
	var cmd tea.Cmd
	m.capture.Input, cmd = m.capture.Input.Update(msg)
	return m, cmd
}

func (m Model) submitQuickCapture() (tea.Model, tea.Cmd) {
	note, err := domain.NewCaptureNote(m.capture.Input.Value(), m.capture.Context, m.workspace.Scope)
	if err != nil {
		m.status = "Capture needs text · Esc discards"
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.UpsertRecord(ws, note)
	})
	if err != nil {
		m.status = "Capture rolled back: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.rebuildSearch()
	m.refreshResults()
	m.closeQuickCapture()
	m.status = fmt.Sprintf("Captured draft · %d unfiled · not canon", len(domain.UnfiledCaptures(m.workspace, m.workspace.Scope)))
	return m, nil
}

// fileSelectedCapture clears the inbox tag on the selected capture. Filing is
// not promotion: the note keeps its draft authority.
func (m Model) fileSelectedCapture() (tea.Model, tea.Cmd) {
	record := m.selectedRecord()
	if record == nil || !domain.IsUnfiledCapture(*record) {
		m.status = "Select an unfiled capture first"
		return m, nil
	}
	filed, err := domain.FileCaptureNote(*record)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.UpsertRecord(ws, filed)
	})
	if err != nil {
		m.status = "Filing rolled back: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.rebuildSearch()
	m.refreshResults()
	m.status = "Filed capture · still a draft: " + filed.Title
	return m, nil
}

func (m Model) renderQuickCapture(background string) string {
	width := min(72, max(24, m.width-4))
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("QUICK CAPTURE"))
	builder.WriteString("  ")
	builder.WriteString(mutedStyle.Render("Esc discards"))
	builder.WriteString("\n")
	builder.WriteString(mutedStyle.Render(truncateLine(domain.CaptureContextLabel(m.capture.Context), width)))
	builder.WriteString("\n\n")
	builder.WriteString(m.capture.Input.View())
	builder.WriteString("\n")
	builder.WriteString(helpStyle.Render("Enter save draft   Shift+Enter newline   file it later"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	x := max(0, (m.width-lipgloss.Width(overlay))/2)
	y := max(0, (m.height-lipgloss.Height(overlay))/3)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(background).X(0).Y(0).Z(0),
		lipgloss.NewLayer(overlay).X(x).Y(y).Z(1),
	).Render()
}

func truncateLine(value string, width int) string {
	if width <= 1 || lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if len(runes) <= width-1 {
		return value
	}
	return string(runes[:max(1, width-1)]) + "…"
}

// renderSessionCaptures lists the unfiled captures taken during one session as
// read-only review evidence. It never enters the reconciliation queue.
func (m Model) renderSessionCaptures(sessionID string, width int) string {
	captures := domain.SessionCaptures(m.workspace, sessionID)
	if len(captures) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("\n\n")
	builder.WriteString(labelStyle.Render(fmt.Sprintf("CAPTURED THIS SESSION · %d UNFILED", len(captures))))
	builder.WriteString("\n")
	for index, capture := range captures {
		if index == captureEvidenceRows {
			builder.WriteString(mutedStyle.Render(fmt.Sprintf("  + %d more in the wiki notes", len(captures)-index)))
			builder.WriteString("\n")
			break
		}
		builder.WriteString(mutedStyle.Render("  " + capture.Authority.Marker() + " " + truncateLine(capture.Title, max(8, width-4))))
		builder.WriteString("\n")
	}
	builder.WriteString(mutedStyle.Render("  Drafts only · file or discard them from the wiki"))
	return builder.String()
}
