package tui

import (
	"strings"
	"testing"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

func TestLiveRunSheetNavigatesAndMarksBeatsWithoutEditingMarkdown(t *testing.T) {
	model := New()
	body := "# Scene: Arrival\nOpening image.\n\n## Encounter: Guards\nTwo guards block the gate.\n\n## Clue: Broken seal\nWax lies on the floor.\n"
	plan := domain.PlannedNotes{ID: "run-sheet", Title: "The gate", Scope: model.workspace.Scope, Body: body}
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)
	model.selectedPlanID = plan.ID
	started, _ := model.startSession()
	model = started.(Model)
	model.setSessionFocus(prefs.PaneCampaign)

	first := stripANSIForTest(renderContentLines(model.sessionCampaignContentLines(60, 12)))
	if !strings.Contains(first, "1/3") || !strings.Contains(first, "SCENE") || !strings.Contains(first, "Opening image") {
		t.Fatalf("first beat:\n%s", first)
	}
	model = model.updatePrepPaneKey("j")
	second := stripANSIForTest(renderContentLines(model.sessionCampaignContentLines(60, 12)))
	if !strings.Contains(second, "2/3") || !strings.Contains(second, "ENCOUNTER") || strings.Contains(second, "Opening image") {
		t.Fatalf("second beat:\n%s", second)
	}
	model = model.updatePrepPaneKey("d")
	if got := stripANSIForTest(renderContentLines(model.sessionCampaignContentLines(60, 12))); !strings.Contains(got, "DONE") {
		t.Fatalf("done state missing:\n%s", got)
	}
	model = model.updatePrepPaneKey("x")
	if got := stripANSIForTest(renderContentLines(model.sessionCampaignContentLines(60, 12))); !strings.Contains(got, "SKIPPED") || strings.Contains(got, "DONE") {
		t.Fatalf("skip state missing:\n%s", got)
	}
	if model.sessionPlannedNotes().Body != body {
		t.Fatal("run-sheet state changed source markdown")
	}
}

func TestBrowserRunSheetKeepsBeatAcrossResize(t *testing.T) {
	model := New()
	plan := domain.PlannedNotes{ID: "browser-sheet", Title: "The gate", Scope: model.workspace.Scope, Body: "# Scene: Arrival\nOpening.\n## Clue: Seal\n" + strings.Repeat("Wrapped clue text. ", 20)}
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)
	for index, entry := range model.navEntries() {
		if entry.Kind == NavPrep {
			model.setNavCursor(index)
			break
		}
	}
	model.selectedPlanID = plan.ID
	model.layout.Focus = prefs.PaneDetail
	model.width = 80
	model.movePrepBeat(1)

	before := stripANSIForTest(model.renderTreeDetailWidth(30))
	model.width = 120
	after := stripANSIForTest(model.renderTreeDetailWidth(60))
	if !strings.Contains(before, "2/2") || !strings.Contains(after, "2/2") || !strings.Contains(after, "Seal") {
		t.Fatalf("beat moved across resize:\nbefore=%s\nafter=%s", before, after)
	}
}

func stripANSIForTest(value string) string {
	return testANSI.ReplaceAllString(value, "")
}
