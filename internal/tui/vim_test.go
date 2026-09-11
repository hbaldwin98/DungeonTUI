package tui

import (
	"strings"
	"testing"
)

// vimRun applies Normal-mode keys to text with the cursor at row, col.
func vimRun(text string, row, col int, keys ...string) (vimBuffer, *vimState, vimAction) {
	state := newVimState(text)
	buffer := newVimBuffer(text, row, col)
	action := vimNone
	for _, key := range keys {
		buffer, action = state.apply(buffer, key)
	}
	return buffer, &state, action
}

func TestVimMotionsLandWhereVimDoes(t *testing.T) {
	text := "alpha beta.gamma\n\n  indented line\nlast"
	cases := []struct {
		name     string
		row, col int
		keys     []string
		want     vimPos
	}{
		{"w to next word", 0, 0, []string{"w"}, vimPos{0, 6}},
		{"w stops at punctuation", 0, 6, []string{"w"}, vimPos{0, 10}},
		{"w stops on an empty line", 0, 11, []string{"w"}, vimPos{1, 0}},
		{"counted w", 0, 0, []string{"3", "w"}, vimPos{0, 11}},
		{"e to word end", 0, 0, []string{"e"}, vimPos{0, 4}},
		{"b back across lines", 2, 2, []string{"b"}, vimPos{1, 0}},
		{"b to word start", 0, 8, []string{"b"}, vimPos{0, 6}},
		{"$ to last character", 0, 0, []string{"$"}, vimPos{0, 15}},
		{"0 to column zero", 2, 8, []string{"0"}, vimPos{2, 0}},
		{"^ to first non-blank", 2, 8, []string{"^"}, vimPos{2, 2}},
		{"G to last line", 0, 3, []string{"G"}, vimPos{3, 0}},
		{"counted G to a line", 0, 0, []string{"3", "G"}, vimPos{2, 2}},
		{"gg to first line", 3, 2, []string{"g", "g"}, vimPos{0, 0}},
		{"j keeps the column, clamped", 0, 12, []string{"j", "j"}, vimPos{2, 12}},
		{"l stops on the last character", 3, 2, []string{"9", "l"}, vimPos{3, 3}},
		{"} to the next blank line", 0, 4, []string{"}"}, vimPos{1, 0}},
		{"{ back to the blank line", 3, 1, []string{"{"}, vimPos{1, 0}},
		{"Esc drops a pending count", 0, 0, []string{"3", "esc", "w"}, vimPos{0, 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, _, _ := vimRun(text, tc.row, tc.col, tc.keys...)
			if got := buffer.pos(); got != tc.want {
				t.Fatalf("keys %v: cursor %v, want %v", tc.keys, got, tc.want)
			}
			if buffer.text() != text {
				t.Fatalf("a motion changed the text: %q", buffer.text())
			}
		})
	}
}

