package prefs

import "strings"

// Pane identifies a named workspace pane leaf.
type Pane string

const (
	PaneNav        Pane = "nav"
	PaneList       Pane = "list"
	PaneDetail     Pane = "detail"
	PaneNPC        Pane = "npc"
	PaneLocation   Pane = "location"
	PaneFaction    Pane = "faction"
	PaneItem       Pane = "item"
	PaneThread     Pane = "thread"
	PaneNote       Pane = "note"
	PaneCharacter  Pane = "character"
	PaneCreature   Pane = "creature"
	PaneEvent      Pane = "event"
	PaneCampaign   Pane = "campaign"
	PaneContext    Pane = "context"
	PaneTranscript Pane = "transcript"
	PaneInput      Pane = "input"
)

// BrowserTypePanes are the typed section leaves the campaign browser can show.
var BrowserTypePanes = []Pane{
	PaneNPC, PaneLocation, PaneFaction, PaneThread, PaneItem, PaneNote,
	PaneCharacter, PaneCreature, PaneEvent,
}

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

	// Last-opened campaign context for vault-style relaunch.
	ActiveWorldID    string `json:"active_world_id,omitempty"`
	ActiveCampaignID string `json:"active_campaign_id,omitempty"`

	// ImportHarness is an optional ingest cleanup: off or on.
	ImportHarness string `json:"import_harness,omitempty"`
}

// SplitTree is a rooted pane graph for one workspace mode.
type SplitTree struct {
	Root Node `json:"root"`
}

// Bool returns a *bool for JSON visibility fields.
func Bool(value bool) *bool { return &value }

