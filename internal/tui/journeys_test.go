package tui

import (
	"strings"
	"testing"
)

// TestCoreJourneysUnderPressure runs every core DM journey at every journey
// size. A failure here is a usability regression: record it as its own task
// and fix it rather than loosening the budget or the audit.
func TestCoreJourneysUnderPressure(t *testing.T) {
	for _, journey := range CoreJourneys() {
		for _, size := range JourneySizes {
			journey, width, height := journey, size[0], size[1]
			t.Run(journey.Name+"/"+sizeName(width, height), func(t *testing.T) {
				result := RunJourney(journey, width, height)
				if !result.OK() {
					t.Fatalf("%s", result.Summary())
				}
			})
		}
	}
}

func sizeName(width, height int) string {
	return strings.Join([]string{itoa(width), itoa(height)}, "x")
}

func itoa(value int) string {
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if len(digits) == 0 {
		return "0"
	}
	return string(digits)
}

func TestJourneyAuditCatchesEachRegressionClass(t *testing.T) {
	var problems []string
	note := func(problem string) { problems = append(problems, problem) }

	auditFrame(Frame{Width: 4, Height: 1, Lines: []string{"abc"}, LineWidths: []int{3}, Raw: "abc"}, note)
	if len(problems) != 1 || !strings.Contains(problems[0], "does not fill") {
		t.Fatalf("a short row is a fill problem: %v", problems)
	}

	if codes := BackgroundCodes("\x1b[38;2;48;48;48mfg only\x1b[m"); len(codes) != 0 {
		t.Fatalf("a truecolor foreground containing 48 is not a background: %v", codes)
	}
	for _, raw := range []string{"\x1b[40mx", "\x1b[37;40mx", "\x1b[48;5;236mx", "\x1b[48;2;1;2;3mx", "\x1b[104mx"} {
		if len(BackgroundCodes(raw)) != 1 {
			t.Fatalf("%q paints a background and must be reported", raw)
		}
	}

	h := NewHarness(100, 30)
	if !keyDocumented(h, "/") {
		t.Fatal("/ is documented in the browser")
	}
	if keyDocumented(h, "ctrl+q") {
		t.Fatal("an unmentioned key must not count as documented")
	}

	before := canonSnapshot(h.Model.workspace)
	for index := range h.Model.workspace.Records {
		if h.Model.workspace.Records[index].ID == "draft-sister-elayne" {
			h.Model.workspace.Records[index].Authority = "canon"
		}
	}
	if changes := canonChanges(before, h.Model.workspace); len(changes) != 1 || !strings.Contains(changes[0], "became canon") {
		t.Fatalf("a silent promotion is an accidental canon write: %v", changes)
	}
}

func TestJourneyBudgetOverrunIsReported(t *testing.T) {
	journey := CoreJourneys()[0]
	journey.Budget = 1
	result := RunJourney(journey, 100, 30)
	if result.OK() || !strings.Contains(strings.Join(result.Problems, " "), "over budget") {
		t.Fatalf("a journey past its budget must fail: %s", result.Summary())
	}
}