func TestVimOperatorsEditTheBuffer(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		row, col int
		keys     []string
		want     string
		cursor   vimPos
		insert   bool
	}{
		{"dw deletes a word and its space", "alpha beta", 0, 0, []string{"d", "w"}, "beta", vimPos{0, 0}, false},
		{"dw at line end never joins lines", "one two\nthree", 0, 4, []string{"d", "w"}, "one \nthree", vimPos{0, 3}, false},
		{"counted d2w", "a b c d", 0, 0, []string{"d", "2", "w"}, "c d", vimPos{0, 0}, false},
		{"de is inclusive", "alpha beta", 0, 0, []string{"d", "e"}, " beta", vimPos{0, 0}, false},
		{"db deletes back to word start", "alpha beta", 0, 6, []string{"d", "b"}, "beta", vimPos{0, 0}, false},
		{"d$ to line end", "alpha beta", 0, 5, []string{"d", "$"}, "alpha", vimPos{0, 4}, false},
		{"D is d$", "alpha beta", 0, 5, []string{"D"}, "alpha", vimPos{0, 4}, false},
		{"d0 to line start", "alpha beta", 0, 6, []string{"d", "0"}, "beta", vimPos{0, 0}, false},
		{"dd deletes the line", "a\nb\nc", 1, 0, []string{"d", "d"}, "a\nc", vimPos{1, 0}, false},
		{"2dd deletes two lines", "a\nb\nc", 0, 0, []string{"2", "d", "d"}, "c", vimPos{0, 0}, false},
		{"dd on the last line keeps an empty buffer", "only", 0, 2, []string{"d", "d"}, "", vimPos{0, 0}, false},
		{"dj deletes two lines", "a\nb\nc", 0, 0, []string{"d", "j"}, "c", vimPos{0, 0}, false},
		{"dG deletes to the end", "a\nb\nc", 1, 0, []string{"d", "G"}, "a", vimPos{0, 0}, false},
		{"dgg deletes to the top", "a\nb\nc", 1, 0, []string{"d", "g", "g"}, "c", vimPos{0, 0}, false},
		{"x deletes a character", "abc", 0, 1, []string{"x"}, "ac", vimPos{0, 1}, false},
		{"3x deletes three", "abcdef", 0, 1, []string{"3", "x"}, "aef", vimPos{0, 1}, false},
		{"x at line end steps back", "abc", 0, 2, []string{"x"}, "ab", vimPos{0, 1}, false},
		{"X deletes before the cursor", "abc", 0, 2, []string{"X"}, "ac", vimPos{0, 1}, false},
		{"cw changes to word end", "alpha beta", 0, 0, []string{"c", "w"}, " beta", vimPos{0, 0}, true},
		{"cc keeps indentation", "a\n  old text\nc", 1, 5, []string{"c", "c"}, "a\n  \nc", vimPos{1, 2}, true},
		{"C changes to line end", "alpha beta", 0, 6, []string{"C"}, "alpha ", vimPos{0, 6}, true},
		{"s substitutes a character", "abc", 0, 1, []string{"s"}, "ac", vimPos{0, 1}, true},
		{"S substitutes the line", "abc\nd", 0, 1, []string{"S"}, "\nd", vimPos{0, 0}, true},
		{"J joins with one space", "a  \n   b\nc", 0, 0, []string{"J"}, "a b\nc", vimPos{0, 1}, false},
		{"3J joins three lines", "a\nb\nc", 0, 0, []string{"3", "J"}, "a b c", vimPos{0, 3}, false},
		{"~ toggles case and advances", "abC", 0, 0, []string{"3", "~"}, "ABc", vimPos{0, 2}, false},
		{"r replaces a character", "abc", 0, 1, []string{"r", "X"}, "aXc", vimPos{0, 1}, false},
		{"2r replaces two", "abcd", 0, 1, []string{"2", "r", "-"}, "a--d", vimPos{0, 2}, false},
		{"r past the line end does nothing", "ab", 0, 1, []string{"3", "r", "x"}, "ab", vimPos{0, 1}, false},
		{"o opens a line below", "a\nb", 0, 0, []string{"o"}, "a\n\nb", vimPos{1, 0}, true},
		{"O opens a line above", "a\nb", 1, 0, []string{"O"}, "a\n\nb", vimPos{1, 0}, true},
		{"A appends at line end", "abc", 0, 0, []string{"A"}, "abc", vimPos{0, 3}, true},
		{"a appends after the cursor", "abc", 0, 1, []string{"a"}, "abc", vimPos{0, 2}, true},
		{"I inserts at first non-blank", "  abc", 0, 4, []string{"I"}, "  abc", vimPos{0, 2}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, state, _ := vimRun(tc.text, tc.row, tc.col, tc.keys...)
			if got := buffer.text(); got != tc.want {
				t.Fatalf("keys %v: text %q, want %q", tc.keys, got, tc.want)
			}
			if got := buffer.pos(); got != tc.cursor {
				t.Fatalf("keys %v: cursor %v, want %v", tc.keys, got, tc.cursor)
			}
			if (state.mode == vimInsert) != tc.insert {
				t.Fatalf("keys %v: mode %s", tc.keys, state.mode.label())
			}
		})
	}
}