// DefaultBrowser returns an IDE-style campaign tree: nav | list | detail.
func DefaultBrowser() SplitTree {
	return SplitTree{Root: Node{
		Type:  "split",
		Axis:  AxisVertical,
		Ratio: 0.24,
		Children: []Node{
			{Type: "leaf", Pane: PaneNav, Visible: Bool(true)},
			{
				Type:  "split",
				Axis:  AxisVertical,
				Ratio: 0.42,
				Children: []Node{
					{Type: "leaf", Pane: PaneList, Visible: Bool(true)},
					{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
				},
			},
		},
	}}
}

func balancedSplit(axis Axis, panes ...Pane) Node {
	if len(panes) == 0 {
		return Node{Type: "leaf", Pane: PaneList, Visible: Bool(true)}
	}
	if len(panes) == 1 {
		return Node{Type: "leaf", Pane: panes[0], Visible: Bool(true)}
	}
	mid := len(panes) / 2
	ratio := float64(mid) / float64(len(panes))
	return Node{
		Type:  "split",
		Axis:  axis,
		Ratio: clampRatio(ratio),
		Children: []Node{
			balancedSplit(axis, panes[:mid]...),
			balancedSplit(axis, panes[mid:]...),
		},
	}
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
	} else if isClassicListDetail(out.Browser.Root) || isDenseTypedBrowser(out.Browser.Root) {
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
	switch strings.ToLower(strings.TrimSpace(out.ImportHarness)) {
	case "", "off", "none":
		out.ImportHarness = ""
	case "on", "clean", "apply", "dump", "diagnose":
		out.ImportHarness = "on"
	default:
		out.ImportHarness = ""
	}
	return out
}

func isClassicListDetail(node Node) bool {
	leaves := VisibleLeaves(node)
	if len(leaves) != 2 {
		return false
	}
	hasList, hasDetail := false, false
	for _, pane := range leaves {
		switch pane {
		case PaneList:
			hasList = true
		case PaneDetail:
			hasDetail = true
		default:
			return false
		}
	}
	return hasList && hasDetail
}

// isDenseTypedBrowser detects the pre-tree multi-type-pane layout so prefs
// migrate to the campaign nav tree.
func isDenseTypedBrowser(node Node) bool {
	if FindVisibleLeaf(node, PaneNav) {
		return false
	}
	typed := 0
	for _, pane := range VisibleLeaves(node) {
		for _, candidate := range BrowserTypePanes {
			if pane == candidate {
				typed++
				break
			}
		}
	}
	return typed >= 3
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

// AddBrowserTypePane makes a typed section visible, inserting it when missing.
func (l *Layout) AddBrowserTypePane(pane Pane) bool {
	if pane == PaneDetail || pane == PaneList || pane == PaneNav {
		return false
	}
	if _, ok := FindLeaf(l.Browser.Root, pane); ok {
		setLeafVisible(&l.Browser.Root, pane, true)
		return true
	}
	detail, ok := FindLeaf(l.Browser.Root, PaneDetail)
	if !ok {
		return false
	}
	_ = detail
	// Attach the new leaf beside an existing type section under the left stack.
	insertBrowserLeaf(&l.Browser.Root, pane)
	return FindVisibleLeaf(l.Browser.Root, pane)
}

// CloseBrowserTypePane hides a typed section pane. Detail/nav/list cannot be closed.
// Refuses to hide the last remaining type section.
func (l *Layout) CloseBrowserTypePane(pane Pane) bool {
	if pane == PaneDetail || pane == PaneNav || pane == PaneList || pane == "" {
		return false
	}
	if !FindVisibleLeaf(l.Browser.Root, pane) {
		return false
	}
	visibleTypes := 0
	for _, candidate := range VisibleLeaves(l.Browser.Root) {
		if candidate == PaneDetail || candidate == PaneList || candidate == PaneNav {
			continue
		}
		visibleTypes++
	}
	if visibleTypes <= 1 {
		return false
	}
	setLeafVisible(&l.Browser.Root, pane, false)
	return !FindVisibleLeaf(l.Browser.Root, pane)
}

func insertBrowserLeaf(node *Node, pane Pane) {
	if node == nil {
		return
	}
	if node.IsLeaf() {
		if node.Pane == PaneDetail {
			return
		}
		*node = Node{
			Type:  "split",
			Axis:  AxisVertical,
			Ratio: 0.5,
			Children: []Node{
				{Type: "leaf", Pane: node.Pane, Visible: node.Visible},
				{Type: "leaf", Pane: pane, Visible: Bool(true)},
			},
		}
		return
	}
	if node.Axis == AxisVertical && len(node.Children) == 2 && node.Children[1].IsLeaf() && node.Children[1].Pane == PaneDetail {
		insertBrowserLeaf(&node.Children[0], pane)
		return
	}
	for index := range node.Children {
		if node.Children[index].IsLeaf() && node.Children[index].Pane == PaneDetail {
			continue
		}
		insertBrowserLeaf(&node.Children[index], pane)
		if FindVisibleLeaf(*node, pane) {
			return
		}
	}
}

// CycleBrowserTypeVisibility toggles denser section presets for the campaign browser.
func (l *Layout) CycleBrowserTypeVisibility() {
	core := []Pane{PaneNPC, PaneLocation, PaneItem, PaneThread}
	extended := []Pane{PaneNPC, PaneLocation, PaneFaction, PaneThread, PaneItem, PaneNote}
	visible := 0
	for _, pane := range BrowserTypePanes {
		if leafVisible(l.Browser.Root, pane) {
			visible++
		}
	}
	switch {
	case visible >= 6:
		for _, pane := range BrowserTypePanes {
			setLeafVisible(&l.Browser.Root, pane, false)
		}
		for _, pane := range core {
			if _, ok := FindLeaf(l.Browser.Root, pane); ok {
				setLeafVisible(&l.Browser.Root, pane, true)
			} else {
				l.AddBrowserTypePane(pane)
			}
		}
	default:
		for _, pane := range extended {
			if _, ok := FindLeaf(l.Browser.Root, pane); ok {
				setLeafVisible(&l.Browser.Root, pane, true)
			} else {
				l.AddBrowserTypePane(pane)
			}
		}
	}
}

// FindVisibleLeaf reports whether a pane exists and is currently visible.
func FindVisibleLeaf(node Node, pane Pane) bool {
	found, ok := FindLeaf(node, pane)
	return ok && found.IsVisible()
}

// CloseSessionUpperPane hides campaign or context when the sibling remains.
func (l *Layout) CloseSessionUpperPane(pane Pane) bool {
	if pane != PaneCampaign && pane != PaneContext {
		return false
	}
	other := PaneContext
	if pane == PaneContext {
		other = PaneCampaign
	}
	if !leafVisible(l.Session.Root, other) {
		return false
	}
	setLeafVisible(&l.Session.Root, pane, false)
	return !leafVisible(l.Session.Root, pane)
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
