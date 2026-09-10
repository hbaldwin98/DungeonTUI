package tui

import (
	"fmt"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

type beatState string

const (
	beatDone    beatState = "done"
	beatSkipped beatState = "skipped"
)

func (m *Model) ensurePrepState() {
	if m.prepBeat == nil {
		m.prepBeat = map[string]int{}
	}
	if m.prepBeatState == nil {
		m.prepBeatState = map[string]map[string]beatState{}
	}
}

func (m Model) runSheetPlan() *domain.PlannedNotes {
	if m.session != nil {
		return m.sessionPlannedNotes()
	}
	return m.plannedByID(m.selectedPlanID)
}

func (m Model) currentPrepBeat(plan domain.PlannedNotes) ([]domain.PlannedBeat, int) {
	beats := domain.ParsePlannedOutline(plan.Body)
	if len(beats) == 0 {
		return nil, 0
	}
	return beats, clamp(m.prepBeat[m.prepRunKey(plan.ID)], 0, len(beats)-1)
}

func (m Model) prepRunKey(planID string) string {
	if m.session != nil {
		return m.session.ID + "\x1f" + planID
	}
	return planID
}

func (m *Model) movePrepBeat(delta int) {
	plan := m.runSheetPlan()
	if plan == nil {
		return
	}
	beats, index := m.currentPrepBeat(*plan)
	if len(beats) == 0 {
		return
	}
	m.ensurePrepState()
	m.prepBeat[m.prepRunKey(plan.ID)] = clamp(index+delta, 0, len(beats)-1)
	m.prepScroll = 0
	view := m.ensureDetailView()
	view.offset = 0
	view.contentValid = false
}

func (m *Model) togglePrepBeatState(want beatState) {
	plan := m.runSheetPlan()
	if plan == nil {
		return
	}
	beats, index := m.currentPrepBeat(*plan)
	if len(beats) == 0 {
		return
	}
	m.ensurePrepState()
	key := m.prepRunKey(plan.ID)
	if m.prepBeatState[key] == nil {
		m.prepBeatState[key] = map[string]beatState{}
	}
	if m.prepBeatState[key][beats[index].ID] == want {
		delete(m.prepBeatState[key], beats[index].ID)
	} else {
		m.prepBeatState[key][beats[index].ID] = want
	}
}

func (m Model) renderRunSheet(plan domain.PlannedNotes, width int) string {
	beats, index := m.currentPrepBeat(plan)
	if len(beats) == 0 {
		return mutedStyle.Render("No prepared notes")
	}
	if len(beats) == 1 && beats[0].ID == "notes-1" {
		return m.renderMarkdown(strings.TrimSpace(plan.Body), max(1, width))
	}
	beat := beats[index]
	state := m.prepBeatState[m.prepRunKey(plan.ID)][beat.ID]
	marker := ""
	if state == beatDone {
		marker = " · DONE"
	} else if state == beatSkipped {
		marker = " · SKIPPED"
	}
	header := fmt.Sprintf("%d/%d  %s  %s%s", index+1, len(beats), strings.ToUpper(string(beat.Kind)), beat.Title, marker)
	lines := strings.Split(strings.ReplaceAll(plan.Body, "\r\n", "\n"), "\n")
	start, end := clamp(beat.StartLine, 0, len(lines)), clamp(beat.EndLine, 0, len(lines))
	body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if body == "" {
		body = "(empty beat)"
	}
	return sectionStyle.Render(header) + "\n" + m.renderMarkdown(body, max(1, width))
}