func TestVimYankAndPut(t *testing.T) {
	cases := []struct {
		name string
		text string
		keys []string
		want string
	}{
		{"yy p duplicates the line below", "a\nb", []string{"y", "y", "p"}, "a\na\nb"},
		{"Y P duplicates the line above", "a\nb", []string{"j", "Y", "P"}, "a\nb\nb"},
		{"dd p moves a line down", "a\nb\nc", []string{"d", "d", "p"}, "b\na\nc"},
		{"2yy G p copies two lines to the end", "a\nb\nc", []string{"2", "y", "y", "G", "p"}, "a\nb\nc\na\nb"},
		{"yw P puts a word before the cursor", "alpha beta", []string{"y", "w", "$", "P"}, "alpha betalpha a"},
		{"x p swaps two characters", "ab", []string{"x", "p"}, "ba"},
		{"3p repeats a put", "a", []string{"y", "l", "3", "p"}, "aaaa"},
		{"put with nothing yanked does nothing", "a", []string{"p"}, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, _, _ := vimRun(tc.text, 0, 0, tc.keys...)
			if got := buffer.text(); got != tc.want {
				t.Fatalf("keys %v: text %q, want %q", tc.keys, got, tc.want)
			}
		})
	}
}

func TestVimUndoAndRedoRestoreTextAndCursor(t *testing.T) {
	buffer, state, _ := vimRun("one two three", 0, 4, "d", "w", "x")
	if buffer.text() != "one hree" {
		t.Fatalf("setup text %q", buffer.text())
	}
	buffer, _ = state.apply(buffer, "u")
	if buffer.text() != "one three" {
		t.Fatalf("first undo %q", buffer.text())
	}
	buffer, _ = state.apply(buffer, "u")
	if buffer.text() != "one two three" || buffer.pos() != (vimPos{0, 4}) {
		t.Fatalf("second undo %q at %v", buffer.text(), buffer.pos())
	}
	buffer, _ = state.apply(buffer, "u")
	if buffer.text() != "one two three" {
		t.Fatalf("undo past the start changed text: %q", buffer.text())
	}
	buffer, _ = state.apply(buffer, "2")
	buffer, _ = state.apply(buffer, "ctrl+r")
	if buffer.text() != "one hree" {
		t.Fatalf("counted redo %q", buffer.text())
	}
	buffer, _ = state.apply(buffer, "u")
	buffer, _ = state.apply(buffer, "x")
	if len(state.redo) != 0 {
		t.Fatal("a new change must drop the redo trail")
	}
	// A motion is never an undo step.
	before := len(state.undo)
	buffer, _ = state.apply(buffer, "w")
	if len(state.undo) != before {
		t.Fatal("a motion pushed an undo step")
	}
}

func TestVimInsertIsOneUndoStepAndEmptyInsertsLeaveNone(t *testing.T) {
	state := newVimState("abc")
	buffer := newVimBuffer("abc", 0, 0)
	buffer, _ = state.apply(buffer, "i")
	buffer = state.leaveInsert(buffer)
	if len(state.undo) != 0 {
		t.Fatalf("an insert that typed nothing left %d undo steps", len(state.undo))
	}
	buffer, _ = state.apply(buffer, "A")
	buffer.lines[0] = []rune("abcdef")
	buffer.col = 6
	buffer = state.leaveInsert(buffer)
	if buffer.pos() != (vimPos{0, 5}) {
		t.Fatalf("leaving Insert must step back onto a character, got %v", buffer.pos())
	}
	buffer, _ = state.apply(buffer, "u")
	if buffer.text() != "abc" {
		t.Fatalf("undo of an insert gave %q", buffer.text())
	}
}

