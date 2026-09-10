package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runHarness(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestHarnessRefusesBadCommandLinesBeforeRunning(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := runHarness(t, "-out", out, "-scenario", "smok")
	if code != exitUsage || !strings.Contains(stderr, `unknown scenario "smok"`) || stdout != "" {
		t.Fatalf("unknown scenario: code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Fatalf("a refused run wrote %d entries", len(entries))
	}
	code, _, stderr = runHarness(t, "-width", "wide")
	if code != exitUsage || !strings.Contains(stderr, "invalid value") {
		t.Fatalf("bad flag: code %d stderr %q", code, stderr)
	}
}

func TestHarnessRunsOneScenarioAndWritesItsScreencaps(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := runHarness(t, "-out", out, "-scenario", "smoke", "-width", "80", "-height", "24")
	if code != exitOK || stderr != "" {
		t.Fatalf("smoke: code %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "scenario=smoke status=ok caps=3") {
		t.Fatalf("smoke output %q", stdout)
	}
	for _, stem := range []string{"smoke-01-browser", "smoke-02-session", "smoke-03-ended"} {
		if _, err := os.Stat(filepath.Join(out, "smoke", stem+".screen.txt")); err != nil {
			t.Fatalf("missing screencap %s: %v", stem, err)
		}
	}
}

func TestHarnessScrollCapturesBottomAndScrolledUp(t *testing.T) {
	out := t.TempDir()
	code, stdout, _ := runHarness(t, "-out", out, "-scenario", "scroll")
	if code != exitOK || !strings.Contains(stdout, "scenario=scroll status=ok caps=2") {
		t.Fatalf("scroll: code %d output %q", code, stdout)
	}
	bottom, _ := os.ReadFile(filepath.Join(out, "scroll", "scroll-01-bottom.screen.txt"))
	up, _ := os.ReadFile(filepath.Join(out, "scroll", "scroll-02-up.screen.txt"))
	if len(bottom) == 0 || bytes.Equal(bottom, up) {
		t.Fatal("scrolling up should change the captured transcript")
	}
}

func TestHarnessAllRunsJourneysThenEveryScenario(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := runHarness(t, "-out", out, "-scenario", "all")
	if code != exitOK || stderr != "" {
		t.Fatalf("all: code %d stderr %q", code, stderr)
	}
	order := []string{"scenario=journeys status=ok", "scenario=smoke", "scenario=resize", "scenario=search", "scenario=scroll"}
	last := -1
	for _, marker := range order {
		index := strings.Index(stdout, marker)
		if index <= last {
			t.Fatalf("%q missing or out of order in %q", marker, stdout)
		}
		last = index
	}
}

func TestHarnessReportsFailureWhenCapturesCannotBeWritten(t *testing.T) {
	// A file where the output directory should be makes every capture fail.
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"smoke", "journeys"} {
		code, _, stderr := runHarness(t, "-out", blocked, "-scenario", scenario)
		if code != exitFail || stderr == "" {
			t.Fatalf("%s into a file: code %d stderr %q", scenario, code, stderr)
		}
	}
}
