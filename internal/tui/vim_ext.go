package tui

import (
	"strings"
	"unicode"
)

// Visual mode, text objects, search, named registers, and dot repeat. They
// extend the same pure engine: every key still maps a buffer to a buffer.

type vimRegister struct {
	text     string
	linewise bool
}

func (v vimState) visual() bool { return v.mode == vimVisual || v.mode == vimVisualLine }

// reopen starts a new editor session that keeps what Vim keeps across
// buffers: the registers, the last search, and the last change for dot.
func (v vimState) reopen(saved string) vimState {
	next := newVimState(saved)
	next.register, next.linewise, next.named = v.register, v.linewise, v.named
	next.search, next.lastChange, next.lastInsert = v.search, v.lastChange, v.lastInsert
	return next
}

// apply runs one Normal- or Visual-mode key and records the command it
// belongs to, so a command that changed text can be repeated with dot.
func (v *vimState) apply(b vimBuffer, key string) (vimBuffer, vimAction) {
	if key == "." && v.mode == vimNormal && v.pending == "" && !v.commandL {
		count := v.count
		v.count = ""
		return v.repeatChange(b, count), vimNone
	}
	if v.mode == vimNormal && v.pending == "" && v.count == "" && !v.commandL && v.regSel == 0 {
		v.rec, v.editMark = nil, v.edits
	}
	v.rec = append(v.rec, key)
	b, action := v.applyKey(b, key)
	switch {
	case v.mode == vimInsert:
		// leaveInsert completes the change once the insert's text is known.
		v.insertRec, v.insertBase, v.insertAt = v.rec, b.text(), b.offset(b.pos())
		v.rec, v.regSel, v.regFresh = nil, 0, false
	case v.regFresh:
		// "x only chose a register; the command it prefixes comes next.
		v.regFresh = false
	case v.mode == vimNormal && v.pending == "" && v.count == "" && !v.commandL:
		if v.edits != v.editMark {
			v.lastChange, v.lastInsert = v.rec, ""
		}
		v.rec, v.regSel = nil, 0
	}
	return b, action
}

// repeatChange is dot: it replays the last change's keys, with a new count
// replacing the one they were typed with, then retypes what its insert typed.
func (v *vimState) repeatChange(b vimBuffer, count string) vimBuffer {
	if len(v.lastChange) == 0 {
		return b
	}
	keys := v.lastChange
	if count != "" {
		for len(keys) > 0 && isVimDigit(keys[0]) {
			keys = keys[1:]
		}
		keys = append(strings.Split(count, ""), keys...)
	}
	v.replaying = true
	defer func() { v.replaying = false }()
	for _, key := range keys {
		b, _ = v.applyKey(b, key)
	}
	if v.mode == vimInsert {
		end := b.insertText(b.pos(), v.lastInsert)
		b.row, b.col = end.row, end.col
		b = v.leaveInsert(b)
	}
	v.regSel, v.regFresh = 0, false
	return b
}

// offset is p's index into text(), counted in runes.
func (b vimBuffer) offset(p vimPos) int {
	offset := 0
	for row := 0; row < p.row; row++ {
		offset += len(b.lines[row]) + 1
	}
	return offset + min(p.col, b.lineLen(p.row))
}

func (b vimBuffer) posAt(offset int) vimPos {
	for row, line := range b.lines {
		if offset <= len(line) {
			return vimPos{row, offset}
		}
		offset -= len(line) + 1
	}
	last := len(b.lines) - 1
	return vimPos{last, b.lineLen(last)}
}

// insertedText is what an Insert session typed at rune offset at, or "" when
// it also removed text around that point, so dot retypes pure insertions.
func insertedText(base string, at int, final string) string {
	before, after := []rune(base), []rune(final)
	if at > len(before) || len(after) < len(before) {
		return ""
	}
	tail := len(before) - at
	if string(after[:at]) != string(before[:at]) || string(after[len(after)-tail:]) != string(before[at:]) {
		return ""
	}
	return string(after[at : len(after)-tail])
}

// selectRegister takes the key after ". Only a-z, A-Z (append), and the
// unnamed " register exist.
func (v *vimState) selectRegister(key string) {
	r := []rune(key)
	if len(r) == 1 && ((r[0] < unicode.MaxASCII && unicode.IsLetter(r[0])) || r[0] == '"') {
		v.regSel, v.regFresh = r[0], true
	}
}

