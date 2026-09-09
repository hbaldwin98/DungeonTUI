package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

func (m Model) openPlayback(targetEntryID ...string) (tea.Model, tea.Cmd) {
	session := m.selectedSession()
	if session == nil {
		m.status = "Select an ended session to play back"
		return m, nil
	}
	if session.EndedAt == nil {
		m.status = "End the session before playback"
		return m, nil
	}
	_, frames, ok := domain.SessionPlayback(m.workspace, session.ID)
	if !ok {
		m.status = "Session not found"
		return m, nil
	}
	m.playingBack = true
	m.playbackCursor = 0
	if len(targetEntryID) > 0 && targetEntryID[0] != "" {
		for index, frame := range frames {
			if frame.EntryID == targetEntryID[0] {
				m.playbackCursor = index
				break
			}
		}
	}
	if len(frames) == 0 {
		m.status = "Playback · no derived beats yet"
	} else {
		m.status = fmt.Sprintf("Playback · %d beats · ← rewind · → fast-forward", len(frames))
	}
	return m, nil
}

func (m Model) playbackCursorForEntry(entryID string) (int, bool) {
	if entryID == "" {
		return 0, false
	}
	_, frames, ok := domain.SessionPlayback(m.workspace, m.selectedSessionID)
	if !ok {
		return 0, false
	}
	for index, frame := range frames {
		if frame.EntryID == entryID {
			return index, true
		}
	}
	return 0, false
}

func (m Model) updatePlayback(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	_, frames, _ := domain.SessionPlayback(m.workspace, m.selectedSessionID)
	switch msg.String() {
	case "esc", "q":
		m.playingBack = false
		m.status = "Closed playback · transcript unchanged"
		return m, nil
	case "left", "h", "k", "up":
		if len(frames) > 0 {
			m.playbackCursor = clamp(m.playbackCursor-1, 0, len(frames)-1)
		}
		return m, nil
	case "right", "l", "j", "down":
		if len(frames) > 0 {
			m.playbackCursor = clamp(m.playbackCursor+1, 0, len(frames)-1)
		}
		return m, nil
	case "home":
		m.playbackCursor = 0
		return m, nil
	case "end":
		if len(frames) > 0 {
			m.playbackCursor = len(frames) - 1
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderPlaybackOverlay() string {
	width := min(92, max(56, m.width-8))
	session, frames, ok := domain.SessionPlayback(m.workspace, m.selectedSessionID)
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render("SESSION PLAYBACK"))
	if !ok {
		builder.WriteString("\n\n")
		builder.WriteString(mutedStyle.Render("Session not found."))
	} else {
		builder.WriteString("  ")
		builder.WriteString(filterStyle.Render(session.Title))
		if len(frames) > 0 {
			builder.WriteString(fmt.Sprintf("  %d/%d", m.playbackCursor+1, len(frames)))
		}
		builder.WriteString("\n")
		builder.WriteString(mutedStyle.Render("Derived beats · transcript stays immutable"))
		builder.WriteString("\n\n")
		if len(frames) == 0 {
			builder.WriteString(mutedStyle.Render("No derived beats for this sit."))
		} else {
			cursor := clamp(m.playbackCursor, 0, len(frames)-1)
			start := 0
			visible := 8
			if cursor >= visible {
				start = cursor - visible + 1
			}
			end := min(len(frames), start+visible)
			for index := start; index < end; index++ {
				frame := frames[index]
				prefix := "  "
				style := searchResultStyle
				if index == cursor {
					prefix = "▸ "
					style = selectedSearchResultStyle
				}
				status := ""
				if frame.Status != "" {
					status = " [" + frame.Status + "]"
				}
				line := fmt.Sprintf("%s%-5s %s%s", prefix, frame.Label, frame.Summary, status)
				builder.WriteString(style.Render(line))
				builder.WriteString("\n")
			}
			builder.WriteString("\n")
			current := frames[cursor]
			if current.EntryText != "" {
				builder.WriteString(labelStyle.Render("BEAT"))
				builder.WriteString("\n")
				builder.WriteString(current.EntryText)
				builder.WriteString("\n")
			}
			if record := m.recordByID(current.RecordID); record != nil {
				builder.WriteString("\n")
				builder.WriteString(labelStyle.Render("ENTITY"))
				builder.WriteString("  " + typeStyle.Render(string(record.Type)) + "  ")
				builder.WriteString(authorityStyle(record.Authority).Render(record.Authority.Marker() + " " + record.Authority.Label()))
				builder.WriteString("\n")
				builder.WriteString(detailTitleStyle.Render(record.Title))
				builder.WriteString("\n")
				if record.Summary != "" {
					builder.WriteString(record.Summary)
				}
			}
		}
	}
	builder.WriteString("\n\n")
	builder.WriteString(helpStyle.Render("← rewind  → fast-forward  Home/End  Esc close"))
	overlay := searchPanelStyle.Width(width).Render(builder.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#24283B"))),
	)
}

func (m Model) recordByID(id string) *domain.Record {
	if id == "" {
		return nil
	}
	for index := range m.workspace.Records {
		if m.workspace.Records[index].ID == id {
			record := m.workspace.Records[index]
			return &record
		}
	}
	return nil
}
