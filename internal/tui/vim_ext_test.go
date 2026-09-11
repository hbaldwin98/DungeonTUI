package tui

import (
	"strings"
	"testing"
)

// vimType stands in for the textarea during an Insert session: it types
// text at the cursor and presses Esc.
func vimType(b vimBuffer, state *vimState, text string) vimBuffer {
	end := b.insertText(b.pos(), text)
	b.row, b.col = end.row, end.col
	return state.leaveInsert(b)
}

func TestVimTextObjectsEditTheBuffer(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		row, col int
		keys     []string
		want     string
	}{
		{"diw deletes the word only", "alpha beta gamma", 0, 7, []string{"d", "i", "w"}, "alpha  gamma"},
		{"daw takes the trailing blank", "alpha beta gamma", 0, 7, []string{"d", "a", "w"}, "alpha gamma"},
		{"daw on the last word takes the leading blank", "alpha beta", 0, 7, []string{"d", "a", "w"}, "alpha"},
		{"diw on punctuation", "a.,b", 0, 1, []string{"d", "i", "w"}, "ab"},
		{"dip deletes the paragraph", "a\nb\n\nc", 0, 0, []string{"d", "i", "p"}, "\nc"},
		{"dap takes the blank lines after", "a\nb\n\nc", 1, 0, []string{"d", "a", "p"}, "c"},
		{"dap on the last paragraph takes the blanks before", "a\n\nc", 2, 0, []string{"d", "a", "p"}, "a"},
		{"diw on an empty line does nothing", "a\n\nb", 1, 0, []string{"d", "i", "w"}, "a\n\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, _, _ := vimRun(tc.text, tc.row, tc.col, tc.keys...)
			if buffer.text() != tc.want {
				t.Fatalf("keys %v: got %q, want %q", tc.keys, buffer.text(), tc.want)
			}
		})
	}
	_, state, _ := vimRun("alpha beta gamma", 0, 7, "y", "i", "w")
	if state.register != "beta" || state.linewise {
		t.Fatalf("yiw register %q linewise=%v", state.register, state.linewise)
	}
	buffer, state, _ := vimRun("alpha beta", 0, 7, "c", "i", "w")
	if state.mode != vimInsert || buffer.text() != "alpha " {
		t.Fatalf("ciw should delete and enter Insert: %q mode %s", buffer.text(), state.mode.label())
	}
}

func TestVimVisualModeOperatesOnTheSelection(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		row, col int
		keys     []string
		want     string
	}{
		{"v e d deletes through the word end", "alpha beta", 0, 0, []string{"v", "e", "d"}, " beta"},
		{"v selects backwards too", "alpha beta", 0, 4, []string{"v", "0", "x"}, " beta"},
		{"o swaps the ends", "abcdef", 0, 1, []string{"v", "l", "o", "h", "d"}, "def"},
		{"V j d deletes both lines", "a\nb\nc", 0, 0, []string{"V", "j", "d"}, "c"},
		{"D in charwise Visual deletes whole lines", "ab\ncd\nef", 0, 1, []string{"v", "j", "D"}, "ef"},
		{"~ flips case", "abc", 0, 0, []string{"v", "l", "~"}, "ABc"},
		{"V J joins the lines", "a\nb\nc", 0, 0, []string{"V", "j", "J"}, "a b\nc"},
		{"v i w d selects the word", "alpha beta gamma", 0, 7, []string{"v", "i", "w", "d"}, "alpha  gamma"},
		{"v i p makes a line selection", "a\nb\n\nc", 0, 0, []string{"v", "i", "p", "d"}, "\nc"},
		{"Esc leaves Visual without a change", "abc", 0, 0, []string{"v", "l", "esc", "x"}, "ac"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, state, _ := vimRun(tc.text, tc.row, tc.col, tc.keys...)
			if buffer.text() != tc.want {
				t.Fatalf("keys %v: got %q, want %q", tc.keys, buffer.text(), tc.want)
			}
			if state.visual() {
				t.Fatal("the operator should return to Normal mode")
			}
		})
	}
	_, state, _ := vimRun("a\nb", 0, 0, "V", "y")
	if state.register != "a" || !state.linewise {
		t.Fatalf("V y register %q linewise=%v", state.register, state.linewise)
	}
	_, state, _ = vimRun("abc", 0, 1, "v")
	if got := state.status(); got != "VISUAL from 1:2" {
		t.Fatalf("status %q", got)
	}
	_, state, _ = vimRun("abc", 0, 0, "v", "V")
	if state.mode != vimVisualLine {
		t.Fatal("V in Visual should switch to line selection")
	}
	_, state, _ = vimRun("abc", 0, 0, "v", "l", "c")
	if state.mode != vimInsert {
		t.Fatal("c in Visual should enter Insert")
	}
}