func TestVimExCommandsAndZZ(t *testing.T) {
	cases := []struct {
		keys []string
		want vimAction
	}{
		{[]string{":", "w", "enter"}, vimWrite},
		{[]string{":", "q", "enter"}, vimQuit},
		{[]string{":", "q", "!", "enter"}, vimForceQuit},
		{[]string{":", "w", "q", "enter"}, vimWriteQuit},
		{[]string{":", "x", "enter"}, vimWriteQuit},
		{[]string{"Z", "Z"}, vimWriteQuit},
		{[]string{"Z", "Q"}, vimForceQuit},
		{[]string{":", "w", "esc"}, vimNone},
		{[]string{":", "q", "backspace", "w", "enter"}, vimWrite},
		{[]string{":", "e", "d", "i", "t", "enter"}, vimNone},
	}
	for _, tc := range cases {
		buffer, state, action := vimRun("text", 0, 0, tc.keys...)
		if action != tc.want {
			t.Fatalf("keys %v: action %v, want %v", tc.keys, action, tc.want)
		}
		if state.commandL || buffer.text() != "text" {
			t.Fatalf("keys %v left the command line open or changed text", tc.keys)
		}
	}
	// Letters typed on the command line are never Normal-mode commands.
	buffer, state, _ := vimRun("text", 0, 0, ":", "d", "d")
	if buffer.text() != "text" || state.status() != ":dd" {
		t.Fatalf("command line leaked keys: %q %q", buffer.text(), state.status())
	}
}

func TestVimStatusShowsModeAndPendingKeys(t *testing.T) {
	_, state, _ := vimRun("text", 0, 0, "2", "d")
	if state.status() != "NORMAL 2d" {
		t.Fatalf("status %q", state.status())
	}
	_, state, _ = vimRun("text", 0, 0, "i")
	if state.status() != "INSERT" {
		t.Fatalf("status %q", state.status())
	}
}

func vimModel(t *testing.T) Model {
	t.Helper()
	model := New()
	model.width = 100
	model.height = 36
	model.layout.VimEditing = true
	return model
}

func vimPress(t *testing.T, model Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		updated, _ := model.Update(parseKey(key))
		model = updated.(Model)
	}
	return model
}

func countTitled(model Model, title string) int {
	count := 0
	for _, record := range model.workspace.Records {
		if record.Title == title {
			count++
		}
	}
	return count
}

func TestVimEntityEditorOpensInNormalModeAndEditsModally(t *testing.T) {
	model := vimPress(t, vimModel(t), "n")
	if !model.editing || model.vim.mode != vimNormal {
		t.Fatalf("editor should open in Normal mode, editing=%v mode=%s", model.editing, model.vim.mode.label())
	}
	before := model.editBody.Value()
	// Normal-mode letters are commands, never text.
	model = vimPress(t, model, "j", "k", "l")
	if model.editBody.Value() != before {
		t.Fatalf("Normal mode typed text: %q", model.editBody.Value())
	}
	if !strings.Contains(model.View().Content, "NORMAL") {
		t.Fatal("footer should show NORMAL")
	}
	// The cursor starts on the title heading; replace the title.
	model = vimPress(t, model, "0", "w", "C", "V", "a", "l", "e")
	if model.vim.mode != vimInsert || !strings.Contains(model.View().Content, "INSERT") {
		t.Fatal("C should enter Insert mode and the footer should say so")
	}
	model = vimPress(t, model, "esc")
	if !model.editing || model.vim.mode != vimNormal {
		t.Fatal("Esc in Insert returns to Normal without closing the editor")
	}
	if !strings.Contains(model.editBody.Value(), "# Vale") {
		t.Fatalf("title not replaced: %q", model.editBody.Value())
	}
	model = vimPress(t, model, "esc")
	if !model.editing {
		t.Fatal("Esc in Normal mode must not discard the edit")
	}
	model = vimPress(t, model, "u")
	if model.editBody.Value() != before {
		t.Fatalf("undo should restore the opened text, got %q", model.editBody.Value())
	}
}

