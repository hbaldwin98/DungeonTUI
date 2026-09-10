package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

func navigationModel(t *testing.T) Model {
	t.Helper()
	model := newModel(demoWorkspace(), nil, nil)
	model.width, model.height = 120, 40
	model.enterScope(model.workspace.Scope)
	return model
}

func focusRecordList(t *testing.T, model *Model, entityType domain.EntityType) {
	t.Helper()
	for index, entry := range model.navEntries() {
		if entry.Kind == NavType && entry.Type == entityType {
			model.setNavCursor(index)
		}
	}
	model.setBrowserFocus(prefs.PaneList)
}

func hopTo(t *testing.T, model Model, recordID string) Model {
	t.Helper()
	updated, _ := model.jumpToHop(detailHop{Kind: hopWiki, Label: recordID, RecordID: recordID})
	return updated.(Model)
}

func TestFollowThreeLinksAndReturnStepByStep(t *testing.T) {
	model := navigationModel(t)
	trail := []string{"npc-captain-vale", "npc-father-merrow", "draft-sister-elayne"}
	for _, id := range trail {
		model = hopTo(t, model, id)
	}
	if model.selectedID != "draft-sister-elayne" {
		t.Fatalf("selected = %q", model.selectedID)
	}
	if len(model.browserHistory) != 3 {
		t.Fatalf("history depth = %d", len(model.browserHistory))
	}
	if label := model.browserTrailLabel(); label != "← 3 back" {
		t.Fatalf("trail label = %q", label)
	}

	for index := len(trail) - 2; index >= 0; index-- {
		updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		model = updated.(Model)
		if model.selectedID != trail[index] {
			t.Fatalf("step back to %q, got %q", trail[index], model.selectedID)
		}
	}
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if len(model.browserHistory) != 0 || len(model.browserForward) != 3 {
		t.Fatalf("back=%d forward=%d", len(model.browserHistory), len(model.browserForward))
	}
	if !strings.Contains(model.browserTrailLabel(), "3 fwd →") {
		t.Fatalf("trail label = %q", model.browserTrailLabel())
	}
}

