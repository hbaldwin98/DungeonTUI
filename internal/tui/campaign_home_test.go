package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

func TestCampaignEntryOpensHome(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 80
	model.height = 24
	model.enterScope(model.workspace.Scope)

	if model.currentNav().Kind != NavHome {
		t.Fatalf("nav kind = %q", model.currentNav().Kind)
	}
	view := model.View().Content
	for _, want := range []string{"Home", "NEXT ACTION", "CAMPAIGN HOME", "Draft prep", "No upcoming prep yet"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in home view", want)
		}
	}
}

func TestCampaignHomeKeyboardCaptureOpensQuickCapture(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.enterScope(model.workspace.Scope)
	model.setBrowserFocus(prefs.PaneList)
	model.cursor = 3

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.capture.Open {
		t.Fatal("capture action should open the quick capture overlay")
	}
}

func TestCampaignHomeMouseRunsClickedAction(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 120
	model.height = 40
	model.enterScope(model.workspace.Scope)

	var x, y int
	found := false
	for _, region := range model.browserRegions() {
		if region.Pane == prefs.PaneList && len(region.HomeRows) == 4 {
			x = region.MinX + 2
			y = region.Offset + 3
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected home action hit region")
	}
	updated, _ := model.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	model = updated.(Model)
	if !model.capture.Open {
		t.Fatal("clicked capture action should open the quick capture overlay")
	}
}

func TestCampaignHomePrepActionsCreateOrResumePlan(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		model := newModel(demoWorkspace(), nil, nil)

		updated, _ := model.activateHomeAction(0)
		model = updated.(Model)
		if !model.planning || model.planDraft.ID == "" {
			t.Fatalf("create prep action did not open a new plan: %#v", model.planDraft)
		}
	})

	t.Run("resume", func(t *testing.T) {
		model := newModel(demoWorkspace(), nil, nil)
		plan := domain.PlannedNotes{ID: "next-plan", Title: "Next sit", Scope: model.workspace.Scope, CreatedAt: time.Now().UTC()}
		model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)

		updated, _ := model.activateHomeAction(0)
		model = updated.(Model)
		if !model.planning || model.planDraft.ID != plan.ID || model.selectedPlanID != plan.ID {
			t.Fatalf("resume prep action selected the wrong plan: selected=%q draft=%#v", model.selectedPlanID, model.planDraft)
		}
	})
}

func TestCampaignHomeReviewActionHandlesEmptyAndPendingReview(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	updated, _ := model.activateHomeAction(2)
	model = updated.(Model)
	if model.reconciling || model.status != "Nothing waiting" {
		t.Fatalf("disabled review action = reconciling %t, status %q", model.reconciling, model.status)
	}

	session := domain.SessionRecord{ID: "sit", Scope: model.workspace.Scope}
	model.workspace.Sessions = append(model.workspace.Sessions, session)
	model.workspace.Reconciliations = append(model.workspace.Reconciliations, domain.ReconciliationRecord{
		ID: "review", SessionID: session.ID, CreatedAt: time.Now().UTC(),
		Items: []domain.ReconciliationItem{{Status: domain.ReconPending}},
	})
	updated, _ = model.activateHomeAction(2)
	model = updated.(Model)
	if !model.reconciling || model.reconIndex != 0 {
		t.Fatalf("pending review action = reconciling %t, index %d", model.reconciling, model.reconIndex)
	}
}

func TestCampaignHomeStartUsesNextPlan(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	plan := domain.PlannedNotes{ID: "next-plan", Title: "Next sit", Scope: model.workspace.Scope, CreatedAt: time.Now().UTC()}
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)

	updated, _ := model.activateHomeAction(1)
	model = updated.(Model)
	if model.session == nil || model.session.PlannedNotesID != plan.ID {
		t.Fatalf("started session = %#v", model.session)
	}
}
