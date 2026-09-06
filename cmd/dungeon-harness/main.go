package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/tui"
)

func main() {
	outDir := flag.String("out", "testdata/harness", "directory for screencaps")
	width := flag.Int("width", 100, "terminal width")
	height := flag.Int("height", 30, "terminal height")
	scenario := flag.String("scenario", "smoke", "built-in scenario: smoke|scroll|resize|search|all")
	flag.Parse()

	scenarios := map[string]tui.Scenario{
		"smoke": {
			Name: "smoke", Width: *width, Height: *height,
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
			Name: "scroll", Width: *width, Height: *height,
			Steps: []tui.ScenarioStep{
				{Op: "key", Text: "s"},
			},
		},
		"resize": {
			Name: "resize", Width: *width, Height: *height,
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
			Name: "search", Width: *width, Height: *height,
			Steps: []tui.ScenarioStep{
				{Op: "key", Text: "/"},
				{Op: "type", Text: "Vale"},
				{Op: "screencap", Stem: "search-01-results"},
				{Op: "key", Text: "ctrl+a"},
				{Op: "screencap", Stem: "search-02-with-ai"},
			},
		},
	}

	names := []string{*scenario}
	if *scenario == "all" {
		names = []string{"smoke", "resize", "search", "scroll"}
	}

	for _, name := range names {
		sc, ok := scenarios[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown scenario %q\n", name)
			os.Exit(2)
		}
		dir := filepath.Join(*outDir, name)
		h, caps, err := tui.RunScenario(dir, sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
		if name == "scroll" {
			h.SeedTranscript(35, "scroll event")
			path, err := h.Screencap(dir, "scroll-01-bottom")
			if err != nil {
				fmt.Fprintf(os.Stderr, "scroll cap: %v\n", err)
				os.Exit(1)
			}
			caps = append(caps, path)
			for index := 0; index < 8; index++ {
				h.WheelUp()
			}
			path, err = h.Screencap(dir, "scroll-02-up")
			if err != nil {
				fmt.Fprintf(os.Stderr, "scroll cap: %v\n", err)
				os.Exit(1)
			}
			caps = append(caps, path)
		}
		frame := h.Frame()
		status := "ok"
		if !frame.FillsTerminal() {
			status = "FILL_FAIL " + strings.Join(frame.FillErrors(), "; ")
		}
		fmt.Printf("scenario=%s status=%s caps=%d out=%s\n", name, status, len(caps), dir)
		for _, cap := range caps {
			fmt.Printf("  %s\n", cap)
		}
		if !frame.FillsTerminal() {
			os.Exit(1)
		}
	}
}
