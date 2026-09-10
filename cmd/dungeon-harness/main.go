package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/tui"
)

// Exit codes: every scenario passed, a scenario failed, or the command line
// was wrong and nothing ran.
const (
	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
)

// allScenarios is the order -scenario all runs after the journeys.
var allScenarios = []string{"smoke", "resize", "search", "scroll"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the whole command, taking its arguments and output streams so
// tests drive it exactly as a shell does.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dungeon-harness", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outDir := flags.String("out", "testdata/harness", "directory for screencaps")
	width := flags.Int("width", 100, "terminal width")
	height := flags.Int("height", 30, "terminal height")
	scenario := flags.String("scenario", "smoke", "built-in scenario: smoke|scroll|resize|search|journeys|all")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	scenarios := builtinScenarios(*width, *height)
	names, journeys, err := scenarioPlan(*scenario, scenarios)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if journeys && !runJourneys(filepath.Join(*outDir, "journeys"), stdout, stderr) {
		return exitFail
	}
	for _, name := range names {
		if !runScenario(filepath.Join(*outDir, name), scenarios[name], stdout, stderr) {
			return exitFail
		}
	}
	return exitOK
}

// scenarioPlan resolves -scenario into the scripted scenarios to run and
// whether the journeys run first. An unknown name is refused before anything
// runs, so a typo never leaves partial screencaps behind.
func scenarioPlan(name string, scenarios map[string]tui.Scenario) ([]string, bool, error) {
	switch name {
	case "all":
		return allScenarios, true, nil
	case "journeys":
		return nil, true, nil
	}
	if _, ok := scenarios[name]; !ok {
		return nil, false, fmt.Errorf("unknown scenario %q", name)
	}
	return []string{name}, false, nil
}

func builtinScenarios(width, height int) map[string]tui.Scenario {
	return map[string]tui.Scenario{
		"smoke": {
			Name: "smoke", Width: width, Height: height,
			Steps: []tui.ScenarioStep{
				{Op: "screencap", Stem: "smoke-01-browser"},
				{Op: "key", Text: "s"},
				{Op: "type", Text: "The party enters the crypt"},
				{Op: "key", Text: "enter"},
				{Op: "screencap", Stem: "smoke-02-session"},
				{Op: "key", Text: "ctrl+e"},
				{Op: "screencap", Stem: "smoke-03-ended"},
			},
		},
		"scroll": {
			Name: "scroll", Width: width, Height: height,
			Steps: []tui.ScenarioStep{
				{Op: "key", Text: "s"},
			},
		},
		"resize": {
			Name: "resize", Width: width, Height: height,
			Steps: []tui.ScenarioStep{
				{Op: "screencap", Stem: "resize-01-browser"},
				{Op: "key", Text: "s"},
				{Op: "screencap", Stem: "resize-02-session"},
				{Op: "drag", X: 33, Y: 4, ToX: 45, ToY: 4},
				{Op: "screencap", Stem: "resize-03-vertical"},
				{Op: "drag", X: 70, Y: 12, ToX: 70, ToY: 16},
				{Op: "screencap", Stem: "resize-04-horizontal"},
				{Op: "resize", X: 80, Y: 24},
				{Op: "screencap", Stem: "resize-05-window"},
			},
		},
		"search": {
			Name: "search", Width: width, Height: height,
			Steps: []tui.ScenarioStep{
				{Op: "key", Text: "/"},
				{Op: "type", Text: "Vale"},
				{Op: "screencap", Stem: "search-01-results"},
				{Op: "key", Text: "ctrl+a"},
				{Op: "screencap", Stem: "search-02-with-ai"},
			},
		},
	}
}

// runScenario runs one scripted scenario, prints its line and screencaps,
// and reports whether it ran and its final frame fills the terminal.
func runScenario(dir string, scenario tui.Scenario, stdout, stderr io.Writer) bool {
	h, caps, err := tui.RunScenario(dir, scenario)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", scenario.Name, err)
		return false
	}
	if scenario.Name == "scroll" {
		scrolled, err := captureScroll(h, dir)
		if err != nil {
			fmt.Fprintf(stderr, "scroll cap: %v\n", err)
			return false
		}
		caps = append(caps, scrolled...)
	}
	frame := h.Frame()
	status := "ok"
	if !frame.FillsTerminal() {
		status = "FILL_FAIL " + strings.Join(frame.FillErrors(), "; ")
	}
	fmt.Fprintf(stdout, "scenario=%s status=%s caps=%d out=%s\n", scenario.Name, status, len(caps), dir)
	for _, path := range caps {
		fmt.Fprintf(stdout, "  %s\n", path)
	}
	return frame.FillsTerminal()
}

// captureScroll seeds a long transcript, then captures it at the bottom and
// again after scrolling up with the wheel.
func captureScroll(h *tui.Harness, dir string) ([]string, error) {
	h.SeedTranscript(35, "scroll event")
	bottom, err := h.Screencap(dir, "scroll-01-bottom")
	if err != nil {
		return nil, err
	}
	for range 8 {
		h.WheelUp()
	}
	up, err := h.Screencap(dir, "scroll-02-up")
	if err != nil {
		return nil, err
	}
	return []string{bottom, up}, nil
}

// runJourneys runs the core DM journeys at every journey size, prints one
// line each, and captures every final frame. It reports whether all passed.
func runJourneys(dir string, stdout, stderr io.Writer) bool {
	ok := true
	passed := 0
	results := tui.RunCoreJourneys()
	for _, result := range results {
		fmt.Fprintf(stdout, "journey %s\n", result.Summary())
		stem := fmt.Sprintf("%s-%dx%d", result.Journey, result.Width, result.Height)
		if _, err := result.Final.Screencap(dir, stem); err != nil {
			fmt.Fprintf(stderr, "journey cap %s: %v\n", stem, err)
			ok = false
		}
		if result.OK() {
			passed++
		} else {
			ok = false
		}
	}
	fmt.Fprintf(stdout, "scenario=journeys status=%s passed=%d/%d out=%s\n", map[bool]string{true: "ok", false: "FAIL"}[ok], passed, len(results), dir)
	return ok
}
