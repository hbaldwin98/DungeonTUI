package prefs

import "strings"

// Pane identifies a named workspace pane leaf.
type Pane string

const (
	PaneList       Pane = "list"
	PaneDetail     Pane = "detail"
	PaneCampaign   Pane = "campaign"
	PaneContext    Pane = "context"
	PaneTranscript Pane = "transcript"
	PaneInput      Pane = "input"
)

// Axis is the split direction for an interior node.
type Axis string

const (
	AxisVertical   Axis = "vertical"   // left | right
	AxisHorizontal Axis = "horizontal" // top / bottom
)

// Node is one node in a binary split tree.
type Node struct {
	Type     string  `json:"type"` // "leaf" or "split"
	Pane     Pane    `json:"pane,omitempty"`
	Visible  *bool   `json:"visible,omitempty"`
	Axis     Axis    `json:"axis,omitempty"`
	Ratio    float64 `json:"ratio,omitempty"` // first child fraction in (0,1)
	Children []Node  `json:"children,omitempty"`
}

// IsLeaf reports whether the node is a pane leaf.
func (n Node) IsLeaf() bool { return n.Type == "leaf" || n.Type == "" && n.Pane != "" }

// IsVisible returns whether a leaf should render. Missing means visible.
func (n Node) IsVisible() bool {
	if n.Visible == nil {
		return true
	}
	return *n.Visible
}

// SetVisible mutates leaf visibility.
func (n *Node) SetVisible(visible bool) {
	value := visible
	n.Visible = &value
}

// Layout captures personal pane geometry as named split trees.
type Layout struct {
	// Legacy fields retained for migration from earlier preferences files.
	PaneLayout      int `json:"pane_layout,omitempty"`
	PaneSplit       int `json:"pane_split,omitempty"`
	HorizontalSplit int `json:"horizontal_split,omitempty"`

	Browser SplitTree `json:"browser"`
	Session SplitTree `json:"session"`
	Focus   Pane      `json:"focus,omitempty"`
}

// SplitTree is a rooted pane graph for one workspace mode.
type SplitTree struct {
	Root Node `json:"root"`
}

// Bool returns a *bool for JSON visibility fields.
func Bool(value bool) *bool { return &value }

