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
