package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
)

// Every buffer position must map to a cell the textarea maps straight back,
// across soft wraps, wide runes, line ends, and rows scrolled out of view.
func TestTextAreaCellRoundTripsThroughPositionAt(t *testing.T) {
	ta := newMarkdownTextArea(60, 12)
	lines := []string{strings.Repeat("wrapping words ", 12), "héllo 世界 wide", ""}
	for index := range 30 {
		lines = append(lines, fmt.Sprintf("line %d", index))
	}
	ta.SetValue(strings.Join(lines, "\n"))
	for _, scrolled := range []bool{false, true} {
		row := 0
		if scrolled {
			row = len(lines) - 1
		}
		restoreTextAreaCursor(&ta, row, 0)
		for _, p := range []vimPos{{0, 0}, {0, 70}, {0, len([]rune(lines[0]))}, {1, 6}, {1, 8}, {2, 0}, {20, 3}, {len(lines) - 1, 6}} {
			x, y := textAreaCell(&ta, p)
			if got := ta.PositionAt(x, y); got != (textarea.Position{Row: p.row, Col: p.col}) {
				t.Fatalf("scrolled=%v: cell (%d,%d) for %v maps back to %v", scrolled, x, y, p, got)
			}
		}
	}
}

func TestVimVisualSelectionIsPaintedAndCleared(t *testing.T) {
	model := vimPress(t, vimModel(t), "n", "0", "w", "v", "e")
	if got := model.editBody.SelectedText(); got != "New" {
		t.Fatalf("v e should paint the word: %q", got)
	}
	view := model.View().Content
	// Lipgloss folds reverse video into the inherited foreground's sequence.
	if !strings.Contains(view, "\x1b[7m") && !strings.Contains(view, "\x1b[7;") {
		t.Fatal("the selection should draw in reverse video")
	}
	if codes := BackgroundCodes(view); len(codes) != 0 {
		t.Fatalf("the selection must not paint a background: %v", codes)
	}
	model = vimPress(t, model, "o")
	if got := model.editBody.SelectedText(); got != "New" || model.editBody.Column() != 2 {
		t.Fatalf("o keeps the selection and moves the cursor: %q col %d", got, model.editBody.Column())
	}
	model = vimPress(t, model, "V")
	if got := model.editBody.SelectedText(); got != "# New NPC" {
		t.Fatalf("V should paint the whole line: %q", got)
	}
	model = vimPress(t, model, "esc")
	if model.editBody.HasSelection() {
		t.Fatal("Esc should clear the painted selection")
	}
	model = vimPress(t, model, "v", "e", "c", "X")
	if model.editBody.HasSelection() || !strings.Contains(model.editBody.Value(), "# X NPC") {
		t.Fatalf("c should clear the selection so Insert types normally: %q", model.editBody.Value())
	}
}

func TestVimVisualSelectionFollowsWrappedAndScrolledLines(t *testing.T) {
	model := vimPress(t, vimModel(t), "n")
	long := strings.Repeat("word ", 40)
	lines := []string{long}
	for index := range 40 {
		lines = append(lines, fmt.Sprintf("line %d", index))
	}
	model.editBody.SetValue(strings.Join(lines, "\n"))
	restoreTextAreaCursor(&model.editBody, 0, 0)
	model = vimPress(t, model, "v", "$")
	if got := model.editBody.SelectedText(); got != long {
		t.Fatalf("v $ across a soft wrap: %q", got)
	}
	model = vimPress(t, model, "esc", "G")
	model.View()
	model = vimPress(t, model, "V", "k")
	if got := model.editBody.SelectedText(); got != "line 38\nline 39" {
		t.Fatalf("V k on scrolled lines: %q", got)
	}
	if model.editBody.Line() != 39 {
		t.Fatalf("the cursor should stay on the moving end, line %d", model.editBody.Line())
	}
}