// DefaultBrowser returns list|detail with a vertical split.
func DefaultBrowser() SplitTree {
	return SplitTree{Root: Node{
		Type:  "split",
		Axis:  AxisVertical,
		Ratio: 0.34,
		Children: []Node{
			{Type: "leaf", Pane: PaneList, Visible: Bool(true)},
			{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
		},
	}}
}

// DefaultSession returns the table-oriented session composition.
func DefaultSession() SplitTree {
	return SplitTree{Root: Node{
		Type:  "split",
		Axis:  AxisHorizontal,
		Ratio: 0.52,
		Children: []Node{
			{
				Type:  "split",
				Axis:  AxisVertical,
				Ratio: 0.34,
				Children: []Node{
					{Type: "leaf", Pane: PaneCampaign, Visible: Bool(true)},
					{Type: "leaf", Pane: PaneContext, Visible: Bool(true)},
				},
			},
			{
				Type:  "split",
				Axis:  AxisHorizontal,
				Ratio: 0.55,
				Children: []Node{
					{Type: "leaf", Pane: PaneTranscript, Visible: Bool(true)},
					{Type: "leaf", Pane: PaneInput, Visible: Bool(true)},
				},
			},
		},
	}}
}

// DefaultLayout returns portable defaults for both browser and session.
func DefaultLayout() Layout {
	return Layout{
		Browser: DefaultBrowser(),
		Session: DefaultSession(),
		Focus:   PaneInput,
	}
}

// Normalize fills defaults and migrates legacy integer fields.
func (l Layout) Normalize() Layout {
	out := l
	if out.Browser.Root.Type == "" && out.Browser.Root.Pane == "" {
		out.Browser = DefaultBrowser()
	}
	if out.Session.Root.Type == "" && out.Session.Root.Pane == "" {
		out.Session = DefaultSession()
		out.migrateLegacySession()
	} else if out.PaneLayout != 0 || out.PaneSplit != 0 || out.HorizontalSplit != 0 {
		out.migrateLegacySession()
	}
	out.Browser.Root = normalizeNode(out.Browser.Root)
	out.Session.Root = normalizeNode(out.Session.Root)
	if out.Focus == "" {
		out.Focus = PaneInput
	}
	return out
}

func (l *Layout) migrateLegacySession() {
	if l.Session.Root.Type == "" && l.Session.Root.Pane == "" {
		l.Session = DefaultSession()
	}
	switch l.PaneLayout {
	case 1:
		setLeafVisible(&l.Session.Root, PaneCampaign, false)
		setLeafVisible(&l.Session.Root, PaneContext, true)
	case 2:
		setLeafVisible(&l.Session.Root, PaneCampaign, true)
		setLeafVisible(&l.Session.Root, PaneContext, false)
	}
	if l.PaneSplit > 0 {
		setSplitRatio(&l.Session.Root, AxisVertical, ratioFromWidth(l.PaneSplit, 100))
		setSplitRatio(&l.Browser.Root, AxisVertical, ratioFromWidth(l.PaneSplit, 100))
	}
	if l.HorizontalSplit > 0 {
		setSplitRatio(&l.Session.Root, AxisHorizontal, ratioFromHeight(l.HorizontalSplit, 30))
	}
}

func normalizeNode(node Node) Node {
	if node.IsLeaf() {
		node.Type = "leaf"
		if node.Visible == nil {
			node.SetVisible(true)
		}
		return node
	}
	node.Type = "split"
	if node.Axis == "" {
		node.Axis = AxisVertical
	}
	if node.Ratio <= 0 || node.Ratio >= 1 {
		node.Ratio = 0.5
	}
	if len(node.Children) < 2 {
		node.Children = []Node{
			{Type: "leaf", Pane: PaneList, Visible: Bool(true)},
			{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
		}
	}
	for index := range node.Children {
		node.Children[index] = normalizeNode(node.Children[index])
	}
	return node
}

func setLeafVisible(node *Node, pane Pane, visible bool) {
	if node.IsLeaf() {
		if node.Pane == pane {
			node.SetVisible(visible)
		}
		return
	}
	for index := range node.Children {
		setLeafVisible(&node.Children[index], pane, visible)
	}
}

func setSplitRatio(node *Node, axis Axis, ratio float64) {
	if node == nil || node.IsLeaf() {
		return
	}
	if node.Axis == axis {
		node.Ratio = clampRatio(ratio)
		return
	}
	for index := range node.Children {
		setSplitRatio(&node.Children[index], axis, ratio)
	}
}

func ratioFromWidth(split, total int) float64 {
	if total <= 0 {
		return 0.34
	}
	return clampRatio(float64(split) / float64(total))
}

func ratioFromHeight(split, total int) float64 {
	if total <= 0 {
		return 0.52
	}
	return clampRatio(float64(split) / float64(total))
}

func clampRatio(value float64) float64 {
	if value < 0.2 {
		return 0.2
	}
	if value > 0.8 {
		return 0.8
	}
	return value
}

// CycleSessionUpperVisibility walks campaign/context visibility presets.
func (l *Layout) CycleSessionUpperVisibility() {
	campaign := leafVisible(l.Session.Root, PaneCampaign)
	context := leafVisible(l.Session.Root, PaneContext)
	switch {
	case campaign && context:
		setLeafVisible(&l.Session.Root, PaneCampaign, false)
		setLeafVisible(&l.Session.Root, PaneContext, true)
	case !campaign && context:
		setLeafVisible(&l.Session.Root, PaneCampaign, true)
		setLeafVisible(&l.Session.Root, PaneContext, false)
	default:
		setLeafVisible(&l.Session.Root, PaneCampaign, true)
		setLeafVisible(&l.Session.Root, PaneContext, true)
	}
}

// SessionLeafVisible reports whether a session leaf is currently shown.
func (l Layout) SessionLeafVisible(pane Pane) bool {
	return leafVisible(l.Session.Root, pane)
}

// SetVerticalSplitRatio updates browser and session vertical gutters together.
func (l *Layout) SetVerticalSplitRatio(ratio float64) {
	setSplitRatio(&l.Browser.Root, AxisVertical, ratio)
	setSplitRatio(&l.Session.Root, AxisVertical, ratio)
}

// SetSessionUpperRatio updates the top/bottom split above the transcript stack.
func (l *Layout) SetSessionUpperRatio(ratio float64) {
	if l.Session.Root.Type == "split" && l.Session.Root.Axis == AxisHorizontal {
		l.Session.Root.Ratio = clampRatio(ratio)
		return
	}
	setSplitRatio(&l.Session.Root, AxisHorizontal, ratio)
}

// VerticalRatio returns the active left/right split fraction.
func (l Layout) VerticalRatio() float64 {
	if l.Browser.Root.Type == "split" && l.Browser.Root.Axis == AxisVertical {
		return clampRatio(l.Browser.Root.Ratio)
	}
	return 0.34
}

// SessionUpperRatio returns the fraction of non-input height given to upper panes.
func (l Layout) SessionUpperRatio() float64 {
	if l.Session.Root.Type == "split" && l.Session.Root.Axis == AxisHorizontal {
		return clampRatio(l.Session.Root.Ratio)
	}
	return 0.52
}

func leafVisible(node Node, pane Pane) bool {
	if node.IsLeaf() {
		if node.Pane == pane {
			return node.IsVisible()
		}
		return false
	}
	for _, child := range node.Children {
		if leafVisible(child, pane) {
			return true
		}
	}
	return false
}

// FindLeaf reports whether a pane exists and is visible in the tree.
func FindLeaf(node Node, pane Pane) (Node, bool) {
	if node.IsLeaf() {
		if node.Pane == pane {
			return node, true
		}
		return Node{}, false
	}
	for _, child := range node.Children {
		if found, ok := FindLeaf(child, pane); ok {
			return found, true
		}
	}
	return Node{}, false
}

// VisibleLeaves returns named panes that should render.
func VisibleLeaves(node Node) []Pane {
	if node.IsLeaf() {
		if node.IsVisible() {
			return []Pane{node.Pane}
		}
		return nil
	}
	var panes []Pane
	for _, child := range node.Children {
		panes = append(panes, VisibleLeaves(child)...)
	}
	return panes
}

// Describe returns a short debug label for tests.
func (n Node) Describe() string {
	if n.IsLeaf() {
		vis := "on"
		if !n.IsVisible() {
			vis = "off"
		}
		return string(n.Pane) + ":" + vis
	}
	parts := make([]string, 0, len(n.Children))
	for _, child := range n.Children {
		parts = append(parts, child.Describe())
	}
	return string(n.Axis) + "{" + strings.Join(parts, "|") + "}"
}
