package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hbaldwin98/dungeon/internal/dice"
	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

func TestHarnessBrowserFillsCommonSizes(t *testing.T) {
	sizes := [][2]int{{80, 24}, {100, 30}, {120, 40}, {60, 20}}
	for _, size := range sizes {
		h := NewHarness(size[0], size[1])
		frame := h.Frame()
		if errs := frame.FillErrors(); len(errs) > 0 {
			t.Fatalf("browser %dx%d fill failed: %s\n%s", size[0], size[1], strings.Join(errs, "; "), frame.Plain)
		}
	}
}

func TestHarnessSessionScrollResizeAndScreencap(t *testing.T) {
	dir := t.TempDir()
	h := NewHarness(100, 30)
	h.Key("s")
	for index := 0; index < 40; index++ {
		h.Model.session.Entries = append(h.Model.session.Entries, domain.TranscriptEntry{
			Text:      strings.Repeat("event ", 8) + strings.Repeat("x", index%7),
			CreatedAt: time.Unix(int64(index), 0),
		})
	}
	h.Model.refreshTranscriptViewport()
	h.Model.configureTranscriptViewport()

	before := h.Frame()
	if !before.Contains("SESSION TRANSCRIPT") {
		t.Fatalf("session frame missing transcript:\n%s", before.Plain)
	}
	if path, err := h.Screencap(dir, "session-bottom"); err != nil {
		t.Fatal(err)
	} else if path == "" {
		t.Fatal("expected screencap path")
	}

	if h.Model.transcriptView.AtBottom() == false {
		t.Fatal("expected transcript to start at bottom")
	}
	h.WheelUp()
	h.WheelUp()
	h.WheelUp()
	if h.Model.transcriptView.AtBottom() {
		t.Fatal("wheel up should leave the newest lines")
	}
	if _, err := h.Screencap(dir, "session-scrolled"); err != nil {
		t.Fatal(err)
	}

	divider := h.Model.splitWidth(h.Model.width)
	h.DragLeft(divider, 4, 45, 4)
	if h.Model.paneSplit != 45 {
		t.Fatalf("vertical drag should set paneSplit=45, got %d", h.Model.paneSplit)
	}
	upper := h.Model.sessionUpperHeight()
	beforeTranscript := h.Model.sessionTranscriptHeight()
	h.DragLeft(70, upper+1, 70, upper+5)
	if h.Model.sessionTranscriptHeight() >= beforeTranscript {
		t.Fatalf("horizontal drag should shrink transcript: before=%d after=%d", beforeTranscript, h.Model.sessionTranscriptHeight())
	}
	frame := h.Frame()
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("resized session fill failed: %s", strings.Join(errs, "; "))
	}
	if _, err := h.Screencap(dir, "session-resized"); err != nil {
		t.Fatal(err)
	}

	entries, err := filepath.Glob(filepath.Join(dir, "*.screen.txt"))
	if err != nil || len(entries) < 3 {
		t.Fatalf("expected screencaps, got %v err=%v", entries, err)
	}
}

