package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

// ansiEscape matches CSI / OSC sequences so screencaps stay readable for agents.
var ansiEscape = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\))`)

// Harness drives the Bubble Tea model without a TTY so tests and agents can
// exercise keyboard, mouse, layout, and capture full frames as screencaps.
type Harness struct {
	Model Model
}

// NewHarness starts from the demo workspace at the given terminal size.
func NewHarness(width, height int) *Harness {
	model := New()
	model.width = width
	model.height = height
	return &Harness{Model: model}
}

// Resize sends a WindowSizeMsg and returns the rendered frame.
func (h *Harness) Resize(width, height int) Frame {
	return h.Send(tea.WindowSizeMsg{Width: width, Height: height})
}

// Key sends a key press identified by bubbletea's Key.String() form
// (for example "s", "G", "ctrl+e", "enter", "shift+enter"). A name that is
// not a named key or chord, such as "ZZ" or "gg", presses each rune in turn.
func (h *Harness) Key(name string) Frame {
	var frame Frame
	for _, key := range parseKeys(name) {
		frame = h.Send(key)
	}
	return frame
}

// Type injects plain runes into the focused editor/input by sending KeyPressMsg
// values. Control sequences should use Key instead.
func (h *Harness) Type(text string) Frame {
	var frame Frame
	for _, r := range text {
		frame = h.Send(runeKey(r))
	}
	return frame
}

// Click sends a left mouse click at cell coordinates.
func (h *Harness) Click(x, y int) Frame {
	return h.Send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

// DragLeft performs click → motion → release for gutter resizing.
func (h *Harness) DragLeft(fromX, fromY, toX, toY int) Frame {
	h.Send(tea.MouseClickMsg{X: fromX, Y: fromY, Button: tea.MouseLeft})
	h.Send(tea.MouseMotionMsg{X: toX, Y: toY, Button: tea.MouseLeft})
	return h.Send(tea.MouseReleaseMsg{X: toX, Y: toY, Button: tea.MouseLeft})
}

// WheelUp / WheelDown send mouse-wheel events.
func (h *Harness) WheelUp() Frame {
	return h.Send(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
}

func (h *Harness) WheelDown() Frame {
	return h.Send(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
}

// Send applies any Bubble Tea message and returns the current frame.
func (h *Harness) Send(msg tea.Msg) Frame {
	updated, _ := h.Model.Update(msg)
	h.Model = updated.(Model)
	return h.Frame()
}

// Frame captures the current View as structured screen content.
func (h *Harness) Frame() Frame {
	view := h.Model.View()
	content := view.Content
	plain := stripANSI(content)
	lines := strings.Split(plain, "\n")
	widths := make([]int, len(lines))
	for index, line := range lines {
		widths[index] = lipgloss.Width(line)
	}
	return Frame{
		Width:       h.Model.width,
		Height:      h.Model.height,
		Raw:         content,
		Plain:       plain,
		Lines:       lines,
		LineWidths:  widths,
		MouseMode:   view.MouseMode,
		WindowTitle: view.WindowTitle,
	}
}

// Screencap writes plain and ANSI captures under dir using the given stem.
// It creates dir when needed and returns the plain-text path.
func (h *Harness) Screencap(dir, stem string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	frame := h.Frame()
	plainPath := filepath.Join(dir, stem+".screen.txt")
	ansiPath := filepath.Join(dir, stem+".ansi.txt")
	metaPath := filepath.Join(dir, stem+".meta.txt")
	if err := os.WriteFile(plainPath, []byte(frame.Plain+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(ansiPath, []byte(frame.Raw+"\n"), 0o644); err != nil {
		return "", err
	}
	meta := fmt.Sprintf("width=%d\nheight=%d\nrows=%d\nfill_ok=%t\nmouse_mode=%v\ntitle=%s\n",
		frame.Width, frame.Height, len(frame.Lines), frame.FillsTerminal(), frame.MouseMode, frame.WindowTitle)
	if err := os.WriteFile(metaPath, []byte(meta), 0o644); err != nil {
		return "", err
	}
	return plainPath, nil
}

// SetSessionDraft replaces the active session input buffer. No-op outside a session.
func (h *Harness) SetSessionDraft(text string) {
	if h.Model.session == nil {
		return
	}
	h.Model.sessionInput.SetValue(text)
	h.Model.refreshSuggestions()
	if !h.Model.reviewPinned {
		h.Model.review = h.Model.resolveReference(text)
	}
}

// SeedTranscript appends many transcript lines for scroll/regression scenarios.
func (h *Harness) SeedTranscript(count int, prefix string) {
	if h.Model.session == nil {
		return
	}
	if prefix == "" {
		prefix = "scroll event"
	}
	for index := 0; index < count; index++ {
		h.Model.session.Entries = append(h.Model.session.Entries, domain.TranscriptEntry{
			ID:        fmt.Sprintf("seed-%d", index),
			Text:      fmt.Sprintf("%s %d", prefix, index),
			CreatedAt: time.Unix(int64(index+1), 0),
		})
	}
	h.Model.refreshTranscriptViewport()
	h.Model.configureTranscriptViewport()
}

// Frame is one rendered terminal screen.
type Frame struct {
	Width       int
	Height      int
	Raw         string
	Plain       string
	Lines       []string
	LineWidths  []int
	MouseMode   tea.MouseMode
	WindowTitle string
}

// FillsTerminal reports whether every row matches the harness size.
func (f Frame) FillsTerminal() bool {
	if len(f.Lines) != f.Height {
		return false
	}
	for _, width := range f.LineWidths {
		if width != f.Width {
			return false
		}
	}
	return true
}

// Contains reports whether the plain screencap includes text.
func (f Frame) Contains(text string) bool {
	return strings.Contains(f.Plain, text)
}

// FillErrors lists dimension mismatches for assertions.
func (f Frame) FillErrors() []string {
	var errors []string
	if len(f.Lines) != f.Height {
		errors = append(errors, fmt.Sprintf("row count %d != height %d", len(f.Lines), f.Height))
	}
	for index, width := range f.LineWidths {
		if width != f.Width {
			errors = append(errors, fmt.Sprintf("row %d width %d != %d", index, width, f.Width))
			if len(errors) >= 8 {
				break
			}
		}
	}
	return errors
}

func stripANSI(input string) string {
	return ansiEscape.ReplaceAllString(input, "")
}

// namedKeys maps harness key names to the keys a real terminal sends. None
// carry Text: Key.String() prefers Text, so a named key with Text would
// report the wrong name to the model.
var namedKeys = map[string]tea.Key{
	"enter":       {Code: tea.KeyEnter},
	"return":      {Code: tea.KeyEnter},
	"ctrl+enter":  {Code: tea.KeyEnter, Mod: tea.ModCtrl},
	"shift+enter": {Code: tea.KeyEnter, Mod: tea.ModShift},
	"esc":         {Code: tea.KeyEscape},
	"escape":      {Code: tea.KeyEscape},
	"tab":         {Code: tea.KeyTab},
	"shift+tab":   {Code: tea.KeyTab, Mod: tea.ModShift},
	"backspace":   {Code: tea.KeyBackspace},
	"up":          {Code: tea.KeyUp},
	"down":        {Code: tea.KeyDown},
	"left":        {Code: tea.KeyLeft},
	"right":       {Code: tea.KeyRight},
	"alt+left":    {Code: tea.KeyLeft, Mod: tea.ModAlt},
	"alt+right":   {Code: tea.KeyRight, Mod: tea.ModAlt},
	"ctrl+up":     {Code: tea.KeyUp, Mod: tea.ModCtrl},
	"ctrl+down":   {Code: tea.KeyDown, Mod: tea.ModCtrl},
	"pgup":        {Code: tea.KeyPgUp},
	"pageup":      {Code: tea.KeyPgUp},
	"pgdown":      {Code: tea.KeyPgDown},
	"pagedown":    {Code: tea.KeyPgDown},
	"home":        {Code: tea.KeyHome},
	"end":         {Code: tea.KeyEnd},
}

// parseKey resolves one harness key name. Named keys and chords match in any
// case; a single rune keeps its case, so "G" and "g" are different keys.
func parseKey(name string) tea.KeyPressMsg {
	return parseKeys(name)[0]
}

// parseKeys resolves a harness key name into the presses a terminal sends:
// one for a named key, chord, or rune, and one per rune for anything else.
func parseKeys(name string) []tea.KeyPressMsg {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	if key, ok := namedKeys[lower]; ok {
		return []tea.KeyPressMsg{tea.KeyPressMsg(key)}
	}
	if runes := []rune(lower); strings.HasPrefix(lower, "ctrl+") && len(runes) == 6 {
		// Control chords produce no text in a real terminal.
		return []tea.KeyPressMsg{tea.KeyPressMsg(tea.Key{Code: runes[5], Mod: tea.ModCtrl})}
	}
	if name == "" {
		name = "/"
	}
	var keys []tea.KeyPressMsg
	for _, r := range name {
		keys = append(keys, runeKey(r))
	}
	return keys
}

// runeKey is a printable key as the terminal decoder reports it: a capital
// letter arrives as its lower-case code with Shift, carrying the capital as
// ShiftedCode and Text.
func runeKey(r rune) tea.KeyPressMsg {
	key := tea.Key{Code: r, Text: string(r)}
	if unicode.IsUpper(r) {
		key.Code, key.ShiftedCode, key.Mod = unicode.ToLower(r), r, tea.ModShift
	}
	return tea.KeyPressMsg(key)
}

// Scenario runs a scripted sequence used by tests and the harness CLI.
type Scenario struct {
	Name   string
	Width  int
	Height int
	Steps  []ScenarioStep
}

// ScenarioStep is one harness action.
type ScenarioStep struct {
	Op       string
	Text     string
	X, Y     int
	ToX, ToY int
	Stem     string
}

// RunScenario executes steps and writes screencaps into dir when Stem is set.
func RunScenario(dir string, scenario Scenario) (*Harness, []string, error) {
	width, height := scenario.Width, scenario.Height
	if width == 0 {
		width = 100
	}
	if height == 0 {
		height = 30
	}
	h := NewHarness(width, height)
	var caps []string
	for _, step := range scenario.Steps {
		switch strings.ToLower(step.Op) {
		case "resize":
			h.Resize(step.X, step.Y)
		case "key":
			h.Key(step.Text)
		case "type":
			h.Type(step.Text)
		case "click":
			h.Click(step.X, step.Y)
		case "drag":
			h.DragLeft(step.X, step.Y, step.ToX, step.ToY)
		case "wheelup":
			h.WheelUp()
		case "wheeldown":
			h.WheelDown()
		case "screencap", "cap":
			stem := step.Stem
			if stem == "" {
				stem = sanitizeStem(step.Text)
			}
			if stem == "" {
				stem = "frame"
			}
			path, err := h.Screencap(dir, stem)
			if err != nil {
				return h, caps, err
			}
			caps = append(caps, path)
		default:
			return h, caps, fmt.Errorf("unknown scenario op %q", step.Op)
		}
	}
	return h, caps, nil
}

func sanitizeStem(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else if r == ' ' {
			builder.WriteByte('-')
		}
	}
	return builder.String()
}
