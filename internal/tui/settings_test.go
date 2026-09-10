package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/fivecli"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

type recordingPrefs struct {
	layout prefs.Layout
	saved  int
}

func (p *recordingPrefs) Load() (prefs.Layout, error) { return p.layout, nil }

func (p *recordingPrefs) Save(layout prefs.Layout) error {
	p.layout = layout
	p.saved++
	return nil
}

func typeInto(model Model, text string) Model {
	for _, r := range text {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
		model = updated.(Model)
	}
	return model
}

// The 5e binary is configured in the app, not only on PATH, so a workstation
// that keeps the tool elsewhere does not have to relaunch through a wrapper.
func TestSettingsSaveFiveCLIBinaryPath(t *testing.T) {
	stub := rulesStubBinary(t)
	store := &recordingPrefs{}
	model := rulesSearchModel(t, "")
	model.session = nil
	model.searching = false
	model.prefs = store
	model.width, model.height = 120, 40

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: ',', Text: ","}))
	model = updated.(Model)
	if !model.settingsOpen {
		t.Fatal("',' should open settings from the browser")
	}

	model = typeInto(model, stub)
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.layout.FiveCLIBinary != stub {
		t.Fatalf("configured binary: %q", model.layout.FiveCLIBinary)
	}
	if store.saved == 0 || store.layout.FiveCLIBinary != stub {
		t.Fatalf("preference not persisted: %#v", store)
	}
	if model.fivecliAdapter().Binary != stub {
		t.Fatal("lookups should use the configured binary")
	}

	model = runCmds(t, model, cmd)
	if !model.rulesStatus.loaded || !model.rulesStatus.ready {
		t.Fatalf("saving should re-diagnose the tool: %#v", model.rulesStatus)
	}
	rendered := testANSI.ReplaceAllString(model.View().Content, "")
	if !strings.Contains(rendered, "SETTINGS") || !strings.Contains(rendered, stub) {
		t.Fatalf("settings overlay should show the configured path:\n%s", rendered)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.settingsOpen {
		t.Fatal("esc should close settings")
	}
}

// The configured path wins over PATH, and changing it invalidates the cached
// diagnosis so a stale "ready" cannot outlive the tool it described.
func TestSettingsBinaryOverridesEnvironmentAndInvalidatesStatus(t *testing.T) {
	t.Setenv(fivecli.BinaryEnv, "/nonexistent/5e")
	stub := rulesStubBinary(t)
	model := rulesSearchModel(t, "")
	model.prefs = &recordingPrefs{}
	model.rulesStatus = rulesStatus{loaded: true, ready: true, summary: "stale"}

	model.settingsOpen = true
	model.settingsInput.SetValue(stub)
	updated, cmd := model.saveFiveCLIBinary()
	model = updated.(Model)
	if model.rulesStatus.loaded {
		t.Fatal("changing the path must invalidate the cached diagnosis")
	}

	model = runCmds(t, model, cmd)
	if !model.rulesStatus.ready || model.rulesStatus.binary != stub {
		t.Fatalf("diagnosis after save: %#v", model.rulesStatus)
	}
	model.searchInput.SetValue("fireball")
	model = runCmds(t, model, model.runRulesSearch("fireball"))
	if len(model.results) != 1 {
		t.Fatalf("configured binary should serve lookups: %#v", model.results)
	}
}

func TestSettingsReachableFromRulesSearch(t *testing.T) {
	model := rulesSearchModel(t, "fireball")
	model.width, model.height = 120, 40

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if !model.settingsOpen || !model.settingsFromSearch {
		t.Fatal("Ctrl+G should open settings over the rules search")
	}
	if !model.searching || model.searchScope != searchsvc.RulesReference {
		t.Fatal("settings must not discard the search behind it")
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)
	if model.settingsOpen || !model.searchInput.Focused() {
		t.Fatal("closing settings should return to the search input")
	}
	if !strings.Contains(model.renderSearchOverlay(), "Ctrl+G") {
		t.Fatal("the rules scope should advertise the 5e path key")
	}
}
