package tui

import (
	"sort"

	"charm.land/bubbles/v2/textarea"
)

// The textarea draws selections itself but accepts them only as cells
// relative to its own top-left, the way a mouse drag reports them. Finding a
// buffer position's cell by searching the textarea's own PositionAt keeps
// soft wrapping, the line-number gutter, and scrolling exactly as it draws.

// textAreaCell returns the textarea-relative cell that PositionAt maps to p.
func textAreaCell(ta *textarea.Model, p vimPos) (int, int) {
	target := textarea.Position{Row: p.row, Col: p.col}
	bound := len([]rune(ta.Value())) + ta.LineCount()
	// The display row is the last one that starts at or before p. Rows past
	// the end start at the buffer's end, so p at the very end still resolves.
	first := sort.Search(2*bound+1, func(i int) bool {
		return positionAfter(ta.PositionAt(0, i-bound), target)
	})
	y := first - 1 - bound
	// The column is the first cell on that row that reaches p.
	x := sort.Search(ta.Width()+64, func(x int) bool {
		return !positionAfter(target, ta.PositionAt(x, y))
	})
	return x, y
}

func positionAfter(a, b textarea.Position) bool {
	return a.Row > b.Row || (a.Row == b.Row && a.Col > b.Col)
}

// paintVimSelection mirrors the Visual selection into the textarea, which
// draws it, and clears it in every other mode so typing never replaces it.
func paintVimSelection(ta *textarea.Model, v vimState, b vimBuffer) {
	ta.ClearSelection()
	if !v.visual() {
		return
	}
	from, to := v.selection(b)
	// The textarea's range is half-open; Vim's includes its last character.
	to.col = min(to.col+1, b.lineLen(to.row))
	if v.mode == vimVisualLine {
		to.col = b.lineLen(to.row)
	}
	fromX, fromY := textAreaCell(ta, from)
	toX, toY := textAreaCell(ta, to)
	ta.BeginSelection(fromX, fromY)
	ta.ExtendSelection(toX, toY)
	ta.EndSelection()
	// Selecting moved the textarea's cursor to the far end; Vim's cursor is
	// the end that moves, which may be either one.
	restoreTextAreaCursor(ta, b.row, b.col)
}