func TestForwardRetracesTheTrailAndBranchingDropsIt(t *testing.T) {
	model := navigationModel(t)
	model = hopTo(t, model, "npc-captain-vale")
	model = hopTo(t, model, "npc-father-merrow")

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if model.selectedID != "npc-captain-vale" {
		t.Fatalf("back landed on %q", model.selectedID)
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	model = updated.(Model)
	if model.selectedID != "npc-father-merrow" {
		t.Fatalf("forward landed on %q", model.selectedID)
	}
	if len(model.browserForward) != 0 {
		t.Fatalf("forward stack = %d", len(model.browserForward))
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	model = hopTo(t, model, "location-monastery")
	if len(model.browserForward) != 0 {
		t.Fatal("navigating somewhere new must drop the forward trail")
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt})
	model = updated.(Model)
	if !strings.Contains(model.status, "No later browser location") {
		t.Fatalf("status = %q", model.status)
	}
}

func TestHistoryStaysBounded(t *testing.T) {
	model := navigationModel(t)
	ids := []string{"npc-captain-vale", "npc-father-merrow", "location-monastery"}
	for index := 0; index < maxBrowserHistoryEntries*2; index++ {
		model = hopTo(t, model, ids[index%len(ids)])
	}
	if len(model.browserHistory) > maxBrowserHistoryEntries {
		t.Fatalf("history grew to %d", len(model.browserHistory))
	}
}

func TestBackRestoresSearchQueryAndRow(t *testing.T) {
	model := navigationModel(t)
	updated, _ := model.openSearch()
	model = updated.(Model)
	model.searchInput.SetValue("e")
	model.refreshResults()
	if len(model.results) < 3 {
		t.Fatalf("expected several results, got %d", len(model.results))
	}
	model.selected = 2
	target := model.results[2]

	updated, _ = model.openSearchResult(target)
	model = updated.(Model)
	if model.searching {
		t.Fatal("inspecting a result should close the search overlay")
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if !model.searching {
		t.Fatal("back should reopen search")
	}
	if model.searchInput.Value() != "e" || model.selected != 2 {
		t.Fatalf("restored query=%q row=%d", model.searchInput.Value(), model.selected)
	}
	if !strings.Contains(model.status, "search · e") {
		t.Fatalf("status = %q", model.status)
	}
}

func TestBackRestoresSearchScopeAndProposalToggle(t *testing.T) {
	model := navigationModel(t)
	updated, _ := model.openSearch()
	model = updated.(Model)
	model.searchScope = searchsvc.EntireLibrary
	model.includeIdeas = true
	model.searchInput.SetValue("vale")
	model.refreshResults()
	model.pushBrowserLocation()

	model.searching = false
	model.searchScope = searchsvc.CurrentCampaign
	model.includeIdeas = false

	model.restoreBrowserLocation()
	if model.searchScope != searchsvc.EntireLibrary || !model.includeIdeas {
		t.Fatalf("scope=%v ideas=%t", model.searchScope, model.includeIdeas)
	}
}

func TestClosingPreviewReturnsToItsSearchOrigin(t *testing.T) {
	model := navigationModel(t)
	updated, _ := model.openSearch()
	model = updated.(Model)
	model.searchInput.SetValue("vale")
	model.refreshResults()
	model.selected = 0

	origin := model.snapshotSearchLocation()
	model.searching = false
	model.openPreviewFrom(detailHop{Kind: hopWiki, Label: "Captain Vale", RecordID: "npc-captain-vale"}, origin)

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("Esc should close the preview")
	}
	if !model.searching || model.searchInput.Value() != "vale" {
		t.Fatalf("search origin not restored: searching=%t query=%q", model.searching, model.searchInput.Value())
	}
}

func TestNestedPreviewsUnwindOneStepAtATime(t *testing.T) {
	model := navigationModel(t)
	model.openPreview(detailHop{Kind: hopWiki, Label: "Captain Vale", RecordID: "npc-captain-vale"})
	model.openPreview(detailHop{Kind: hopWiki, Label: "Father Merrow", RecordID: "npc-father-merrow"})
	if model.preview == nil || model.preview.parent == nil {
		t.Fatal("expected a nested preview")
	}

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.preview == nil || model.preview.Hop.RecordID != "npc-captain-vale" {
		t.Fatalf("closing a nested preview should reveal its parent: %#v", model.preview)
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("closing the last preview should return to the browser")
	}
}

func TestDeleteSelectsTheNextRowThenThePreviousOne(t *testing.T) {
	model := navigationModel(t)
	focusRecordList(t, &model, domain.NPC)
	records := model.listRecords()
	if len(records) < 3 {
		t.Fatalf("expected several NPCs, got %d", len(records))
	}

	model.selectedID = records[1].ID
	model.cursor = 1
	model.deleteSelected()
	if model.selectedID != records[2].ID {
		t.Fatalf("delete should select the following row, got %q", model.selectedID)
	}

	remaining := model.listRecords()
	model.selectedID = remaining[len(remaining)-1].ID
	model.deleteSelected()
	if model.selectedID != remaining[len(remaining)-2].ID {
		t.Fatalf("deleting the last row should select the previous one, got %q", model.selectedID)
	}

	for len(model.listRecords()) > 0 {
		model.selectedID = model.listRecords()[0].ID
		model.deleteSelected()
	}
	if model.selectedID != "" {
		t.Fatalf("an empty list should leave nothing selected, got %q", model.selectedID)
	}
}

func TestEditKeepsListRowDetailScrollAndLinkCursor(t *testing.T) {
	model := navigationModel(t)
	focusRecordList(t, &model, domain.NPC)
	records := model.listRecords()
	model.selectedID = records[0].ID
	model.cursor = 0
	model.historyCursor = 1
	model.ensureDetailView().offset = 4

	updated, _ := model.openEditor(false)
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model = updated.(Model)

	if model.editing {
		t.Fatalf("save should close the editor: %q", model.status)
	}
	if model.selectedID != records[0].ID || model.cursor != 0 {
		t.Fatalf("edit moved the selection: id=%q cursor=%d", model.selectedID, model.cursor)
	}
	if model.historyCursor != 1 {
		t.Fatalf("edit reset the detail link cursor to %d", model.historyCursor)
	}
	if model.ensureDetailView().offset != 4 {
		t.Fatalf("edit reset the detail scroll to %d", model.ensureDetailView().offset)
	}
}

func TestBackToADeletedTargetFallsBackDeterministically(t *testing.T) {
	model := navigationModel(t)
	focusRecordList(t, &model, domain.NPC)
	records := model.listRecords()
	model = hopTo(t, model, records[1].ID)
	model = hopTo(t, model, records[0].ID)

	model.selectedID = records[1].ID
	model.deleteSelected()
	model.selectedID = records[0].ID

	model.restoreBrowserLocation()
	if model.selectedID == "" || model.selectedID == records[1].ID {
		t.Fatalf("back should land on a live row, got %q", model.selectedID)
	}
	if !strings.Contains(model.status, "is gone") {
		t.Fatalf("status should say the target vanished: %q", model.status)
	}
	if record := model.selectedRecord(); record == nil {
		t.Fatal("the fallback selection must resolve to a real record")
	}
}

func TestCrossBranchHopAndReturnKeepsBothBranches(t *testing.T) {
	model := navigationModel(t)
	focusRecordList(t, &model, domain.NPC)
	model.selectedID = model.listRecords()[0].ID
	npcNav := model.navCursor

	model = hopTo(t, model, "location-monastery")
	if model.currentNav().Type != domain.Location {
		t.Fatalf("hop should move to the location branch, nav=%q", model.currentNav().Type)
	}

	model.restoreBrowserLocation()
	if model.navCursor != npcNav {
		t.Fatalf("back should return to the NPC branch: nav=%d want %d", model.navCursor, npcNav)
	}
	if model.currentNav().Type != domain.NPC {
		t.Fatalf("nav type = %q", model.currentNav().Type)
	}
}

func TestSwitchingCampaignsDropsTheTrail(t *testing.T) {
	model := navigationModel(t)
	model = hopTo(t, model, "npc-father-merrow")
	model.restoreBrowserLocation()
	if len(model.browserHistory) != 0 || len(model.browserForward) == 0 {
		t.Fatal("expected a forward trail to exist before switching")
	}

	model.enterScope(model.workspace.Scope)
	if len(model.browserHistory) != 0 || len(model.browserForward) != 0 {
		t.Fatalf("campaign entry must isolate history: back=%d forward=%d",
			len(model.browserHistory), len(model.browserForward))
	}
}

func TestBreadcrumbShowsTrailDepthAndSurvivesResize(t *testing.T) {
	model := navigationModel(t)
	focusRecordList(t, &model, domain.NPC)
	model = hopTo(t, model, "npc-father-merrow")

	for _, width := range []int{80, 120} {
		model.width = width
		model.height = 24
		plain := testANSI.ReplaceAllString(model.View().Content, "")
		// The detail pane wraps the breadcrumb at narrow widths; only its
		// presence and leading section are asserted here.
		if !strings.Contains(plain, "PATH  The Ashen Crown / NPCs") {
			t.Fatalf("breadcrumb missing at width %d:\n%s", width, plain)
		}
		if !strings.Contains(plain, "← 1 back") {
			t.Fatalf("trail depth missing at width %d:\n%s", width, plain)
		}
		if model.selectedID != "npc-father-merrow" {
			t.Fatalf("resize moved the selection at width %d", width)
		}
	}
}

func TestLiveInspectionStillReturnsToCapture(t *testing.T) {
	model := liveSessionModel(t)
	model.searching = true
	model.sessionInput.Blur()
	model.searchInput.SetValue("vale")
	model.refreshResults()

	updated, _ := model.openSearchResult(model.results[0])
	model = updated.(Model)
	if model.preview == nil {
		t.Fatal("expected a live preview")
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)

	if model.searching {
		t.Fatal("live inspection must return to capture, not the search overlay")
	}
	if model.session == nil || !model.sessionInput.Focused() {
		t.Fatal("capture should regain focus after live inspection")
	}
}