// setRegister stores a yank or delete in the unnamed register and, after a
// "x prefix, in register x; a capital appends to the lower-case register.
func (v *vimState) setRegister(text string, linewise bool) {
	v.register, v.linewise = text, linewise
	if v.regSel == 0 || v.regSel == '"' {
		return
	}
	if v.named == nil {
		v.named = map[rune]vimRegister{}
	}
	name := unicode.ToLower(v.regSel)
	if prev := v.named[name]; unicode.IsUpper(v.regSel) && prev.text != "" {
		separator := ""
		if prev.linewise || linewise {
			separator = "\n"
		}
		text, linewise = prev.text+separator+text, prev.linewise || linewise
	}
	v.named[name] = vimRegister{text, linewise}
}

func (v vimState) getRegister() (string, bool) {
	if v.regSel == 0 || v.regSel == '"' {
		return v.register, v.linewise
	}
	register := v.named[unicode.ToLower(v.regSel)]
	return register.text, register.linewise
}

// vimObject is a text object's inclusive range.
type vimObject struct {
	from, to vimPos
	linewise bool
}

// textObject resolves i or a (kind) with w or p at the cursor.
func (b vimBuffer) textObject(kind, key string) (vimObject, bool) {
	switch key {
	case "w":
		return b.wordObject(kind == "a")
	case "p":
		return b.paragraphObject(kind == "a"), true
	}
	return vimObject{}, false
}

// wordObject is iw, the run of one character class under the cursor, or aw,
// which adds the following blanks, else the preceding ones.
func (b vimBuffer) wordObject(around bool) (vimObject, bool) {
	row, n := b.row, b.lineLen(b.row)
	if n == 0 {
		return vimObject{}, false
	}
	same := func(col int, c vimClass) bool { return col >= 0 && col < n && b.class(vimPos{row, col}) == c }
	col := min(b.col, n-1)
	c := b.class(vimPos{row, col})
	start, end := col, col
	for same(start-1, c) {
		start--
	}
	for same(end+1, c) {
		end++
	}
	if around {
		switch {
		case c == vimSpace && end+1 < n:
			// On blanks, aw is the blanks plus the word after them.
			next := b.class(vimPos{row, end + 1})
			for same(end+1, next) {
				end++
			}
		case same(end+1, vimSpace):
			for same(end+1, vimSpace) {
				end++
			}
		default:
			for same(start-1, vimSpace) {
				start--
			}
		}
	}
	return vimObject{from: vimPos{row, start}, to: vimPos{row, end}}, true
}

// paragraphObject is ip, the run of non-blank (or blank) lines around the
// cursor, or ap, which adds the run after it, else the one before.
func (b vimBuffer) paragraphObject(around bool) vimObject {
	last := len(b.lines) - 1
	blank := b.blankLine(b.row)
	start, end := b.row, b.row
	for start > 0 && b.blankLine(start-1) == blank {
		start--
	}
	for end < last && b.blankLine(end+1) == blank {
		end++
	}
	if around {
		if end < last {
			for end < last && b.blankLine(end+1) != blank {
				end++
			}
		} else {
			for start > 0 && b.blankLine(start-1) != blank {
				start--
			}
		}
	}
	return vimObject{from: vimPos{start, 0}, to: vimPos{end, 0}, linewise: true}
}

// objectOperator completes d, c, or y with a text object (diw, yap, cip).
func (v *vimState) objectOperator(b vimBuffer, key string) vimBuffer {
	op, kind := v.pending[:1], v.pending[1:]
	v.reset()
	object, ok := b.textObject(kind, key)
	if !ok {
		return b
	}
	b.row, b.col = object.from.row, object.from.col
	return v.operate(b, op, vimMotion{to: object.to, linewise: object.linewise, inclusive: true})
}

// motion resolves a key as a motion, adding n and N, which repeat the last
// search and so depend on the Vim state rather than the buffer alone.
func (v *vimState) motion(b vimBuffer, key string, count int, explicit bool) (vimMotion, bool) {
	if key != "n" && key != "N" {
		return b.motion(key, count, explicit)
	}
	if v.search == "" {
		v.notice = "No previous search · / searches"
		return vimMotion{}, false
	}
	p := b.pos()
	for range count {
		next, ok := b.find(v.search, p, key == "n")
		if !ok {
			v.notice = "Pattern not found: " + v.search
			return vimMotion{}, false
		}
		p = next
	}
	return vimMotion{to: p}, true
}

