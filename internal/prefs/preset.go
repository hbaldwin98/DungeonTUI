package prefs

import "fmt"

// Preset names a task-shaped emphasis of the one workspace. A preset only
// changes pane visibility and split ratios inside the existing named trees;
// it never adds panes or changes what a pane shows.
type Preset string

const (
	PresetBrowse Preset = "browse"
	PresetPrep   Preset = "prep"
	PresetRun    Preset = "run"
	PresetReview Preset = "review"
)

// Presets lists every preset in palette order.
var Presets = []Preset{PresetBrowse, PresetPrep, PresetRun, PresetReview}

// NarrowWidth is the terminal width below which presets give up secondary
// panes so the primary one keeps a readable measure.
const NarrowWidth = 100

// Label is the human name of a preset.
func (p Preset) Label() string {
	switch p {
	case PresetBrowse:
		return "Browse"
	case PresetPrep:
		return "Prep"
	case PresetRun:
		return "Run"
	case PresetReview:
		return "Review"
	}
	return string(p)
}

// Describe says what a preset emphasizes, for the palette and help.
func (p Preset) Describe() string {
	switch p {
	case PresetBrowse:
		return "balanced tree, list, and detail"
	case PresetPrep:
		return "wide detail for run sheets"
	case PresetRun:
		return "transcript-first live play"
	case PresetReview:
		return "widest detail for reading changes"
	}
	return ""
}

// session reports whether the preset shapes the live-session tree rather
// than the browser tree.
func (p Preset) session() bool { return p == PresetRun }

// PresetTree returns a preset's default tree for a terminal width. Browser
// presets keep the nav pane visible, because hiding it would switch the
// browser into a different navigation model rather than a different emphasis.
func PresetTree(p Preset, width int) (SplitTree, error) {
	narrow := width > 0 && width < NarrowWidth
	switch p {
	case PresetBrowse:
		tree := DefaultBrowser()
		if narrow {
			tree.Root.Ratio = 0.26
		}
		return tree, nil
	case PresetPrep:
		return browserTree(0.20, pick(narrow, 0.36, 0.30)), nil
	case PresetReview:
		// Nav stays at the 0.20 floor every split already clamps to; the
		// list gives up the width instead.
		return browserTree(0.20, pick(narrow, 0.30, 0.22)), nil
	case PresetRun:
		tree := DefaultSession()
		tree.Root.Ratio = pick(narrow, 0.42, 0.46)
		upper := &tree.Root.Children[0]
		upper.Ratio = 0.30
		if narrow {
			// Two upper panes at this width are each too thin to read; the
			// scene context is what live play needs.
			setLeafVisible(upper, PaneCampaign, false)
		}
		return tree, nil
	}
	return SplitTree{}, fmt.Errorf("unknown layout preset %q", p)
}

func pick(narrow bool, whenNarrow, whenWide float64) float64 {
	if narrow {
		return whenNarrow
	}
	return whenWide
}

func browserTree(navRatio, listRatio float64) SplitTree {
	tree := DefaultBrowser()
	tree.Root.Ratio = navRatio
	tree.Root.Children[1].Ratio = listRatio
	return tree
}

// ParsePreset accepts a preset name case-insensitively.
func ParsePreset(value string) (Preset, bool) {
	for _, preset := range Presets {
		if string(preset) == value || preset.Label() == value {
			return preset, true
		}
	}
	return "", false
}

// ApplyPreset switches the tree the preset owns and records enough to undo
// it. Leaving a preset remembers the owner's ratios under that preset, and
// re-entering it restores them when the tree still has the same shape, so a
// customized split survives a round trip. The first switch away from a
// hand-built layout stashes it for RestoreLayout.
func (l *Layout) ApplyPreset(p Preset, width int) error {
	tree, err := PresetTree(p, width)
	if err != nil {
		return err
	}
	if l.PresetTrees == nil {
		l.PresetTrees = map[Preset]SplitTree{}
	}
	if current, ok := ParsePreset(l.Preset); ok {
		l.PresetTrees[current] = l.presetOwnedTree(current)
	} else if l.Previous == nil {
		l.Previous = &PresetStash{Browser: l.Browser, Session: l.Session}
	}
	if remembered, ok := l.PresetTrees[p]; ok {
		copyRatios(&tree.Root, remembered.Root)
	}
	if p.session() {
		l.Session = tree
	} else {
		l.Browser = tree
	}
	l.Preset = string(p)
	l.ensureVisibleFocus(p)
	return nil
}

// RestoreLayout returns to the hand-built layout from before the first
// preset. It reports false when there is nothing to restore.
func (l *Layout) RestoreLayout() bool {
	if l.Previous == nil {
		return false
	}
	if current, ok := ParsePreset(l.Preset); ok {
		if l.PresetTrees == nil {
			l.PresetTrees = map[Preset]SplitTree{}
		}
		l.PresetTrees[current] = l.presetOwnedTree(current)
	}
	l.Browser = l.Previous.Browser
	l.Session = l.Previous.Session
	l.Previous = nil
	l.Preset = ""
	return true
}

func (l Layout) presetOwnedTree(p Preset) SplitTree {
	if p.session() {
		return l.Session
	}
	return l.Browser
}

// ensureVisibleFocus moves focus off a pane the preset just hid.
func (l *Layout) ensureVisibleFocus(p Preset) {
	root := l.Browser.Root
	fallback := PaneDetail
	if p.session() {
		root = l.Session.Root
		fallback = PaneInput
	}
	if _, inTree := FindLeaf(root, l.Focus); !inTree {
		return
	}
	if !leafVisible(root, l.Focus) {
		l.Focus = fallback
	}
}

// copyRatios carries split ratios from an owner-tuned tree onto a preset
// tree of the same shape. A shape mismatch copies nothing.
func copyRatios(target *Node, source Node) {
	if target.IsLeaf() || source.IsLeaf() {
		return
	}
	if target.Axis != source.Axis || len(target.Children) != len(source.Children) || !sameLeaves(*target, source) {
		return
	}
	if source.Ratio > 0 && source.Ratio < 1 {
		target.Ratio = clampRatio(source.Ratio)
	}
	for index := range target.Children {
		copyRatios(&target.Children[index], source.Children[index])
	}
}

func sameLeaves(a, b Node) bool {
	left, right := allLeaves(a), allLeaves(b)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func allLeaves(node Node) []Pane {
	if node.IsLeaf() {
		return []Pane{node.Pane}
	}
	var panes []Pane
	for _, child := range node.Children {
		panes = append(panes, allLeaves(child)...)
	}
	return panes
}
