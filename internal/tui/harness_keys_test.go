package tui

import "testing"

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
