package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/fivecli"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

func rulesStubBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub binary uses a POSIX shell")
	}
	return writeRulesStub(t, t.TempDir())
}

func writeRulesStub(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "5e")
	script := `#!/bin/sh
case "$*" in
  "search fireball --limit 12 --json")
    printf '[{"kind":"spell","name":"Fireball","source":"PHB","score":9,"snippet":"a bright streak"}]\n'
    ;;
  "get spell Fireball --source PHB --json")
    printf '{"kind":"spell","name":"Fireball","source":"PHB","page":241,"text":"A bright streak flashes from your pointing finger."}\n'
    ;;
  "doctor --json")
    printf '{"dataPath":"/data","dataExists":true,"indexPath":"/cache/index.sqlite","indexExists":true,"current":true,"ready":true}\n'
    ;;
  *)
    printf 'unexpected: %s\n' "$*" >&2
    exit 9
    ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// runCmds executes a command tree, feeding every message back into the model.
// tea.Tick commands are run immediately rather than waited on.
func runCmds(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return model
	}
	deadline := time.Now().Add(10 * time.Second)
	pending := []tea.Cmd{cmd}
	for len(pending) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("commands did not settle")
		}
		next := pending[0]
		pending = pending[1:]
		if next == nil {
			continue
		}
		msg := next()
		switch typed := msg.(type) {
		case nil:
			continue
		case tea.BatchMsg:
			pending = append(pending, typed...)
		default:
			updated, follow := model.Update(msg)
			model = updated.(Model)
			pending = append(pending, follow)
		}
	}
	return model
}

func rulesSearchModel(t *testing.T, query string) Model {
	t.Helper()
	model := liveSessionModel(t)
	model.searchScope = searchsvc.RulesReference
	model.searching = true
	model.searchInput.SetValue(query)
	return model
}

func TestRulesSearchScopeQueriesFiveCLI(t *testing.T) {
	t.Setenv(fivecli.BinaryEnv, rulesStubBinary(t))
	model := rulesSearchModel(t, "fireball")

	model = runCmds(t, model, model.scheduleRulesSearch())

	if len(model.results) != 1 || model.results[0].Kind != searchsvc.KindRule {
		t.Fatalf("results: %#v", model.results)
	}
	if model.results[0].Title != "Fireball" {
		t.Fatalf("unexpected hit: %#v", model.results[0])
	}
	if !model.rulesStatus.loaded || !model.rulesStatus.ready {
		t.Fatalf("expected a cached ready diagnosis: %#v", model.rulesStatus)
	}
}

// Typing must not spend one subprocess per character: the keystroke only
// schedules a debounce tick, and a superseded tick is discarded.
func TestRulesSearchDebouncesKeystrokes(t *testing.T) {
	t.Setenv(fivecli.BinaryEnv, rulesStubBinary(t))
	model := rulesSearchModel(t, "fireba")
	model.rulesStatus = rulesStatus{loaded: true, ready: true, summary: "ready"}

	cmd := model.scheduleRulesSearch()
	if cmd == nil {
		t.Fatal("expected a debounce command")
	}
	stale, ok := cmd().(rulesDebounceMsg)
	if !ok {
		t.Fatalf("expected a debounce message, got %T", cmd())
	}
	if model.rulesSearchSeq != 0 {
		t.Fatal("scheduling must not run a lookup yet")
	}

	// A further keystroke supersedes the pending tick.
	model.searchInput.SetValue("fireball")
	model.scheduleRulesSearch()
	updated, follow := model.Update(stale)
	model = updated.(Model)
	if follow != nil {
		t.Fatal("a superseded debounce must not search")
	}
	if model.rulesSearchSeq != 0 {
		t.Fatal("a superseded debounce must not search")
	}

	model = runCmds(t, model, model.runRulesSearch("fireball"))
	if len(model.results) != 1 {
		t.Fatalf("results: %#v", model.results)
	}
}

// The diagnosis is a subprocess, so entering the scope must schedule it
// instead of blocking the update loop, and the result is cached per binary.
func TestRulesSetupHintUsesCachedDiagnosis(t *testing.T) {
	t.Setenv(fivecli.BinaryEnv, rulesStubBinary(t))
	model := rulesSearchModel(t, "")

	if hint := model.rulesSetupHint(); !strings.Contains(hint, "Checking") {
		t.Fatalf("hint before the diagnosis lands: %q", hint)
	}
	model = runCmds(t, model, model.scheduleRulesSearch())
	if !model.rulesStatus.loaded {
		t.Fatal("expected a cached diagnosis")
	}
	if model.ensureRulesStatus() != nil {
		t.Fatal("a cached diagnosis must not be re-run")
	}
	if hint := model.rulesSetupHint(); !strings.Contains(hint, "Type to search") {
		t.Fatalf("ready hint: %q", hint)
	}
}

func TestRuleResultsCarryReferenceAuthority(t *testing.T) {
	result := searchsvc.Result{Kind: searchsvc.KindRule, Title: "Fireball", RuleKind: "spell", RuleSource: "PHB"}
	if got := result.Authority().Label(); got != "REFERENCE" {
		t.Fatalf("rules hits must never read as canon, got %q", got)
	}
}

func TestLiveRulesPreviewReturnsToCapture(t *testing.T) {
	t.Setenv(fivecli.BinaryEnv, rulesStubBinary(t))
	model := rulesSearchModel(t, "fireball")
	model = runCmds(t, model, model.scheduleRulesSearch())

	updated, cmd := model.openSearchResult(model.results[0])
	model = updated.(Model)
	model = runCmds(t, model, cmd)
	if model.preview == nil || model.preview.Hop.Kind != hopRule {
		t.Fatal("expected a rule preview")
	}
	if model.session == nil {
		t.Fatal("rule preview must not end live capture")
	}
	if !strings.Contains(model.preview.ruleText, "bright streak") {
		t.Fatalf("preview body: %q", model.preview.ruleText)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("enter should dismiss the live rule preview")
	}
	if !model.sessionInput.Focused() {
		t.Fatal("capture input should regain focus")
	}
}

// Outside a session there is nowhere to jump, so Enter keeps the text on
// screen rather than discarding what the reader opened.
func TestBrowserRulePreviewStaysOpenOnEnter(t *testing.T) {
	t.Setenv(fivecli.BinaryEnv, rulesStubBinary(t))
	model := rulesSearchModel(t, "fireball")
	model.session = nil
	model = runCmds(t, model, model.scheduleRulesSearch())

	updated, cmd := model.openSearchResult(model.results[0])
	model = updated.(Model)
	model = runCmds(t, model, cmd)

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview == nil || model.preview.Hop.Kind != hopRule {
		t.Fatal("enter should keep the rule preview open")
	}
}