func TestVimEntityEditorExCommandsSaveStayAndRefuseToDropWork(t *testing.T) {
	model := vimPress(t, vimModel(t), "n", "0", "w", "C", "V", "a", "l", "e", "esc")
	model = vimPress(t, model, ":", "q", "enter")
	if !model.editing || model.status != vimQuitRefusal {
		t.Fatalf(":q with changes must refuse, editing=%v status=%q", model.editing, model.status)
	}
	model = vimPress(t, model, ":", "w", "enter")
	if !model.editing || model.creating || model.editID == "" {
		t.Fatalf(":w should save and keep editing, editing=%v creating=%v id=%q", model.editing, model.creating, model.editID)
	}
	if countTitled(model, "Vale") != 1 {
		t.Fatalf("saved record missing, got %d", countTitled(model, "Vale"))
	}
	// A second :w updates the same record rather than creating another.
	model = vimPress(t, model, "G", "o", "M", "o", "r", "e", "esc", ":", "w", "enter")
	if countTitled(model, "Vale") != 1 {
		t.Fatalf("second :w duplicated the record: %d", countTitled(model, "Vale"))
	}
	model = vimPress(t, model, ":", "q", "enter")
	if model.editing {
		t.Fatalf(":q after :w should close, status %q", model.status)
	}
	record := model.selectedRecord()
	// The first paragraph under the title is the summary.
	if record == nil || record.Title != "Vale" || record.Summary != "More" {
		t.Fatalf("saved record wrong: %#v", record)
	}
}

func TestVimEntityEditorForceQuitDiscardsAndZZSaves(t *testing.T) {
	model := vimPress(t, vimModel(t), "n", "0", "w", "C", "G", "o", "n", "e", "esc", ":", "q", "!", "enter")
	if model.editing || countTitled(model, "Gone") != 0 {
		t.Fatalf(":q! must close without saving, editing=%v", model.editing)
	}
	model = vimPress(t, model, "n", "0", "w", "C", "K", "e", "p", "t", "esc", "Z", "Z")
	if model.editing || countTitled(model, "Kept") != 1 {
		t.Fatalf("ZZ must save and close, editing=%v status=%q", model.editing, model.status)
	}
}

func TestVimEntityEditorKeepsEditorChordsAndSuggestions(t *testing.T) {
	model := vimPress(t, vimModel(t), "n")
	start := model.editType
	model = vimPress(t, model, "ctrl+t")
	if model.editType == start {
		t.Fatal("Ctrl+T must still cycle the type in Normal mode")
	}
	// Insert mode keeps @ suggestions and Tab completion.
	model = vimPress(t, model, "G", "o", "@", "C", "a", "p")
	if len(model.suggestions) == 0 {
		t.Fatal("@ suggestions should appear in Insert mode")
	}
	model = vimPress(t, model, "tab")
	if !strings.Contains(model.editBody.Value(), "@Captain Vale") {
		t.Fatalf("Tab should complete in Insert mode: %q", model.editBody.Value())
	}
	model = vimPress(t, model, "esc")
	if len(model.suggestions) != 0 {
		t.Fatal("leaving Insert mode should clear suggestions")
	}
	model = vimPress(t, model, "ctrl+s")
	if model.editing {
		t.Fatalf("Ctrl+S must still save and close, status %q", model.status)
	}
}

func TestVimOffLeavesTheEditorUnchanged(t *testing.T) {
	model := New()
	model.width, model.height = 100, 36
	model = vimPress(t, model, "n", "j", "k")
	if !strings.Contains(model.editBody.Value(), "jk") {
		t.Fatalf("without Vim, letters are text: %q", model.editBody.Value())
	}
	if strings.Contains(model.View().Content, "NORMAL") {
		t.Fatal("no mode label without Vim")
	}
	model = vimPress(t, model, "esc")
	if model.editing {
		t.Fatal("without Vim, Esc still cancels")
	}
}

func TestVimPlannedNotesCoverTheBodyOnly(t *testing.T) {
	model := vimModel(t)
	updated, _ := model.openPlannedNotes(true)
	model = updated.(Model)
	title := model.planTitle.Value()
	// The title field is a plain input: letters type.
	model = vimPress(t, model, "end", "x")
	if model.planTitle.Value() != title+"x" {
		t.Fatalf("title should take text, got %q", model.planTitle.Value())
	}
	model = vimPress(t, model, "tab")
	if model.planField != 1 {
		t.Fatal("Tab should reach the body")
	}
	body := model.planBody.Value()
	model = vimPress(t, model, "g", "g", "d", "d")
	if model.planBody.Value() == body || strings.HasPrefix(model.planBody.Value(), "# Next session") {
		t.Fatalf("dd should delete the first body line, got %q", model.planBody.Value())
	}
	model = vimPress(t, model, "esc")
	if !model.planning {
		t.Fatal("Esc in Normal mode must not close prep")
	}
	model = vimPress(t, model, "tab")
	if model.planField != 0 {
		t.Fatal("Tab must still switch fields in Normal mode")
	}
	model = vimPress(t, model, "tab", ":", "w", "enter")
	if !model.planning {
		t.Fatalf(":w should keep prep open, status %q", model.status)
	}
	saved := model.plannedByID(model.planID)
	if saved == nil || saved.Body != model.planBody.Value() {
		t.Fatal(":w should persist the prep body")
	}
	model = vimPress(t, model, ":", "q", "enter")
	if model.planning {
		t.Fatalf(":q after :w should close prep, status %q", model.status)
	}
}

