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

func TestDefaultBrowserUsesTypedSectionPanes(t *testing.T) {
	leaves := VisibleLeaves(DefaultBrowser().Root)
	want := map[Pane]bool{
		PaneNPC: true, PaneLocation: true, PaneFaction: true,
		PaneThread: true, PaneItem: true, PaneNote: true,
		PaneDetail: true,
	}
	if len(leaves) != 7 {
		t.Fatalf("expected 6 type sections + detail, got %v", leaves)
	}
	for _, pane := range leaves {
		if !want[pane] {
			t.Fatalf("unexpected leaf %q", pane)
		}
	}
	if FindVisibleLeaf(DefaultBrowser().Root, PaneList) {
		t.Fatal("default browser should not use the sparse all-records list pane")
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
	if len(leaves) < 5 {
		t.Fatalf("classic list|detail should upgrade to typed sections, got %v", leaves)
	}
	if FindVisibleLeaf(classic.Browser.Root, PaneList) {
		t.Fatal("upgraded browser should drop the all-records list leaf")
	}
}

func TestAddBrowserTypePaneShowsHiddenSection(t *testing.T) {
	layout := DefaultLayout()
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
