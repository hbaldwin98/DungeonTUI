package tui

import (
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// Vim editing is an opt-in keymap over the two full-screen Markdown editors.
// The engine works on a plain line buffer so every motion and operator is a
// pure function; the textarea only ever receives the resulting text and
// cursor. Insert mode hands keys back to the editor, so @ suggestions, Tab,
// and the Ctrl chords behave exactly as they do without Vim.

type vimMode int

const (
	vimNormal vimMode = iota
	vimInsert
	vimVisual
	vimVisualLine
)

func (mode vimMode) label() string {
	switch mode {
	case vimInsert:
		return "INSERT"
	case vimVisual:
		return "VISUAL"
	case vimVisualLine:
		return "VISUAL LINE"
	}
	return "NORMAL"
}

// vimAction is what an ex command or ZZ/ZQ asks the editor to do.
type vimAction int

const (
	vimNone vimAction = iota
	vimWrite
	vimQuit
	vimWriteQuit
	vimForceQuit
)

type vimPos struct{ row, col int }

func (p vimPos) before(o vimPos) bool {
	return p.row < o.row || (p.row == o.row && p.col < o.col)
}

type vimBuffer struct {
	lines [][]rune
	row   int
	col   int
}

func newVimBuffer(text string, row, col int) vimBuffer {
	parts := strings.Split(text, "\n")
	lines := make([][]rune, len(parts))
	for index, part := range parts {
		lines[index] = []rune(part)
	}
	b := vimBuffer{lines: lines, row: row, col: col}
	b.row = clamp(b.row, 0, len(b.lines)-1)
	b.col = clamp(b.col, 0, len(b.lines[b.row]))
	return b
}

func (b vimBuffer) text() string {
	parts := make([]string, len(b.lines))
	for index, line := range b.lines {
		parts[index] = string(line)
	}
	return strings.Join(parts, "\n")
}

func (b vimBuffer) clone() vimBuffer {
	lines := make([][]rune, len(b.lines))
	for index, line := range b.lines {
		lines[index] = append([]rune(nil), line...)
	}
	return vimBuffer{lines: lines, row: b.row, col: b.col}
}

func (b vimBuffer) pos() vimPos { return vimPos{b.row, b.col} }

func (b vimBuffer) lineLen(row int) int { return len(b.lines[row]) }

// normalCol keeps the cursor on a character, as Normal mode does.
func (b vimBuffer) normalCol(row, col int) int {
	return clamp(col, 0, max(0, b.lineLen(row)-1))
}

func (b *vimBuffer) moveTo(p vimPos) {
	b.row = clamp(p.row, 0, len(b.lines)-1)
	b.col = b.normalCol(b.row, p.col)
}

func (b vimBuffer) firstNonBlank(row int) int {
	for index, r := range b.lines[row] {
		if !unicode.IsSpace(r) {
			return index
		}
	}
	return 0
}

type vimClass int

const (
	vimSpace vimClass = iota
	vimWordChar
	vimPunct
)

func (b vimBuffer) class(p vimPos) vimClass {
	if p.col >= b.lineLen(p.row) {
		return vimSpace
	}
	r := b.lines[p.row][p.col]
	switch {
	case unicode.IsSpace(r):
		return vimSpace
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return vimWordChar
	default:
		return vimPunct
	}
}

// next steps one character forward; crossed reports a line change.
func (b vimBuffer) next(p vimPos) (vimPos, bool, bool) {
	if p.col < b.lineLen(p.row)-1 {
		return vimPos{p.row, p.col + 1}, false, true
	}
	if p.row < len(b.lines)-1 {
		return vimPos{p.row + 1, 0}, true, true
	}
	return p, false, false
}

func (b vimBuffer) prev(p vimPos) (vimPos, bool, bool) {
	if p.col > 0 {
		return vimPos{p.row, min(p.col-1, max(0, b.lineLen(p.row)-1))}, false, true
	}
	if p.row > 0 {
		return vimPos{p.row - 1, max(0, b.lineLen(p.row-1)-1)}, true, true
	}
	return p, false, false
}

func (b vimBuffer) wordForward(p vimPos) vimPos {
	start := p
	if c := b.class(p); c != vimSpace {
		for {
			n, crossed, ok := b.next(p)
			if !ok {
				return vimPos{p.row, b.lineLen(p.row)}
			}
			p = n
			if crossed || b.class(p) != c {
				break
			}
		}
	}
	for b.class(p) == vimSpace {
		if b.lineLen(p.row) == 0 && p.row != start.row {
			return p
		}
		n, _, ok := b.next(p)
		if !ok {
			return vimPos{p.row, b.lineLen(p.row)}
		}
		p = n
	}
	return p
}

func (b vimBuffer) wordEnd(p vimPos) vimPos {
	p, _, ok := b.next(p)
	if !ok {
		return p
	}
	for b.class(p) == vimSpace {
		if p, _, ok = b.next(p); !ok {
			return p
		}
	}
	c := b.class(p)
	for {
		n, crossed, ok := b.next(p)
		if !ok || crossed || b.class(n) != c {
			return p
		}
		p = n
	}
}

func (b vimBuffer) wordBackward(p vimPos) vimPos {
	p, _, ok := b.prev(p)
	if !ok {
		return p
	}
	for b.class(p) == vimSpace && b.lineLen(p.row) > 0 {
		if p, _, ok = b.prev(p); !ok {
			return p
		}
	}
	c := b.class(p)
	for {
		n, crossed, ok := b.prev(p)
		if !ok || crossed || b.class(n) != c {
			return p
		}
		p = n
	}
}

func (b vimBuffer) blankLine(row int) bool {
	return strings.TrimSpace(string(b.lines[row])) == ""
}

// paragraph finds the next (dir 1) or previous (dir -1) blank line.
func (b vimBuffer) paragraph(row, dir int) int {
	row += dir
	for row > 0 && row < len(b.lines)-1 && !b.blankLine(row) {
		row += dir
	}
	return clamp(row, 0, len(b.lines)-1)
}

// vimMotion is a resolved cursor target plus how an operator treats it.
type vimMotion struct {
	to        vimPos
	linewise  bool
	inclusive bool
}

// motion resolves key applied count times, or ok=false if key is no motion.
// explicit is whether the owner typed a count, which changes G and gg.
func (b vimBuffer) motion(key string, count int, explicit bool) (vimMotion, bool) {
	p := b.pos()
	last := len(b.lines) - 1
	switch key {
	case "h", "left", "backspace":
		return vimMotion{to: vimPos{p.row, max(0, p.col-count)}}, true
	case "l", "right", "space":
		return vimMotion{to: vimPos{p.row, min(b.lineLen(p.row), p.col+count)}}, true
	case "j", "down", "enter", "ctrl+j":
		return vimMotion{to: vimPos{min(last, p.row+count), p.col}, linewise: true}, true
	case "k", "up", "ctrl+p":
		return vimMotion{to: vimPos{max(0, p.row-count), p.col}, linewise: true}, true
	case "0", "home":
		return vimMotion{to: vimPos{p.row, 0}}, true
	case "^":
		return vimMotion{to: vimPos{p.row, b.firstNonBlank(p.row)}}, true
	case "$", "end":
		row := min(last, p.row+count-1)
		return vimMotion{to: vimPos{row, max(0, b.lineLen(row)-1)}, inclusive: true}, true
	case "G":
		row := last
		if explicit {
			row = clamp(count-1, 0, last)
		}
		return vimMotion{to: vimPos{row, b.firstNonBlank(row)}, linewise: true}, true
	case "gg":
		row := 0
		if explicit {
			row = clamp(count-1, 0, last)
		}
		return vimMotion{to: vimPos{row, b.firstNonBlank(row)}, linewise: true}, true
	case "w", "e", "b":
		step := map[string]func(vimPos) vimPos{"w": b.wordForward, "e": b.wordEnd, "b": b.wordBackward}[key]
		for range count {
			p = step(p)
		}
		return vimMotion{to: p, inclusive: key == "e"}, true
	case "}", "{":
		dir := map[string]int{"}": 1, "{": -1}[key]
		row := p.row
		for range count {
			row = b.paragraph(row, dir)
		}
		return vimMotion{to: vimPos{row, 0}}, true
	}
	return vimMotion{}, false
}

// deleteRange removes the text in [from, to) and returns it.
func (b *vimBuffer) deleteRange(from, to vimPos) string {
	if to.before(from) {
		from, to = to, from
	}
	to.col = min(to.col, b.lineLen(to.row))
	from.col = min(from.col, b.lineLen(from.row))
	var removed strings.Builder
	if from.row == to.row {
		removed.WriteString(string(b.lines[from.row][from.col:to.col]))
	} else {
		removed.WriteString(string(b.lines[from.row][from.col:]))
		for row := from.row + 1; row < to.row; row++ {
			removed.WriteString("\n" + string(b.lines[row]))
		}
		removed.WriteString("\n" + string(b.lines[to.row][:to.col]))
	}
	merged := append(append([]rune(nil), b.lines[from.row][:from.col]...), b.lines[to.row][to.col:]...)
	lines := append(append([][]rune(nil), b.lines[:from.row]...), merged)
	b.lines = append(lines, b.lines[to.row+1:]...)
	b.row, b.col = from.row, from.col
	return removed.String()
}

func (b vimBuffer) lineText(from, to int) string {
	parts := make([]string, 0, to-from+1)
	for row := from; row <= to; row++ {
		parts = append(parts, string(b.lines[row]))
	}
	return strings.Join(parts, "\n")
}

// deleteLines removes whole rows [from, to]; a buffer never loses its last
// line, it becomes empty.
func (b *vimBuffer) deleteLines(from, to int) {
	b.lines = append(append([][]rune(nil), b.lines[:from]...), b.lines[to+1:]...)
	if len(b.lines) == 0 {
		b.lines = [][]rune{{}}
	}
	b.row = min(from, len(b.lines)-1)
	b.col = b.firstNonBlank(b.row)
}

func (b *vimBuffer) insertText(at vimPos, text string) vimPos {
	parts := strings.Split(text, "\n")
	line := b.lines[at.row]
	head := append([]rune(nil), line[:at.col]...)
	tail := append([]rune(nil), line[at.col:]...)
	inserted := make([][]rune, len(parts))
	for index, part := range parts {
		inserted[index] = []rune(part)
	}
	end := vimPos{at.row + len(parts) - 1, len(inserted[len(parts)-1])}
	if len(parts) == 1 {
		end.col += len(head)
	}
	inserted[0] = append(head, inserted[0]...)
	inserted[len(inserted)-1] = append(inserted[len(inserted)-1], tail...)
	lines := append(append([][]rune(nil), b.lines[:at.row]...), inserted...)
	b.lines = append(lines, b.lines[at.row+1:]...)
	return end
}

func (b *vimBuffer) insertLines(at int, text string) {
	inserted := [][]rune{}
	for _, part := range strings.Split(text, "\n") {
		inserted = append(inserted, []rune(part))
	}
	lines := append(append([][]rune(nil), b.lines[:at]...), inserted...)
	b.lines = append(lines, b.lines[at:]...)
}

type vimSnapshot struct {
	text     string
	row, col int
}

// vimState is the per-editor Vim session. Its zero value is Normal mode.
type vimState struct {
	mode     vimMode
	count    string
	pending  string // an operator (d c y), or a prefix waiting for one key (g r Z)
	opCount  int
	commandL bool
	command  string
	register string
	linewise bool
	undo     []vimSnapshot
	redo     []vimSnapshot
	// want is the column j and k aim for while hasWant holds.
	want    int
	hasWant bool
	// insertMark is set when Insert mode pushed its own undo step, so an
	// insert that typed nothing leaves no empty step behind.
	insertMark bool
	// saved is the text as of the last save, so :q can refuse to drop work.
	saved string
	// cmdPrefix is ':' for an ex command and '/' for a search.
	cmdPrefix rune
	search    string
	// notice is a message for the editor's status line, such as a failed search.
	notice string
	// anchor is where a Visual selection began; the cursor is its other end.
	anchor vimPos
	// regSel is the register a "x prefix chose for the command in progress,
	// regFresh marks the key that chose it, and named holds a-z.
	regSel   rune
	regFresh bool
	named    map[rune]vimRegister
	// Dot repeat: rec collects the command in progress and lastChange the
	// keys of the last one that changed text; lastInsert is what its Insert
	// session typed. edits counts undo snapshots so a change is detectable.
	rec, lastChange, insertRec []string
	lastInsert, insertBase     string
	insertAt, edits, editMark  int
	replaying                  bool
}

const vimUndoLimit = 200

func newVimState(saved string) vimState { return vimState{saved: saved} }

func (v vimState) modified(text string) bool { return text != v.saved }

func (v *vimState) snapshot(b vimBuffer) {
	v.edits++
	v.undo = append(v.undo, vimSnapshot{b.text(), b.row, b.col})
	if len(v.undo) > vimUndoLimit {
		v.undo = v.undo[len(v.undo)-vimUndoLimit:]
	}
	v.redo = nil
}

func (v *vimState) reset() {
	v.count, v.pending, v.opCount = "", "", 0
}

// status is the Vim part of the editor footer.
func (v vimState) status() string {
	if v.commandL {
		prefix := v.cmdPrefix
		if prefix == 0 {
			prefix = ':'
		}
		return string(prefix) + v.command
	}
	label := v.mode.label()
	if v.visual() {
		// The textarea cannot paint the selection, so name where it began.
		label += " from " + strconv.Itoa(v.anchor.row+1) + ":" + strconv.Itoa(v.anchor.col+1)
	}
	pending := v.pending + v.count
	if v.pending != "" && v.opCount > 1 {
		pending = strconv.Itoa(v.opCount) + pending
	}
	if v.regSel != 0 {
		pending = `"` + string(v.regSel) + pending
	}
	if pending != "" && v.mode != vimInsert {
		label += " " + pending
	}
	return label
}

func (v *vimState) enterInsert(b vimBuffer, pushUndo bool) {
	if pushUndo {
		v.snapshot(b)
	}
	v.insertMark = pushUndo
	v.mode = vimInsert
	v.reset()
}

// leaveInsert returns to Normal mode, stepping the cursor back onto a
// character as Vim does, and drops an undo step for an insert that typed
// nothing.
func (v *vimState) leaveInsert(b vimBuffer) vimBuffer {
	if !v.replaying && v.insertRec != nil {
		v.lastChange, v.lastInsert = v.insertRec, insertedText(v.insertBase, v.insertAt, b.text())
		v.insertRec = nil
	}
	if v.insertMark && len(v.undo) > 0 && v.undo[len(v.undo)-1].text == b.text() {
		v.undo = v.undo[:len(v.undo)-1]
	}
	v.insertMark = false
	v.mode = vimNormal
	b.col = b.normalCol(b.row, b.col-1)
	return b
}

func (v *vimState) takeCount() (int, bool) {
	count, err := strconv.Atoi(v.count)
	v.count = ""
	if err != nil || count < 1 {
		return 1, false
	}
	return min(count, 10000), true
}

// applyKey runs one Normal- or Visual-mode key. It returns the updated
// buffer and any editor action the key requested.
func (v *vimState) applyKey(b vimBuffer, key string) (vimBuffer, vimAction) {
	if v.commandL {
		if v.cmdPrefix == '/' {
			return v.searchLineKey(b, key), vimNone
		}
		return b, v.commandKey(key)
	}
	if key == "esc" {
		return v.escape(b), vimNone
	}
	if v.pending == `"` {
		v.pending = ""
		v.selectRegister(key)
		return b, vimNone
	}
	// r takes any character, digits included.
	if isVimDigit(key) && (key != "0" || v.count != "") && v.pending != "r" {
		v.count += key
		return b, vimNone
	}
	if key == `"` && v.pending == "" {
		v.pending = `"`
		return b, vimNone
	}
	if v.visual() {
		return v.visualKey(b, key), vimNone
	}
	switch v.pending {
	case "di", "da", "ci", "ca", "yi", "ya":
		return v.objectOperator(b, key), vimNone
	case "r":
		return v.replaceChar(b, key), vimNone
	case "Z":
		v.reset()
		return b, map[string]vimAction{"Z": vimWriteQuit, "Q": vimForceQuit}[key]
	case "g":
		v.pending = ""
		if key != "g" {
			v.reset()
			return b, vimNone
		}
		key = "gg"
	case "d", "c", "y":
		return v.operatorKey(b, key), vimNone
	case "dg", "cg", "yg":
		if key != "g" {
			v.reset()
			return b, vimNone
		}
		v.pending = v.pending[:1]
		return v.operatorKey(b, "gg"), vimNone
	}
	return v.normalKey(b, key)
}

var vimVertical = map[string]bool{"j": true, "k": true, "up": true, "down": true, "ctrl+j": true, "ctrl+p": true, "enter": true}

func isVimDigit(key string) bool {
	return len(key) == 1 && key[0] >= '0' && key[0] <= '9'
}

func (v *vimState) normalKey(b vimBuffer, key string) (vimBuffer, vimAction) {
	count, explicit := v.takeCount()
	if m, ok := v.motion(b, key, count, explicit); ok {
		return v.moveCursor(b, key, m), vimNone
	}
	v.hasWant = false
	switch key {
	case "d", "c", "y", "g", "r", "Z":
		v.pending = key
		v.opCount = count
		return b, vimNone
	case ":", "/":
		v.commandL, v.command, v.cmdPrefix = true, "", []rune(key)[0]
		return b, vimNone
	case "v", "V":
		v.mode, v.anchor = map[string]vimMode{"v": vimVisual, "V": vimVisualLine}[key], b.pos()
		return b, vimNone
	case "u":
		return v.undoStep(b, count), vimNone
	case "ctrl+r":
		return v.redoStep(b, count), vimNone
	case "p", "P":
		return v.put(b, key == "P", count), vimNone
	case "J":
		return v.join(b, max(2, count)), vimNone
	case "~":
		return v.toggleCase(b, count), vimNone
	}
	return v.editKey(b, key, count), vimNone
}

// moveCursor carries out a motion. j and k aim for the column the owner was
// last on, so crossing a short or empty line does not lose it; $ aims for
// every line end.
func (v *vimState) moveCursor(b vimBuffer, key string, m vimMotion) vimBuffer {
	if vimVertical[key] {
		if !v.hasWant {
			v.want, v.hasWant = b.col, true
		}
		m.to.col = v.want
	} else {
		v.hasWant = false
	}
	b.moveTo(m.to)
	if key == "$" || key == "end" {
		v.want, v.hasWant = int(^uint(0)>>1), true
	}
	return b
}

// escape leaves Visual mode and drops anything half-typed.
func (v *vimState) escape(b vimBuffer) vimBuffer {
	if v.visual() {
		v.mode = vimNormal
		b.col = b.normalCol(b.row, b.col)
	}
	v.reset()
	v.regSel, v.regFresh = 0, false
	return b
}

// editKey handles the insert entries and the one-key edits that are short
// forms of an operator plus motion.
func (v *vimState) editKey(b vimBuffer, key string, count int) vimBuffer {
	switch key {
	case "i", "insert":
		v.enterInsert(b, true)
	case "a":
		v.enterInsert(b, true)
		b.col = min(b.col+1, b.lineLen(b.row))
	case "I":
		v.enterInsert(b, true)
		b.col = b.firstNonBlank(b.row)
	case "A":
		v.enterInsert(b, true)
		b.col = b.lineLen(b.row)
	case "o", "O":
		v.enterInsert(b, true)
		at := b.row
		if key == "o" {
			at++
		}
		b.insertLines(at, "")
		b.row, b.col = at, 0
	case "x", "delete", "X":
		return v.deleteChars(b, key == "X", count)
	case "D", "C", "s", "S", "Y":
		v.opCount = count
		v.pending = map[string]string{"D": "d", "C": "c", "s": "c", "S": "c", "Y": "y"}[key]
		motion := map[string]string{"D": "$", "C": "$", "s": "l", "S": v.pending, "Y": "y"}[key]
		return v.operatorKey(b, motion)
	}
	return b
}

func (v *vimState) deleteChars(b vimBuffer, backward bool, count int) vimBuffer {
	if b.lineLen(b.row) == 0 || (backward && b.col == 0) {
		return b
	}
	v.snapshot(b)
	from, to := vimPos{b.row, b.col}, vimPos{b.row, min(b.lineLen(b.row), b.col+count)}
	if backward {
		from, to = vimPos{b.row, max(0, b.col-count)}, vimPos{b.row, b.col}
	}
	v.setRegister(b.deleteRange(from, to), false)
	b.col = b.normalCol(b.row, b.col)
	return b
}

// operatorKey completes d, c, or y with a motion, or with itself for lines.
func (v *vimState) operatorKey(b vimBuffer, key string) vimBuffer {
	op := v.pending
	motionCount, explicit := v.takeCount()
	count := max(1, v.opCount) * motionCount
	explicit = explicit || v.opCount > 1
	if key == "g" || key == "i" || key == "a" {
		v.pending = op + key
		return b
	}
	v.reset()
	var m vimMotion
	switch {
	case key == op:
		m = vimMotion{to: vimPos{min(len(b.lines)-1, b.row+count-1), 0}, linewise: true}
	case op == "c" && key == "w" && b.class(b.pos()) != vimSpace:
		// Vim's cw changes to the end of the word, not onto the next one.
		m, _ = b.motion("e", count, explicit)
	default:
		var ok bool
		if m, ok = v.motion(b, key, count, explicit); !ok {
			return b
		}
		if key == "w" && m.to.row > b.row {
			// An operator never carries w's jump onto the next line.
			m.to = vimPos{b.row, b.lineLen(b.row)}
		}
	}
	return v.operate(b, op, m)
}

func (v *vimState) operate(b vimBuffer, op string, m vimMotion) vimBuffer {
	start := b.pos()
	if m.linewise {
		from, to := min(start.row, m.to.row), max(start.row, m.to.row)
		v.setRegister(b.lineText(from, to), true)
		if op == "y" {
			b.moveTo(vimPos{from, b.col})
			return b
		}
		v.snapshot(b)
		if op == "c" {
			// cc keeps one line, and its indentation, to type into.
			indent := append([]rune(nil), b.lines[from][:b.firstNonBlank(from)]...)
			lines := append(append([][]rune(nil), b.lines[:from]...), indent)
			b.lines = append(lines, b.lines[to+1:]...)
			b.row, b.col = from, len(indent)
			v.enterInsert(b, false)
			return b
		}
		b.deleteLines(from, to)
		return b
	}
	from, to := start, m.to
	if to.before(from) {
		from, to = to, from
	}
	if m.inclusive {
		to.col++
	}
	if op == "y" {
		scratch := b.clone()
		v.setRegister(scratch.deleteRange(from, to), false)
		b.moveTo(from)
		return b
	}
	v.snapshot(b)
	v.setRegister(b.deleteRange(from, to), false)
	if op == "c" {
		v.enterInsert(b, false)
		return b
	}
	b.col = b.normalCol(b.row, b.col)
	return b
}

func (v *vimState) put(b vimBuffer, before bool, count int) vimBuffer {
	register, linewise := v.getRegister()
	if register == "" && !linewise {
		return b
	}
	v.snapshot(b)
	text := strings.Repeat(register, count)
	if linewise {
		text = strings.TrimSuffix(strings.Repeat(register+"\n", count), "\n")
		at := b.row + 1
		if before {
			at = b.row
		}
		b.insertLines(at, text)
		b.row = at
		b.col = b.firstNonBlank(at)
		return b
	}
	if !before && b.lineLen(b.row) > 0 {
		b.col++
	}
	end := b.insertText(b.pos(), text)
	b.moveTo(vimPos{end.row, end.col - 1})
	return b
}

func (v *vimState) join(b vimBuffer, count int) vimBuffer {
	if b.row >= len(b.lines)-1 {
		return b
	}
	v.snapshot(b)
	for range count - 1 {
		if b.row >= len(b.lines)-1 {
			break
		}
		head := strings.TrimRight(string(b.lines[b.row]), " \t")
		tail := strings.TrimLeft(string(b.lines[b.row+1]), " \t")
		joined := head
		if head != "" && tail != "" {
			joined += " "
		}
		// The cursor rests on the join: the added space, or the first
		// joined character when no space was needed.
		b.col = len([]rune(head))
		if tail == "" {
			b.col = max(0, b.col-1)
		}
		b.lines[b.row] = []rune(joined + tail)
		b.lines = append(b.lines[:b.row+1], b.lines[b.row+2:]...)
	}
	b.col = b.normalCol(b.row, b.col)
	return b
}

func (v *vimState) toggleCase(b vimBuffer, count int) vimBuffer {
	if b.lineLen(b.row) == 0 {
		return b
	}
	v.snapshot(b)
	line := b.lines[b.row]
	end := min(len(line), b.col+count)
	for index := b.col; index < end; index++ {
		line[index] = flipCase(line[index])
	}
	b.col = b.normalCol(b.row, end)
	return b
}

func (v *vimState) replaceChar(b vimBuffer, key string) vimBuffer {
	count := max(1, v.opCount)
	v.reset()
	r := []rune(key)
	if key == "space" {
		r = []rune{' '}
	}
	if len(r) != 1 || b.col+count > b.lineLen(b.row) {
		return b
	}
	v.snapshot(b)
	for index := b.col; index < b.col+count; index++ {
		b.lines[b.row][index] = r[0]
	}
	b.col += count - 1
	return b
}

func (v *vimState) undoStep(b vimBuffer, count int) vimBuffer {
	for range count {
		if len(v.undo) == 0 {
			break
		}
		last := v.undo[len(v.undo)-1]
		v.undo = v.undo[:len(v.undo)-1]
		v.redo = append(v.redo, vimSnapshot{b.text(), b.row, b.col})
		b = newVimBuffer(last.text, last.row, last.col)
	}
	b.col = b.normalCol(b.row, b.col)
	return b
}

func (v *vimState) redoStep(b vimBuffer, count int) vimBuffer {
	for range count {
		if len(v.redo) == 0 {
			break
		}
		last := v.redo[len(v.redo)-1]
		v.redo = v.redo[:len(v.redo)-1]
		v.undo = append(v.undo, vimSnapshot{b.text(), b.row, b.col})
		b = newVimBuffer(last.text, last.row, last.col)
	}
	b.col = b.normalCol(b.row, b.col)
	return b
}

func (v *vimState) commandKey(key string) vimAction {
	command, done := v.lineKey(key)
	if !done {
		return vimNone
	}
	return map[string]vimAction{
		"w": vimWrite, "write": vimWrite,
		"q": vimQuit, "quit": vimQuit,
		"q!": vimForceQuit, "quit!": vimForceQuit,
		"wq": vimWriteQuit, "x": vimWriteQuit, "xit": vimWriteQuit,
	}[strings.TrimSpace(command)]
}

// lineKey edits the : or / line; done reports Enter with the typed line.
func (v *vimState) lineKey(key string) (string, bool) {
	switch key {
	case "esc":
		v.commandL, v.command = false, ""
	case "backspace":
		if v.command == "" {
			v.commandL = false
		} else {
			runes := []rune(v.command)
			v.command = string(runes[:len(runes)-1])
		}
	case "enter":
		command := v.command
		v.commandL, v.command = false, ""
		return command, true
	case "space":
		v.command += " "
	default:
		if len([]rune(key)) == 1 {
			v.command += key
		}
	}
	return "", false
}

// readVimBuffer and writeVimBuffer bridge the engine and a textarea. Only a
// changed text is written back, so pure motions keep the textarea's own
// state such as its scroll position.
func readVimBuffer(ta *textarea.Model) vimBuffer {
	return newVimBuffer(ta.Value(), ta.Line(), ta.Column())
}

func writeVimBuffer(ta *textarea.Model, before, after vimBuffer) {
	if text := after.text(); text != before.text() {
		ta.SetValue(text)
	}
	restoreTextAreaCursor(ta, after.row, after.col)
}

// vimEditorKey routes one key through Vim for a Markdown editor. handled is
// false when the editor's own handling should run: Vim is off, the key is
// typed in Insert mode, or it is one of the editor's passthrough chords.
func (m *Model) vimEditorKey(ta *textarea.Model, msg tea.KeyPressMsg, passthrough ...string) (vimAction, bool) {
	if !m.layout.VimEditing {
		return vimNone, false
	}
	key := msg.String()
	if m.vim.mode == vimInsert {
		if key != "esc" && key != "ctrl+[" {
			return vimNone, false
		}
		writeVimBuffer(ta, readVimBuffer(ta), m.vim.leaveInsert(readVimBuffer(ta)))
		m.suggestions = nil
		return vimNone, true
	}
	for _, chord := range passthrough {
		if key == chord && !m.vim.commandL {
			return vimNone, false
		}
	}
	before := readVimBuffer(ta)
	after, action := m.vim.apply(before.clone(), key)
	writeVimBuffer(ta, before, after)
	m.suggestions = nil
	if m.vim.notice != "" {
		m.status, m.vim.notice = m.vim.notice, ""
	}
	return action, true
}

// editorHelpKey reports whether key opens help from a Markdown editor. ? is
// ordinary text there, so it opens help only from Vim Normal mode with
// nothing pending (r? still replaces with ?); Ctrl+G works in every mode.
func (m Model) editorHelpKey(key string) bool {
	if key == "ctrl+g" {
		return true
	}
	vimBody := m.editing || m.planField == 1
	return key == "?" && vimBody && m.layout.VimEditing && m.vim.mode == vimNormal &&
		!m.vim.commandL && m.vim.pending == "" && m.vim.count == ""
}

// vimEditorHelp prefixes an editor footer with the Vim mode when enabled.
func (m Model) vimEditorHelp(help string) string {
	if !m.layout.VimEditing {
		return help
	}
	if m.vim.mode == vimNormal && !m.vim.commandL {
		return m.vim.status() + " · i insert · :w save · :q close · u undo · " + help
	}
	if m.vim.visual() {
		return m.vim.status() + " · d y c ~ J · o other end · Esc normal · " + help
	}
	return m.vim.status() + " · Esc normal · " + help
}

// withVimHelp adds the Vim reference to an editor's help while it is on.
func (m Model) withVimHelp(sections [][]string) [][]string {
	if !m.layout.VimEditing {
		return sections
	}
	return append(sections, []string{"Vim keys", "Normal: h j k l · w b e · 0 ^ $ · gg G · { } · counts",
		"i a I A o O insert · Esc back to Normal",
		"d c y + motion · dd cc yy · x X D C s S Y · p P · J ~ r",
		"v V visual (footer shows where it began) · o other end · d y c x ~ J",
		"iw aw ip ap text objects · diw yap viw",
		"/ search · n N next/previous · lower-case ignores case",
		". repeat last change · \"a register prefix · \"A appends",
		"u undo · Ctrl+R redo",
		":w save · :wq or ZZ save and close · :q close · :q! or ZQ discard"})
}

func (m Model) toggleVimEditing() (tea.Model, tea.Cmd) {
	m.layout.VimEditing = !m.layout.VimEditing
	m.persistPreferences()
	if m.layout.VimEditing {
		m.status = "Vim keys on in the Markdown editors · editors open in Normal mode"
	} else {
		m.status = "Vim keys off in the Markdown editors"
	}
	return m, nil
}

func (m Model) vimCommand() paletteCommand {
	state := "off"
	if m.layout.VimEditing {
		state = "on"
	}
	return paletteCommand{ID: "editor.vim", Label: "Editor: toggle Vim keys (" + state + ")", Aliases: "vim vi modal keybindings markdown"}
}

// vimQuitRefusal explains why :q did not close the editor.
const vimQuitRefusal = "Unsaved changes · :w saves · :wq saves and closes · :q! discards"
