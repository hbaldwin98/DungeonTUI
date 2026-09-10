package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

// recordStatusExpiry swaps in a stub that records each scheduled expiry and
// hands back a command that delivers it immediately, then restores the
// binary-wide stub from TestMain.
func recordStatusExpiry(t *testing.T) *[]time.Duration {
	t.Helper()
	var lifetimes []time.Duration
	previous := scheduleStatusExpiry
	scheduleStatusExpiry = func(lifetime time.Duration, seq uint64) tea.Cmd {
		lifetimes = append(lifetimes, lifetime)
		return func() tea.Msg { return statusExpireMsg{seq: seq} }
	}
	t.Cleanup(func() { scheduleStatusExpiry = previous })
	return &lifetimes
}

func TestStatusExpiresOnItsOwnWithoutDismissal(t *testing.T) {
	lifetimes := recordStatusExpiry(t)
	model := New()
	model.width, model.height = 120, 40
	next, cmd := trackStatus("", withStatus(model, "Filed capture"), nil)
	model = next.(Model)
	if cmd == nil {
		t.Fatal("a new status must schedule its own expiry")
	}
	seq := model.statusSeq

	updated, _ := model.Update(cmd())
	if updated.(Model).status != "" {
		t.Fatal("the scheduled expiry clears the status")
	}
	if len(*lifetimes) != 1 || (*lifetimes)[0] != statusLifetime {
		t.Fatalf("ordinary feedback uses the short lifetime: %v", *lifetimes)
	}
	_ = seq
}

func TestProblemStatusesGetTheLongerLifetime(t *testing.T) {
	lifetimes := recordStatusExpiry(t)
	model := New()
	trackStatus("", withStatus(model, "Filing rolled back: disk full"), nil)
	if len(*lifetimes) != 1 || (*lifetimes)[0] != statusErrorLifetime {
		t.Fatalf("problems stay up longer: %v", *lifetimes)
	}
}

func TestStaleExpiryLeavesANewerStatusAlone(t *testing.T) {
	model := New()
	next, _ := trackStatus("", withStatus(model, "First"), nil)
	model = next.(Model)
	stale := model.statusSeq
	next, _ = trackStatus(model.status, withStatus(model, "Second"), nil)
	model = next.(Model)

	updated, _ := model.Update(statusExpireMsg{seq: stale})
	if updated.(Model).status != "Second" {
		t.Fatalf("an older timer must not clear a newer message: %q", updated.(Model).status)
	}
}

func withStatus(model Model, text string) Model {
	model.status = text
	return model
}

// A status set deep inside an ordinary key handler must be timed by the
// Update wrapper, without that handler knowing about timers. Arming delete is
// such a handler: it only assigns m.status.
func TestUpdateTimesStatusesSetAnywhere(t *testing.T) {
	lifetimes := recordStatusExpiry(t)
	model := New()
	model.width, model.height = 120, 40
	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			model.selectRecord(record)
		}
	}
	model.layout.Focus = prefs.PaneList
	model.status = ""
	before := model.statusSeq

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	if model.status == "" {
		t.Fatal("arming delete should ask for confirmation in the status")
	}
	if model.statusSeq == before || cmd == nil || len(*lifetimes) != 1 {
		t.Fatalf("a key handler's status must be timed by Update: seq %d→%d, %d timers", before, model.statusSeq, len(*lifetimes))
	}
}

func TestUnchangedStatusDoesNotRestartItsTimer(t *testing.T) {
	lifetimes := recordStatusExpiry(t)
	model := withStatus(New(), "Opened Captain Vale")
	next, cmd := trackStatus("Opened Captain Vale", model, nil)
	if next.(Model).statusSeq != model.statusSeq || cmd != nil || len(*lifetimes) != 0 {
		t.Fatal("re-setting the same text must leave the running timer alone")
	}
}

func TestProblemsAreStyledAsWarningsAndLastLonger(t *testing.T) {
	if classifyStatus("Filing rolled back: disk full") != statusProblem {
		t.Fatal("a rollback is a problem")
	}
	if classifyStatus("Filed capture · still a draft") != statusInfo {
		t.Fatal("ordinary feedback is information")
	}
	if renderStatus("Opened Vale") != "Opened Vale" {
		t.Fatal("information is plain text, not a colored surface")
	}
	if renderStatus("Save failed") == "Save failed" {
		t.Fatal("a problem uses the warning foreground")
	}
	if strings.Contains(renderStatus("Save failed"), "48;") {
		t.Fatal("status must never paint a background")
	}
}