func TestPlannedNotesEditorKeysWithoutVim(t *testing.T) {
	model := New()
	model.width, model.height = 100, 36
	updated, _ := model.openPlannedNotes(true)
	model = vimPress(t, updated.(Model), "shift+tab")
	if model.planField != 1 {
		t.Fatal("Shift+Tab should switch fields")
	}
	model = vimPress(t, model, "j")
	if !strings.Contains(model.planBody.Value(), "j") {
		t.Fatal("without Vim the body takes letters as text")
	}
	model = vimPress(t, model, "ctrl+p")
	if model.status == "" {
		t.Fatal("Ctrl+P should report prior sits")
	}
	model.planBody.SetValue(model.planBody.Value() + "\n@Cap")
	model.refreshEditorSuggestions()
	if len(model.suggestions) < 1 {
		t.Fatal("expected @ suggestions")
	}
	model = vimPress(t, model, "down", "up", "tab")
	if !strings.Contains(model.planBody.Value(), "@Captain Vale") {
		t.Fatalf("Tab should complete the chosen suggestion: %q", model.planBody.Value())
	}
	model = vimPress(t, model, "esc")
	if model.planning {
		t.Fatal("without Vim, Esc closes prep")
	}
	updated, _ = model.openPlannedNotes(true)
	model = vimPress(t, updated.(Model), "ctrl+s")
	if model.planning || !strings.HasPrefix(model.status, "Saved planned notes") {
		t.Fatalf("Ctrl+S should save and close, status %q", model.status)
	}
}

func TestVimToggleIsAPaletteCommandAndDocumented(t *testing.T) {
	model := New()
	model.width, model.height = 100, 36
	found := false
	for _, command := range model.paletteCommands() {
		if command.ID == "editor.vim" {
			found = strings.Contains(command.Label, "(off)")
		}
	}
	if !found {
		t.Fatal("palette should offer the Vim toggle with its state")
	}
	updated, _ := paletteCommandExecutors()["editor.vim"](model)
	model = updated.(Model)
	if !model.layout.VimEditing {
		t.Fatal("toggle should turn Vim on")
	}
	model = vimPress(t, model, "n", "?")
	if !strings.Contains(model.View().Content, "Vim keys") {
		t.Fatal("help should document Vim keys while they are on")
	}
}

func TestRestoreTextAreaCursorReachesALineBelowSoftWrappedText(t *testing.T) {
	ta := newMarkdownTextArea(60, 20)
	long := strings.Repeat("wrapping words ", 20)
	ta.SetValue(long + "\nsecond\nthird")
	restoreTextAreaCursor(&ta, 2, 3)
	if ta.Line() != 2 || ta.Column() != 3 {
		t.Fatalf("cursor at line %d col %d, want 2,3", ta.Line(), ta.Column())
	}
}

func TestVimWritesBackOnlyChangedText(t *testing.T) {
	ta := newMarkdownTextArea(60, 20)
	ta.SetValue("a\nb")
	before := readVimBuffer(&ta)
	after := before.clone()
	after.moveTo(vimPos{1, 0})
	writeVimBuffer(&ta, before, after)
	if ta.Value() != "a\nb" || ta.Line() != 1 {
		t.Fatalf("motion write-back: %q line %d", ta.Value(), ta.Line())
	}
}
