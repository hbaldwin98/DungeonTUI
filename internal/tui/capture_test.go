package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

func captureModel(t *testing.T) Model {
	t.Helper()
	store := storage.NewJSON(filepath.Join(t.TempDir(), "workspace.json"))
	model := newModel(demoWorkspace(), store, nil)
	model.width = 80
	model.height = 24
	model.enterScope(model.workspace.Scope)
	return model
}

func typeCapture(t *testing.T, model Model, text string) Model {
	t.Helper()
	for _, char := range text {
		updated, _ := model.Update(tea.KeyPressMsg{Code: char, Text: string(char)})
		model = updated.(Model)
	}
	return model
}

func captureNotes(model Model) []domain.Record {
	return domain.UnfiledCaptures(model.workspace, model.workspace.Scope)
}

func TestQuickCaptureOneInvocationPlusTextAndSubmit(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	model = updated.(Model)
	if !model.capture.Open {
		t.Fatal("Ctrl+N should open quick capture from the browser")
	}
	model = typeCapture(t, model, "innkeeper lied")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)

	if model.capture.Open {
		t.Fatal("submitting should close the overlay")
	}
	notes := captureNotes(model)
	if len(notes) != 1 || notes[0].Body != "innkeeper lied" {
		t.Fatalf("captured notes = %#v", notes)
	}
	if notes[0].Authority != domain.Draft {
		t.Fatalf("capture must not be canon, got %q", notes[0].Authority)
	}
	if !strings.Contains(model.status, "not canon") {
		t.Fatalf("status should say the note is not canon: %q", model.status)
	}
}

func TestQuickCapturePersistsAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.json")
	store := storage.NewJSON(path)
	model := newModel(demoWorkspace(), store, nil)
	model.width, model.height = 80, 24
	model.enterScope(model.workspace.Scope)

	updated, _ := model.openQuickCapture()
	model = updated.(Model)
	model = typeCapture(t, model, "ledger is fake")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("capture should persist: %v", err)
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	notes := domain.UnfiledCaptures(reloaded, model.workspace.Scope)
	if len(notes) != 1 || notes[0].Body != "ledger is fake" {
		t.Fatalf("reloaded captures = %#v", notes)
	}
	if notes[0].Capture == nil || notes[0].Capture.Origin != "browser" {
		t.Fatalf("provenance lost across restart: %#v", notes[0].Capture)
	}
}

func TestQuickCaptureRecordsAvailableProvenance(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.startSession()
	model = updated.(Model)
	if model.session == nil {
		t.Fatal("expected a live session")
	}
	model.session.LocationName = "Hollowmere"
	model.session.LocationID = "loc-hollowmere"

	updated, _ = model.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	model = updated.(Model)
	model = typeCapture(t, model, "the bell rang twice")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)

	notes := captureNotes(model)
	if len(notes) != 1 {
		t.Fatalf("captured notes = %#v", notes)
	}
	provenance := notes[0].Capture
	if provenance == nil || provenance.Origin != "live play" {
		t.Fatalf("origin = %#v", provenance)
	}
	if provenance.SessionID != model.session.ID || provenance.LocationName != "Hollowmere" {
		t.Fatalf("session context lost: %#v", provenance)
	}
	if provenance.At.IsZero() {
		t.Fatal("capture should carry a timestamp")
	}
}

func TestQuickCaptureLeavesLivePlayUntouched(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.startSession()
	model = updated.(Model)
	model.sessionInput.SetValue("half typed transcript")
	entries := len(model.session.Entries)

	updated, _ = model.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	model = updated.(Model)
	model = typeCapture(t, model, "side thought")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)

	if model.session == nil {
		t.Fatal("capture must not end the session")
	}
	if got := model.sessionInput.Value(); got != "half typed transcript" {
		t.Fatalf("transcript draft changed: %q", got)
	}
	if len(model.session.Entries) != entries {
		t.Fatal("capture must not write a transcript entry")
	}
	if len(captureNotes(model)) != 1 {
		t.Fatal("expected the capture to be stored as a draft note")
	}
}

func TestQuickCaptureCancelRestoresExactPriorState(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.openSearch()
	model = updated.(Model)
	model.searchInput.SetValue("vale")
	model.selected = 1
	before := model.View().Content
	records := len(model.workspace.Records)

	updated, _ = model.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	model = updated.(Model)
	if !model.capture.Open || model.captureOrigin() != "search" {
		t.Fatalf("capture from search: open=%t origin=%q", model.capture.Open, model.captureOrigin())
	}
	model = typeCapture(t, model, "discard me")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)

	if model.capture.Open {
		t.Fatal("Esc should close the overlay")
	}
	if len(model.workspace.Records) != records {
		t.Fatal("cancelling must not store anything")
	}
	if !model.searching || model.searchInput.Value() != "vale" || model.selected != 1 {
		t.Fatalf("search state changed: searching=%t query=%q selected=%d", model.searching, model.searchInput.Value(), model.selected)
	}
	model.status = ""
	if after := model.View().Content; after != before {
		t.Fatal("cancelling should restore the exact prior view")
	}
}

func TestQuickCaptureRefusesEmptyTextAndKeepsOverlayOpen(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.openQuickCapture()
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)

	if !model.capture.Open {
		t.Fatal("an empty capture should keep the overlay open")
	}
	if len(captureNotes(model)) != 0 {
		t.Fatal("an empty capture should store nothing")
	}
	if !strings.Contains(model.status, "needs text") {
		t.Fatalf("status = %q", model.status)
	}
}