func TestFooterDropsHintsBeforeStatus(t *testing.T) {
	help := "/ search   n new   Enter open   : commands"
	wide := footerWithStatus("Opened Captain Vale", help, 120)
	if !strings.Contains(wide, help) {
		t.Fatalf("wide footer keeps hints: %q", wide)
	}
	narrow := footerWithStatus("That entity is gone · showing Sister Elayne", help, 60)
	if strings.Contains(narrow, "commands") || !strings.Contains(narrow, "Sister Elayne") {
		t.Fatalf("narrow footer keeps the status and drops hints: %q", narrow)
	}
	if footerWithStatus("", help, 40) != help {
		t.Fatal("no status leaves hints untouched")
	}
}

func TestStatusNeverWidensTheBrowserView(t *testing.T) {
	for _, width := range []int{80, 100, 140} {
		model := New()
		model.width, model.height = width, 30
		model.status = strings.Repeat("A long status message about remaining work ", 4)
		for index, line := range strings.Split(model.View().Content, "\n") {
			if got := lipgloss.Width(stripANSIForTest(line)); got > width {
				t.Fatalf("width %d line %d is %d wide", width, index, got)
			}
		}
	}
}

func TestPresetFromThePaletteSwitchesPersistsAndRestores(t *testing.T) {
	model := New()
	model.width, model.height = 140, 40
	store := prefs.NewJSON(t.TempDir() + "/prefs.json")
	model.prefs = store

	var found bool
	for _, command := range model.paletteCommands() {
		if command.ID == "layout.prep" {
			found = true
		}
	}
	if !found {
		t.Fatal("the browser palette offers presets")
	}
	updated, _ := presetExecutors()["layout.prep"](model)
	model = updated.(Model)
	if model.layout.Preset != "prep" || !strings.Contains(model.status, "restore my layout") {
		t.Fatalf("preset status should say how to undo: %q", model.status)
	}
	loaded, err := store.Load()
	if err != nil || loaded.Preset != "prep" {
		t.Fatalf("the preset persists locally: %#v %v", loaded.Preset, err)
	}
	for _, command := range model.paletteCommands() {
		if command.ID == "layout.prep" && !strings.Contains(command.Label, "(current)") {
			t.Fatalf("the palette marks the current preset: %q", command.Label)
		}
	}

	updated, _ = presetExecutors()["layout.restore"](model)
	model = updated.(Model)
	if model.layout.Preset != "" || model.status != "Restored your layout" {
		t.Fatalf("restore = %q %q", model.layout.Preset, model.status)
	}
	for _, command := range model.paletteCommands() {
		if command.ID == "layout.restore" && commandEnabled(command) {
			t.Fatal("restore is disabled once nothing is applied")
		}
	}
}

func TestPresetsDoNotHideRequiredFactsInAnyPreset(t *testing.T) {
	for _, preset := range []string{"layout.browse", "layout.prep", "layout.review"} {
		for _, width := range []int{80, 140} {
			model := New()
			model.width, model.height = width, 30
			for _, record := range model.workspace.Records {
				if record.ID == "npc-captain-vale" {
					model.selectRecord(record)
				}
			}
			updated, _ := presetExecutors()[preset](model)
			model = updated.(Model)
			view := stripANSIForTest(model.View().Content)
			for _, want := range []string{"NPCs", "Captain Vale", "NPC  ● CANON"} {
				if !strings.Contains(view, want) {
					t.Fatalf("%s at %d hides %q: %s", preset, width, want, view)
				}
			}
		}
	}
}

func TestRunPresetFromTheBrowserShapesTheNextLiveSit(t *testing.T) {
	model := New()
	model.width, model.height = 80, 30
	model.layout.Focus = prefs.PaneCampaign
	updated, _ := presetExecutors()["layout.run"](model)
	model = updated.(Model)
	if model.layout.SessionLeafVisible(prefs.PaneCampaign) {
		t.Fatal("run at 80 columns hides the campaign pane")
	}
	updated, _ = model.startSession()
	model = updated.(Model)
	view := stripANSIForTest(model.View().Content)
	if !strings.Contains(view, "CURRENT SCENE") || !strings.Contains(view, "SESSION TRANSCRIPT") {
		t.Fatalf("live play keeps scene and transcript: %s", view)
	}
	_ = domain.Canon
}