func TestVimSearchFindsWrapsAndDrivesOperators(t *testing.T) {
	text := "alpha beta\nBeta gamma"
	search := []string{"/", "b", "e", "t", "a", "enter"}
	buffer, state, _ := vimRun(text, 0, 0, search...)
	if buffer.pos() != (vimPos{0, 6}) || state.commandL {
		t.Fatalf("/beta landed at %v", buffer.pos())
	}
	steps := []struct {
		key  string
		want vimPos
	}{{"n", vimPos{1, 0}}, {"n", vimPos{0, 6}}, {"N", vimPos{1, 0}}}
	for _, step := range steps {
		buffer, _ = state.apply(buffer, step.key)
		if buffer.pos() != step.want {
			t.Fatalf("%s landed at %v, want %v (lower-case matches Beta, and search wraps)", step.key, buffer.pos(), step.want)
		}
	}
	buffer, state, _ = vimRun(text, 0, 0, "/", "B", "e", "t", "a", "enter")
	if buffer.pos() != (vimPos{1, 0}) {
		t.Fatalf("a capital makes the search exact: %v", buffer.pos())
	}
	buffer, state, _ = vimRun(text, 0, 3, "/", "z", "z", "enter")
	if buffer.pos() != (vimPos{0, 3}) || !strings.Contains(state.notice, "Pattern not found") {
		t.Fatalf("a miss keeps the cursor and says so: %v %q", buffer.pos(), state.notice)
	}
	_, state, _ = vimRun(text, 0, 0, "n")
	if !strings.Contains(state.notice, "No previous search") {
		t.Fatalf("n with no search: %q", state.notice)
	}
	buffer, _, _ = vimRun("alpha beta", 0, 0, append(search, "0", "d", "n")...)
	if buffer.text() != "beta" {
		t.Fatalf("dn deletes up to the match: %q", buffer.text())
	}
	_, state, _ = vimRun(text, 0, 0, "/", "b")
	if state.status() != "/b" {
		t.Fatalf("the search line shows its prefix: %q", state.status())
	}
}

func TestVimDotRepeatsTheLastChange(t *testing.T) {
	cases := []struct {
		name string
		text string
		keys []string
		want string
	}{
		{"x .", "abcd", []string{"x", "."}, "cd"},
		{"dw .", "a b c d", []string{"d", "w", "."}, "c d"},
		{"dd 2.", "a\nb\nc\nd", []string{"d", "d", "2", "."}, "d"},
		{"dot keeps the typed count", "abcdefgh", []string{"3", "x", "."}, "gh"},
		{"motions between do not become the change", "ab cd", []string{"x", "w", "."}, "b d"},
		{"undo is not a change", "abc", []string{"x", "u", "l", "."}, "ac"},
		{"Visual changes repeat over the same motions", "abcdef", []string{"v", "l", "d", "."}, "ef"},
		{"yank is not a change", "abc", []string{"x", "y", "l", "."}, "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, _, _ := vimRun(tc.text, 0, 0, tc.keys...)
			if buffer.text() != tc.want {
				t.Fatalf("keys %v: got %q, want %q", tc.keys, buffer.text(), tc.want)
			}
		})
	}

	state := newVimState("one two")
	buffer := newVimBuffer("one two", 0, 0)
	for _, key := range []string{"c", "i", "w"} {
		buffer, _ = state.apply(buffer, key)
	}
	buffer = vimType(buffer, &state, "six")
	for _, key := range []string{"w", "."} {
		buffer, _ = state.apply(buffer, key)
	}
	if buffer.text() != "six six" || state.mode != vimNormal {
		t.Fatalf("ciw then . should retype the change: %q", buffer.text())
	}
	buffer, _ = state.apply(buffer, "u")
	if buffer.text() != "six two" {
		t.Fatalf("u should undo only the repeat: %q", buffer.text())
	}

	state = newVimState("a\nb")
	buffer = newVimBuffer("a\nb", 0, 0)
	buffer, _ = state.apply(buffer, "A")
	buffer = vimType(buffer, &state, "!")
	for _, key := range []string{"j", "."} {
		buffer, _ = state.apply(buffer, key)
	}
	if buffer.text() != "a!\nb!" {
		t.Fatalf("A then . should append on the next line: %q", buffer.text())
	}
}