// find is the next (forward) or previous literal match of pattern from p,
// wrapping around the buffer. A pattern without capitals ignores case.
func (b vimBuffer) find(pattern string, p vimPos, forward bool) (vimPos, bool) {
	text, needle := []rune(b.text()), []rune(pattern)
	if strings.ToLower(pattern) == pattern {
		for index, r := range text {
			text[index] = unicode.ToLower(r)
		}
	}
	n := len(text)
	if n == 0 || len(needle) == 0 {
		return p, false
	}
	start, step := b.offset(p), 1
	if !forward {
		step = -1
	}
	for k := 1; k <= n; k++ {
		at := ((start+k*step)%n + n) % n
		if at+len(needle) <= n && string(text[at:at+len(needle)]) == string(needle) {
			return b.posAt(at), true
		}
	}
	return p, false
}

// searchLineKey edits the / line; Enter searches forward for it, and an
// empty line searches for the previous pattern again.
func (v *vimState) searchLineKey(b vimBuffer, key string) vimBuffer {
	pattern, done := v.lineKey(key)
	if !done {
		return b
	}
	if pattern != "" {
		v.search = pattern
	}
	if m, ok := v.motion(b, "n", 1, false); ok {
		v.hasWant = false
		b.moveTo(m.to)
	}
	return b
}

// selection is the Visual selection in buffer order, both ends inclusive.
func (v vimState) selection(b vimBuffer) (vimPos, vimPos) {
	from, to := v.anchor, b.pos()
	if to.before(from) {
		from, to = to, from
	}
	if v.mode == vimVisualLine {
		from.col, to.col = 0, max(0, b.lineLen(to.row)-1)
	}
	return from, to
}

// vimVisualOps are the operators Visual mode applies to its selection; the
// capitals act on whole lines as they do in Vim.
var vimVisualOps = map[string]struct {
	op       string
	linewise bool
}{
	"d": {"d", false}, "x": {"d", false}, "delete": {"d", false}, "y": {"y", false},
	"c": {"c", false}, "s": {"c", false},
	"D": {"d", true}, "X": {"d", true}, "Y": {"y", true}, "C": {"c", true}, "S": {"c", true},
}

func (v *vimState) visualKey(b vimBuffer, key string) vimBuffer {
	count, explicit := v.takeCount()
	switch v.pending {
	case "i", "a":
		kind := v.pending
		v.pending = ""
		return v.selectObject(b, kind, key)
	case "g":
		v.pending = ""
		if key != "g" {
			return b
		}
		key = "gg"
	}
	if visualOp, ok := vimVisualOps[key]; ok {
		return v.operateVisual(b, visualOp.op, visualOp.linewise)
	}
	switch key {
	case "v", "V":
		mode := map[string]vimMode{"v": vimVisual, "V": vimVisualLine}[key]
		if v.mode == mode {
			mode = vimNormal
		}
		v.mode = mode
		return b
	case "o":
		anchor := v.anchor
		v.anchor = b.pos()
		b.moveTo(anchor)
		return b
	case "i", "a", "g":
		v.pending = key
		return b
	case "~":
		return v.toggleSelection(b)
	case "J":
		from, to := v.selection(b)
		v.mode = vimNormal
		b.row = from.row
		return v.join(b, max(2, to.row-from.row+1))
	}
	if m, ok := v.motion(b, key, count, explicit); ok {
		return v.moveCursor(b, key, m)
	}
	return b
}

// selectObject makes a text object the selection (viw, vap).
func (v *vimState) selectObject(b vimBuffer, kind, key string) vimBuffer {
	object, ok := b.textObject(kind, key)
	if !ok {
		return b
	}
	if object.linewise {
		v.mode = vimVisualLine
	}
	v.anchor = object.from
	b.moveTo(object.to)
	return b
}

func (v *vimState) operateVisual(b vimBuffer, op string, linewise bool) vimBuffer {
	from, to := v.selection(b)
	linewise = linewise || v.mode == vimVisualLine
	v.mode = vimNormal
	b.row, b.col = from.row, from.col
	return v.operate(b, op, vimMotion{to: to, linewise: linewise, inclusive: true})
}

func (v *vimState) toggleSelection(b vimBuffer) vimBuffer {
	from, to := v.selection(b)
	v.mode = vimNormal
	v.snapshot(b)
	for row := from.row; row <= to.row; row++ {
		line := b.lines[row]
		lo, hi := 0, len(line)-1
		if row == from.row {
			lo = from.col
		}
		if row == to.row {
			hi = min(hi, to.col)
		}
		for index := lo; index <= hi; index++ {
			line[index] = flipCase(line[index])
		}
	}
	b.moveTo(from)
	return b
}

func flipCase(r rune) rune {
	if unicode.IsUpper(r) {
		return unicode.ToLower(r)
	}
	return unicode.ToUpper(r)
}