func TestHarnessScenarioScript(t *testing.T) {
	dir := t.TempDir()
	_, caps, err := RunScenario(dir, Scenario{
		Name:   "search-and-session",
		Width:  100,
		Height: 30,
		Steps: []ScenarioStep{
			{Op: "screencap", Stem: "01-browser"},
			{Op: "key", Text: "/"},
			{Op: "type", Text: "Vale"},
			{Op: "screencap", Stem: "02-search"},
			{Op: "key", Text: "esc"},
			{Op: "key", Text: "s"},
			{Op: "type", Text: "@Cap"},
			{Op: "screencap", Stem: "03-suggestions"},
			{Op: "key", Text: "tab"},
			{Op: "key", Text: "enter"},
			{Op: "screencap", Stem: "04-captured"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(caps) != 4 {
		t.Fatalf("expected 4 caps, got %d", len(caps))
	}
}

func TestEndedSessionIsRetainedInWorkspace(t *testing.T) {
	h := NewHarness(100, 30)
	h.Key("s")
	h.Model.sessionInput.SetValue("party arrives")
	h.Key("enter")
	sessionID := h.Model.session.ID
	h.Key("ctrl+e")
	if h.Model.session != nil {
		t.Fatal("session should end")
	}
	found := false
	for _, session := range h.Model.workspace.Sessions {
		if session.ID == sessionID {
			found = true
			if session.EndedAt == nil {
				t.Fatal("ended session must record EndedAt")
			}
			if len(session.Entries) != 1 {
				t.Fatalf("expected retained transcript, got %#v", session.Entries)
			}
		}
	}
	if !found {
		t.Fatal("ended session was discarded from the workspace")
	}
}

func TestTranscriptViewportUsesTranscriptHeight(t *testing.T) {
	h := NewHarness(100, 36)
	h.Key("s")
	for index := 0; index < 30; index++ {
		h.Model.session.Entries = append(h.Model.session.Entries, domain.TranscriptEntry{
			Text:      fmt.Sprintf("unique-line-%02d", index),
			CreatedAt: time.Unix(int64(index), 0),
		})
	}
	h.Model.configureTranscriptViewport()
	h.Model.refreshTranscriptViewport()
	transcriptHeight := h.Model.sessionTranscriptHeight()
	want := max(1, panelInnerHeight(transcriptHeight)-1)
	if h.Model.transcriptView.Height() != want {
		t.Fatalf("viewport height=%d, want transcript-based %d (input height is %d)",
			h.Model.transcriptView.Height(), want, h.Model.sessionInputHeight())
	}
	frame := h.Frame()
	// Newest entries should be visible at the bottom; older ones may scroll away.
	if !frame.Contains("unique-line-29") {
		t.Fatalf("expected newest transcript lines in the rendered frame:\n%s", frame.Plain)
	}
}

func TestSessionPanelsKeepBottomBorders(t *testing.T) {
	h := NewHarness(100, 30)
	h.Key("s")
	h.SetSessionDraft("The party enters the crypt")
	h.Key("enter")
	frame := h.Frame()
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("fill failed: %s", strings.Join(errs, "; "))
	}
	// Clipped MaxHeight used to eat lower borders, gluing panes together.
	bottomBorders := strings.Count(frame.Plain, "╰")
	if bottomBorders < 3 {
		t.Fatalf("expected closed borders for upper/transcript/input panes, found %d:\n%s", bottomBorders, frame.Plain)
	}
}

func TestSessionDiceExpressionsAreRecorded(t *testing.T) {
	h := NewHarness(100, 40)
	h.Key("s")
	h.Model.rollRNG = dice.FixedRNG(17, 4, 5)
	h.SetSessionDraft("The key opens the seal. #d20+5 #damage 2d6+3")
	h.Key("enter")
	if len(h.Model.session.Entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(h.Model.session.Entries))
	}
	rolls := h.Model.session.Entries[0].Rolls
	if len(rolls) != 2 {
		t.Fatalf("expected two rolls, got %#v", rolls)
	}
	if rolls[0].Expression != "d20+5" || rolls[0].Total != 22 {
		t.Fatalf("first roll %#v", rolls[0])
	}
	if rolls[1].Label != "damage" || rolls[1].Total != 12 {
		t.Fatalf("second roll %#v", rolls[1])
	}
	frame := h.Frame()
	if !frame.Contains("roll damage 2d6+3 → [4+5]+3 = 12") && !frame.Contains("roll d20+5 → [17]+5 = 22") {
		t.Fatalf("transcript missing roll detail:\n%s", frame.Plain)
	}
	if !strings.Contains(h.Model.status, "d20+5") || !strings.Contains(h.Model.status, "damage") {
		t.Fatalf("status should summarize rolls: %q", h.Model.status)
	}
}

func TestLayoutPreferencesPersistSeparately(t *testing.T) {
	dir := t.TempDir()
	prefPath := filepath.Join(dir, "preferences.json")
	store := prefs.NewJSON(prefPath)

	h := NewHarness(100, 30)
	h.Model.prefs = store
	divider := h.Model.splitWidth(h.Model.width)
	h.DragLeft(divider, 10, 47, 10)
	h.Key("s")
	h.Key("ctrl+p")

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PaneSplit != 47 {
		t.Fatalf("pane_split=%d want 47", loaded.PaneSplit)
	}
	if loaded.PaneLayout != 1 {
		t.Fatalf("pane_layout=%d want 1", loaded.PaneLayout)
	}

	restored := NewHarness(100, 30)
	restored.Model.prefs = store
	restored.Model.applyPreferences()
	if restored.Model.paneSplit != 47 || restored.Model.paneLayout != 1 {
		t.Fatalf("restored layout split=%d layout=%d", restored.Model.paneSplit, restored.Model.paneLayout)
	}
	if _, err := os.ReadFile(filepath.Join(dir, "workspace.json")); !os.IsNotExist(err) {
		t.Fatal("layout prefs must not write campaign workspace.json")
	}
}

func TestNarrowTerminalStillFillsExactWidth(t *testing.T) {
	h := NewHarness(50, 20)
	frame := h.Frame()
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("narrow browser should fill exact terminal size: %s", strings.Join(errs, "; "))
	}
	h.Key("s")
	frame = h.Frame()
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("narrow session should fill exact terminal size: %s", strings.Join(errs, "; "))
	}
}
