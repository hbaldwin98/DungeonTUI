package prefs

import "testing"

func TestDefaultLayoutHasNamedSessionPanes(t *testing.T) {
	layout := DefaultLayout()
	leaves := VisibleLeaves(layout.Session.Root)
	want := map[Pane]bool{PaneCampaign: true, PaneContext: true, PaneTranscript: true, PaneInput: true}
	if len(leaves) != 4 {
		t.Fatalf("leaves=%v", leaves)
	}
	for _, pane := range leaves {
		if !want[pane] {
			t.Fatalf("unexpected leaf %q", pane)
		}
	}
}

func TestDefaultBrowserUsesCampaignTree(t *testing.T) {
	leaves := VisibleLeaves(DefaultBrowser().Root)
	want := map[Pane]bool{PaneNav: true, PaneList: true, PaneDetail: true}
	if len(leaves) != 3 {
		t.Fatalf("expected nav|list|detail, got %v", leaves)
	}
	for _, pane := range leaves {
		if !want[pane] {
			t.Fatalf("unexpected leaf %q", pane)
		}
	}
	for _, pane := range BrowserTypePanes {
		if FindVisibleLeaf(DefaultBrowser().Root, pane) {
			t.Fatalf("default browser should not tile type pane %q", pane)
		}
	}
}

func TestNormalizeUpgradesClassicListDetailBrowser(t *testing.T) {
	classic := Layout{Browser: SplitTree{Root: Node{
		Type: "split", Axis: AxisVertical, Ratio: 0.34,
		Children: []Node{
			{Type: "leaf", Pane: PaneList, Visible: Bool(true)},
			{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
		},
	}}}.Normalize()
	leaves := VisibleLeaves(classic.Browser.Root)
	if !FindVisibleLeaf(classic.Browser.Root, PaneNav) || len(leaves) != 3 {
		t.Fatalf("classic list|detail should upgrade to campaign tree, got %v", leaves)
	}
}

func TestNormalizeUpgradesDenseTypedBrowser(t *testing.T) {
	dense := Layout{Browser: SplitTree{Root: Node{
		Type: "split", Axis: AxisVertical, Ratio: 0.55,
		Children: []Node{
			{
				Type: "split", Axis: AxisHorizontal, Ratio: 0.5,
				Children: []Node{
					balancedSplit(AxisVertical, PaneNPC, PaneLocation, PaneFaction),
					balancedSplit(AxisVertical, PaneThread, PaneItem, PaneNote),
				},
			},
			{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
		},
	}}}.Normalize()
	if !FindVisibleLeaf(dense.Browser.Root, PaneNav) {
		t.Fatalf("dense typed browser should upgrade to campaign tree, got %v", VisibleLeaves(dense.Browser.Root))
	}
}

func TestAddBrowserTypePaneShowsHiddenSection(t *testing.T) {
	layout := DefaultLayout()
	// Insert a typed pane into a dense-style tree for this helper API.
	layout.Browser = SplitTree{Root: Node{
		Type: "split", Axis: AxisVertical, Ratio: 0.55,
		Children: []Node{
			balancedSplit(AxisVertical, PaneNPC, PaneLocation, PaneFaction),
			{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
		},
	}}
	setLeafVisible(&layout.Browser.Root, PaneFaction, false)
	if leafVisible(layout.Browser.Root, PaneFaction) {
		t.Fatal("setup failed")
	}
	if !layout.AddBrowserTypePane(PaneFaction) {
		t.Fatal("expected add to succeed")
	}
	if !leafVisible(layout.Browser.Root, PaneFaction) {
		t.Fatal("faction pane should be visible after add")
	}
}

func TestCloseBrowserTypePaneHidesSection(t *testing.T) {
	layout := DefaultLayout()
	layout.Browser = SplitTree{Root: Node{
		Type: "split", Axis: AxisVertical, Ratio: 0.55,
		Children: []Node{
			balancedSplit(AxisVertical, PaneNPC, PaneLocation, PaneFaction, PaneThread, PaneItem, PaneNote),
			{Type: "leaf", Pane: PaneDetail, Visible: Bool(true)},
		},
	}}
	if !layout.CloseBrowserTypePane(PaneFaction) {
		t.Fatal("expected to close faction pane")
	}
	if leafVisible(layout.Browser.Root, PaneFaction) {
		t.Fatal("faction should be hidden")
	}
	// Close down to one type pane — further closes must fail.
	for _, pane := range []Pane{PaneLocation, PaneThread, PaneItem, PaneNote} {
		layout.CloseBrowserTypePane(pane)
	}
	if layout.CloseBrowserTypePane(PaneNPC) {
		t.Fatal("should not close the last type pane")
	}
	if !leafVisible(layout.Browser.Root, PaneNPC) {
		t.Fatal("last type pane must remain")
	}
	if layout.CloseBrowserTypePane(PaneDetail) {
		t.Fatal("detail pane must not close")
	}
	if layout.CloseBrowserTypePane(PaneNav) {
		t.Fatal("nav pane must not close")
	}
}

func TestCycleSessionUpperVisibility(t *testing.T) {
	layout := DefaultLayout()
	layout.CycleSessionUpperVisibility()
	if leafVisible(layout.Session.Root, PaneCampaign) || !leafVisible(layout.Session.Root, PaneContext) {
		t.Fatalf("expected context-only, got %s", layout.Session.Root.Describe())
	}
	layout.CycleSessionUpperVisibility()
	if !leafVisible(layout.Session.Root, PaneCampaign) || leafVisible(layout.Session.Root, PaneContext) {
		t.Fatalf("expected campaign-only, got %s", layout.Session.Root.Describe())
	}
	layout.CycleSessionUpperVisibility()
	if !leafVisible(layout.Session.Root, PaneCampaign) || !leafVisible(layout.Session.Root, PaneContext) {
		t.Fatalf("expected both, got %s", layout.Session.Root.Describe())
	}
}

func TestNormalizeMigratesLegacyIntegers(t *testing.T) {
	layout := Layout{PaneLayout: 1, PaneSplit: 40, HorizontalSplit: 12}.Normalize()
	if leafVisible(layout.Session.Root, PaneCampaign) {
		t.Fatal("legacy pane_layout=1 should hide campaign")
	}
	if !leafVisible(layout.Session.Root, PaneContext) {
		t.Fatal("legacy pane_layout=1 should show context")
	}
	root := layout.Session.Root
	if root.Children[0].Ratio < 0.2 || root.Children[0].Ratio > 0.8 {
		t.Fatalf("unexpected upper vertical ratio %v", root.Children[0].Ratio)
	}
}

func TestLayoutJSONRoundTripPreservesTree(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/preferences.json"
	store := NewJSON(path)
	layout := DefaultLayout()
	layout.CycleSessionUpperVisibility()
	layout.Session.Root.Children[0].Ratio = 0.42
	if err := store.Save(layout); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if leafVisible(got.Session.Root, PaneCampaign) {
		t.Fatal("visibility should persist")
	}
	if got.Session.Root.Children[0].Ratio != 0.42 {
		t.Fatalf("ratio=%v", got.Session.Root.Children[0].Ratio)
	}
	if got.PaneSplit != 0 || got.PaneLayout != 0 {
		t.Fatalf("legacy fields should be cleared on save: %#v", got)
	}
}
