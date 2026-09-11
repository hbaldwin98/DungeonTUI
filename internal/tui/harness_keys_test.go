package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The model matches keys by Key.String(), so every harness key name must
// produce exactly the string a real terminal's key would.
func TestParseKeyMatchesTerminalKeyNames(t *testing.T) {
	cases := map[string]string{
		"enter": "enter", "Return": "enter", "ctrl+enter": "ctrl+enter", "shift+enter": "shift+enter",
		"esc": "esc", "escape": "esc", "tab": "tab", "shift+tab": "shift+tab", "backspace": "backspace",
		"up": "up", "down": "down", "left": "left", "right": "right",
		"alt+left": "alt+left", "alt+right": "alt+right", "ctrl+up": "ctrl+up", "ctrl+down": "ctrl+down",
		"pgup": "pgup", "pagedown": "pgdown", "home": "home", "end": "end",
		"ctrl+n": "ctrl+n", "ctrl+e": "ctrl+e", "ctrl+s": "ctrl+s", "ctrl+c": "ctrl+c",
		"j": "j", "/": "/", "?": "?",
	}
	for name, want := range cases {
		if got := parseKey(name).String(); got != want {
			t.Errorf("parseKey(%q).String() = %q, want %q", name, got, want)
		}
	}
	if key := parseKey("ctrl+n"); key.Text != "" {
		t.Fatal("a control chord must carry no text")
	}
}

// A capital arrives from the decoder as its lower-case code with Shift, and
// named keys stay case-insensitive.
func TestParseKeyPreservesCaseLikeATerminal(t *testing.T) {
	g := parseKey("G")
	if g.String() != "G" || g.Code != 'g' || g.ShiftedCode != 'G' || g.Mod != tea.ModShift {
		t.Fatalf("parseKey(G) = %+v", g)
	}
	if parseKey("g").String() != "g" {
		t.Fatal("a lower-case rune stays lower-case")
	}
	if parseKey("ENTER").String() != "enter" || parseKey("Ctrl+N").String() != "ctrl+n" {
		t.Fatal("named keys and chords match in any case")
	}
	keys := parseKeys("ZZ")
	if len(keys) != 2 || keys[0].String() != "Z" || keys[1].String() != "Z" {
		t.Fatalf("parseKeys(ZZ) = %v", keys)
	}
}

func TestHarnessPressesCapitalsAndSequences(t *testing.T) {
	h := NewHarness(100, 36)
	h.Model.layout.VimEditing = true
	h.Key("n")
	h.Key("G")
	h.Key("o")
	h.Type("Harness Line")
	h.Key("esc")
	h.Key("ZZ")
	if h.Model.editing {
		t.Fatal("ZZ should save and close the editor")
	}
	for _, record := range h.Model.workspace.Records {
		if record.Title == "New NPC" && strings.Contains(record.Summary+record.Body, "Harness Line") {
			return
		}
	}
	t.Fatalf("G then o should append the typed line to the saved record; status %q", h.Model.status)
}
