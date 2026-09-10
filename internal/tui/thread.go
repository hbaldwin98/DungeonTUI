package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/app"
	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

// prepThreadRows bounds the threads shown beside prep so the run sheet stays
// on screen.
const prepThreadRows = 4

func neglectedThreads(threads []domain.ThreadStatus) int {
	count := 0
	for _, thread := range threads {
		if thread.Neglected {
			count++
		}
	}
	return count
}

// renderThreadRow is the one-line thread summary shared by Home and prep.
func renderThreadRow(thread domain.ThreadStatus) string {
	line := "  " + thread.Record.Authority.Marker() + " " + thread.Record.Title + "  " +
		mutedStyle.Render(thread.Attention())
	if thread.Neglected {
		line = "  " + thread.Record.Authority.Marker() + " " + thread.Record.Title + "  " +
			neglectStyle.Render(thread.Attention())
	}
	line += "\n"
	if len(thread.NextBeats) > 0 {
		line += mutedStyle.Render("      next: "+thread.NextBeats[0]) + "\n"
	}
	return line
}

// renderPrepThreads lists the active threads this prep's location and cast
// carry, so planning starts from what is already in motion.
func (m Model) renderPrepThreads(plan domain.PlannedNotes) string {
	ids := []string{plan.LocationID}
	for _, link := range plan.Links {
		ids = append(ids, link.RecordID)
	}
	threads := domain.ThreadsTouching(m.workspace, m.workspace.Scope, ids)
	var builder strings.Builder
	builder.WriteString(labelStyle.Render("THREADS IN THIS PREP"))
	builder.WriteString("\n")
	if len(threads) == 0 {
		active := len(domain.CampaignThreads(m.workspace, m.workspace.Scope, false))
		if active == 0 {
			builder.WriteString(mutedStyle.Render("  — no active threads in this campaign"))
		} else {
			builder.WriteString(mutedStyle.Render(fmt.Sprintf("  — none touch this cast · %d active on Home", active)))
		}
		builder.WriteString("\n")
		return builder.String()
	}
	for index, thread := range threads {
		if index == prepThreadRows {
			builder.WriteString(mutedStyle.Render(fmt.Sprintf("  + %d more", len(threads)-index)))
			builder.WriteString("\n")
			break
		}
		builder.WriteString(renderThreadRow(thread))
	}
	return builder.String()
}

// selectedThread returns the selected record when it is a trackable thread.
func (m Model) selectedThread() (domain.Record, string) {
	record := m.selectedRecord()
	if record == nil || record.Type != domain.Thread {
		return domain.Record{}, "Select a thread"
	}
	if record.Authority == domain.Proposal || record.IsAIContent {
		return domain.Record{}, "Accept the AI proposal before tracking its state"
	}
	return *record, ""
}

// setSelectedThreadState is the explicit owner transition. It commits only
// the state change; body, citations, and session history stay as they were.
func (m Model) setSelectedThreadState(state domain.ThreadState) (tea.Model, tea.Cmd) {
	record, reason := m.selectedThread()
	if reason != "" {
		m.status = reason
		return m, nil
	}
	previous := domain.EffectiveThreadState(record)
	if previous == state {
		m.status = fmt.Sprintf("%s is already %s", record.Title, state)
		return m, nil
	}
	updated, err := domain.SetThreadState(record, state)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	next, err := m.app.Commit(m.workspace, func(ws domain.Workspace) (domain.Workspace, error) {
		return app.UpsertRecord(ws, updated)
	})
	if err != nil {
		m.status = "Thread change rolled back: " + err.Error()
		return m, nil
	}
	m.replaceWorkspace(next)
	m.rebuildSearch()
	m.refreshResults()
	m.status = fmt.Sprintf("%s · %s → %s · history kept", record.Title, previous, state)
	return m, nil
}