func TestQuickCaptureNeedsAScope(t *testing.T) {
	model := newModel(domain.Workspace{}, nil, nil)
	model.width, model.height = 80, 24
	updated, _ := model.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	model = updated.(Model)
	if model.capture.Open {
		t.Fatal("capture should not open without a world")
	}
	if !strings.Contains(model.status, "before capturing") {
		t.Fatalf("status = %q", model.status)
	}
}

func TestQuickCaptureRendersWithinNarrowTerminal(t *testing.T) {
	model := captureModel(t)
	model.width, model.height = 80, 24
	updated, _ := model.openQuickCapture()
	model = updated.(Model)
	view := model.View().Content
	for _, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(testANSI.ReplaceAllString(line, "")); width > model.width {
			t.Fatalf("line of width %d exceeds %d: %q", width, model.width, line)
		}
	}
	plain := testANSI.ReplaceAllString(view, "")
	for _, want := range []string{"QUICK CAPTURE", "Esc discards", "Enter save draft"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in narrow capture overlay:\n%s", want, plain)
		}
	}
}

func TestQuickCaptureMultilineAndMouseAreContained(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.openQuickCapture()
	model = updated.(Model)
	model = typeCapture(t, model, "first")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	model = updated.(Model)
	model = typeCapture(t, model, "second")
	if got := model.capture.Input.Value(); !strings.Contains(got, "first\nsecond") {
		t.Fatalf("shift+enter should add a newline, got %q", got)
	}

	updated, _ = model.Update(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	model = updated.(Model)
	if !model.capture.Open {
		t.Fatal("clicks must not fall through to the surface behind capture")
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model = updated.(Model)
	notes := captureNotes(model)
	if len(notes) != 1 || !strings.Contains(notes[0].Body, "first\nsecond") {
		t.Fatalf("multiline capture = %#v", notes)
	}
}

func TestCampaignHomeAndPaletteSurfaceUnfiledCaptures(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.openQuickCapture()
	model = updated.(Model)
	model = typeCapture(t, model, "unfiled thought")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)

	plain := testANSI.ReplaceAllString(model.View().Content, "")
	for _, want := range []string{"UNFILED CAPTURES · 1", "unfiled thought"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q on campaign home:\n%s", want, plain)
		}
	}

	model.selectedID = captureNotes(model)[0].ID
	found := false
	for _, command := range model.paletteCommands() {
		if command.ID == "capture.file" {
			found = true
			if !commandEnabled(command) {
				t.Fatalf("file command should be enabled for a selected capture: %#v", command)
			}
			if !strings.Contains(command.Label, "1 unfiled") {
				t.Fatalf("file command label = %q", command.Label)
			}
		}
	}
	if !found {
		t.Fatal("browser palette should offer filing a capture")
	}
}

func TestFileSelectedCaptureLeavesInboxWithoutPromoting(t *testing.T) {
	model := captureModel(t)
	updated, _ := model.openQuickCapture()
	model = updated.(Model)
	model = typeCapture(t, model, "file me")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(Model)
	model.selectedID = captureNotes(model)[0].ID

	updated, _ = model.fileSelectedCapture()
	model = updated.(Model)
	if len(captureNotes(model)) != 0 {
		t.Fatal("filed capture should leave the inbox")
	}
	filed, ok := recordByID(model.workspace.Records, model.selectedID)
	if !ok {
		t.Fatal("filing must keep the note")
	}
	if filed.Authority != domain.Draft {
		t.Fatalf("filing must not promote to canon, got %q", filed.Authority)
	}

	updated, _ = model.fileSelectedCapture()
	model = updated.(Model)
	if !strings.Contains(model.status, "unfiled capture") {
		t.Fatalf("filing twice status = %q", model.status)
	}
}

func TestPostSessionReviewShowsSessionCapturesAsEvidence(t *testing.T) {
	model := reconciliationInboxModel()
	note, err := domain.NewCaptureNote("the seal hums at dusk",
		domain.CaptureContext{Origin: "live play", SessionID: "sit-review", SessionTitle: "The Broken Seal", At: time.Now().UTC()},
		model.workspace.Scope)
	if err != nil {
		t.Fatal(err)
	}
	model.workspace.Records = append(model.workspace.Records, note)

	updated, _ := model.openReconciliation()
	model = updated.(Model)
	plain := testANSI.ReplaceAllString(model.View().Content, "")
	for _, want := range []string{"CAPTURED THIS SESSION · 1 UNFILED", "the seal hums at dusk", "2 remaining"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in post-session review:\n%s", want, plain)
		}
	}
	if len(model.unresolvedReconciliationItems()) != 2 {
		t.Fatal("captures must not enter the reconciliation queue")
	}
}

func TestPostSessionReviewMouseGeometrySurvivesCaptureEvidence(t *testing.T) {
	model := reconciliationInboxModel()
	note, _ := domain.NewCaptureNote("aside", domain.CaptureContext{SessionID: "sit-review"}, model.workspace.Scope)
	model.workspace.Records = append(model.workspace.Records, note)
	model.height = 40
	updated, _ := model.openReconciliation()
	model = updated.(Model)

	height := model.reconciliationOverlayHeight()
	top := max(0, (model.height-height)/2)
	updated, _ = model.Update(tea.MouseClickMsg{X: model.width / 2, Y: top + height - 2, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.workspace.Reconciliations[0].Items[1].Status != domain.ReconDeferred {
		t.Fatalf("defer button moved: status=%q", model.workspace.Reconciliations[0].Items[1].Status)
	}
}