func TestVimNamedRegisters(t *testing.T) {
	buffer, _, _ := vimRun("a\nb", 0, 0, `"`, "a", "y", "y", "j", "y", "y", `"`, "a", "p")
	if buffer.text() != "a\nb\na" {
		t.Fatalf(`"ap should put register a: %q`, buffer.text())
	}
	buffer, _, _ = vimRun("a\nb", 0, 0, `"`, "a", "y", "y", "j", "p")
	if buffer.text() != "a\nb\na" {
		t.Fatalf("a named yank also fills the unnamed register: %q", buffer.text())
	}
	buffer, state, _ := vimRun("a\nb", 0, 0, `"`, "a", "y", "y", "j", `"`, "A", "y", "y", "G", `"`, "a", "p")
	if buffer.text() != "a\nb\na\nb" {
		t.Fatalf(`"A should append: %q`, buffer.text())
	}
	if state.named['a'].text != "a\nb" {
		t.Fatalf("register a holds %q", state.named['a'].text)
	}
	buffer, _, _ = vimRun("a b", 0, 0, `"`, "q", "d", "w", "$", `"`, "z", "p")
	if buffer.text() != "b" {
		t.Fatalf("an empty register puts nothing: %q", buffer.text())
	}
	_, state, _ = vimRun("abc", 0, 0, `"`, "a")
	if state.status() != `NORMAL "a` {
		t.Fatalf("a chosen register shows in the status: %q", state.status())
	}
	_, state, _ = vimRun("abc", 0, 0, `"`, "a", "esc")
	if state.regSel != 0 {
		t.Fatal("Esc drops a chosen register")
	}
	buffer, _, _ = vimRun("abc", 0, 0, "r", "5")
	if buffer.text() != "5bc" {
		t.Fatalf("r takes a digit: %q", buffer.text())
	}
}

func TestVimEditorRepeatsSearchesAndKeepsRegistersAcrossEdits(t *testing.T) {
	model := vimPress(t, vimModel(t), "n", "0", "w", "c", "i", "w", "O", "l", "d", "esc", "w", ".")
	if !strings.Contains(model.editBody.Value(), "# Old Old") {
		t.Fatalf("ciw then . in the editor: %q", model.editBody.Value())
	}
	model = vimPress(t, model, "v")
	if !strings.Contains(model.View().Content, "VISUAL from") {
		t.Fatal("the footer should show the Visual selection's start")
	}
	model = vimPress(t, model, "esc", "/", "z", "q", "x", "enter")
	if !strings.Contains(model.status, "Pattern not found") {
		t.Fatalf("a failed search should reach the status line: %q", model.status)
	}
	model = vimPress(t, model, `"`, "a", "y", "y", ":", "q", "!", "enter")
	if model.editing {
		t.Fatal(":q! should close the editor")
	}
	model = vimPress(t, model, "n", `"`, "a", "p")
	if strings.Count(model.editBody.Value(), "# Old Old") != 1 || !strings.Contains(model.editBody.Value(), "# New NPC") {
		t.Fatalf("register a should survive reopening the editor: %q", model.editBody.Value())
	}
}
