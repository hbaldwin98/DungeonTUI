package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Status lifetimes. A status is feedback about the action just taken, so it
// clears on its own; nothing ever has to be dismissed. Problems stay longer
// because they usually ask the owner to do something.
const (
	statusLifetime      = 4 * time.Second
	statusErrorLifetime = 9 * time.Second
)

type statusKind int

const (
	statusInfo statusKind = iota
	statusProblem
)

// statusExpireMsg clears the status it was scheduled for, and only that one.
type statusExpireMsg struct{ seq uint64 }

// problemMarkers identify a status that reports a failure or a refusal.
var problemMarkers = []string{
	"rolled back", "failed", "error", "cannot", "can't", "refused",
	"unknown", "required", "needs ", "no longer exists", "not a ",
}

func classifyStatus(text string) statusKind {
	lower := strings.ToLower(text)
	for _, marker := range problemMarkers {
		if strings.Contains(lower, marker) {
			return statusProblem
		}
	}
	return statusInfo
}

// trackStatus is the one place status timing lives. Every Update passes
// through it, so the many call sites that set m.status stay simple
// assignments and still expire consistently.
func trackStatus(before string, next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	model, ok := next.(Model)
	if !ok || model.status == before || model.status == "" {
		return next, cmd
	}
	model.statusSeq++
	seq := model.statusSeq
	lifetime := statusLifetime
	if classifyStatus(model.status) == statusProblem {
		lifetime = statusErrorLifetime
	}
	return model, tea.Batch(cmd, scheduleStatusExpiry(lifetime, seq))
}

// scheduleStatusExpiry builds the timer that clears a status. It is a
// variable so the test binary can disable it: synchronous test helpers call
// every command inline, and a real tick would make each of them sleep.
var scheduleStatusExpiry = func(lifetime time.Duration, seq uint64) tea.Cmd {
	return tea.Tick(lifetime, func(time.Time) tea.Msg { return statusExpireMsg{seq: seq} })
}

func (m Model) expireStatus(msg statusExpireMsg) (tea.Model, tea.Cmd) {
	if msg.seq == m.statusSeq {
		m.status = ""
	}
	return m, nil
}

// renderStatus styles a status for a footer. Problems use the warning
// foreground from decision #27; nothing gets a filled background.
func renderStatus(text string) string {
	if classifyStatus(text) == statusProblem {
		return neglectStyle.Render(text)
	}
	return text
}

// footerWithStatus joins the status and the key hints so the hints give way
// first on a narrow terminal and the status is never pushed off-screen.
func footerWithStatus(status, help string, width int) string {
	if status == "" {
		return help
	}
	joined := renderStatus(status) + "  ·  " + help
	if lipgloss.Width(joined) <= width-2 {
		return joined
	}
	return renderStatus(status)
}
