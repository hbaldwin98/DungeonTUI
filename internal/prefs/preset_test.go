package prefs

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestEveryPresetKeepsRequiredPanes(t *testing.T) {
	for _, preset := range Presets {
		for _, width := range []int{80, 100, 160} {
			tree, err := PresetTree(preset, width)
			if err != nil {
				t.Fatal(err)
			}
			required := []Pane{PaneNav, PaneList, PaneDetail}
			if preset == PresetRun {
				required = []Pane{PaneContext, PaneTranscript, PaneInput}
			}
			for _, pane := range required {
				if !FindVisibleLeaf(tree.Root, pane) {
					t.Fatalf("%s at %d hides required pane %s: %s", preset, width, pane, tree.Root.Describe())
				}
			}
		}
	}
	if _, err := PresetTree("cinema", 120); err == nil {
		t.Fatal("unknown presets are refused")
	}
}

func TestPresetsEmphasizeDifferentPanes(t *testing.T) {
	browse, _ := PresetTree(PresetBrowse, 140)
	prep, _ := PresetTree(PresetPrep, 140)
	review, _ := PresetTree(PresetReview, 140)
	if !(prep.Root.Ratio < browse.Root.Ratio && review.Root.Children[1].Ratio < prep.Root.Children[1].Ratio) {
		t.Fatalf("prep and review should widen detail: browse %s prep %s review %s", ratios(browse), ratios(prep), ratios(review))
	}
	for _, tree := range []SplitTree{browse, prep, review} {
		if tree.Root.Ratio < 0.2 || tree.Root.Children[1].Ratio < 0.2 {
			t.Fatalf("preset ratios must sit inside the range every drag clamps to: %s", ratios(tree))
		}
	}
	if false {
		t.Fatalf("prep and review should widen detail: browse %s prep %s review %s", ratios(browse), ratios(prep), ratios(review))
	}
}

func ratios(tree SplitTree) string {
	data, _ := json.Marshal([]float64{tree.Root.Ratio, tree.Root.Children[1].Ratio})
	return string(data)
}

func TestRunPresetDropsTheCampaignPaneOnlyWhenNarrow(t *testing.T) {
	narrow, _ := PresetTree(PresetRun, 80)
	wide, _ := PresetTree(PresetRun, 140)
	if FindVisibleLeaf(narrow.Root, PaneCampaign) {
		t.Fatal("80 columns cannot fit two readable upper panes")
	}
	if !FindVisibleLeaf(wide.Root, PaneCampaign) {
		t.Fatal("wide terminals keep the campaign pane")
	}
}

func TestApplyPresetIsReversibleAndRemembersOwnerRatios(t *testing.T) {
	layout := DefaultLayout()
	layout.Browser.Root.Ratio = 0.31 // the owner's hand-tuned nav width
	original := layout.Browser.Root.Describe()

	if err := layout.ApplyPreset(PresetPrep, 140); err != nil {
		t.Fatal(err)
	}
	if layout.Preset != "prep" || layout.Previous == nil || layout.Browser.Root.Ratio == 0.31 {
		t.Fatalf("prep should reshape and stash: %#v", layout)
	}
	layout.Browser.Root.Children[1].Ratio = 0.41 // owner tweaks while in prep
	if err := layout.ApplyPreset(PresetBrowse, 140); err != nil {
		t.Fatal(err)
	}
	if err := layout.ApplyPreset(PresetPrep, 140); err != nil {
		t.Fatal(err)
	}
	if layout.Browser.Root.Children[1].Ratio != 0.41 {
		t.Fatalf("re-entering prep should keep the owner's tweak, got %v", layout.Browser.Root.Children[1].Ratio)
	}
	if !layout.RestoreLayout() {
		t.Fatal("restore should succeed")
	}
	if layout.Browser.Root.Ratio != 0.31 || layout.Browser.Root.Describe() != original || layout.Preset != "" {
		t.Fatalf("restore should return the hand-built layout: %v %s", layout.Browser.Root.Ratio, layout.Browser.Root.Describe())
	}
	if layout.RestoreLayout() {
		t.Fatal("a second restore has nothing to undo")
	}
}

func TestApplyPresetOnlyTouchesTheTreeItOwns(t *testing.T) {
	layout := DefaultLayout()
	session := layout.Session.Root.Describe()
	_ = layout.ApplyPreset(PresetReview, 140)
	if layout.Session.Root.Describe() != session {
		t.Fatal("a browser preset must leave the session tree alone")
	}
	browser := layout.Browser.Root.Ratio
	_ = layout.ApplyPreset(PresetRun, 80)
	if layout.Browser.Root.Ratio != browser {
		t.Fatal("the run preset must leave the browser tree alone")
	}
}

func TestApplyPresetMovesFocusOffAHiddenPane(t *testing.T) {
	layout := DefaultLayout()
	layout.Focus = PaneCampaign
	_ = layout.ApplyPreset(PresetRun, 80)
	if layout.Focus != PaneInput {
		t.Fatalf("focus on a hidden pane should fall back to input, got %s", layout.Focus)
	}
	layout.Focus = PaneList
	_ = layout.ApplyPreset(PresetReview, 80)
	if layout.Focus != PaneList {
		t.Fatalf("focus on a still-visible pane stays, got %s", layout.Focus)
	}
}

func TestPresetStatePersistsAndOldFilesStillLoad(t *testing.T) {
	store := NewJSON(filepath.Join(t.TempDir(), "prefs.json"))
	layout := DefaultLayout()
	_ = layout.ApplyPreset(PresetPrep, 140)
	if err := store.Save(layout); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	loaded = loaded.Normalize()
	if loaded.Preset != "prep" || loaded.Previous == nil || loaded.Browser.Root.Ratio != layout.Browser.Root.Ratio {
		t.Fatalf("preset state should round-trip: %#v", loaded)
	}

	var legacy Layout
	if err := json.Unmarshal([]byte(`{"browser":{"root":{}},"session":{"root":{}}}`), &legacy); err != nil {
		t.Fatal(err)
	}
	legacy = legacy.Normalize()
	if legacy.Preset != "" || legacy.Previous != nil || !FindVisibleLeaf(legacy.Browser.Root, PaneDetail) {
		t.Fatalf("a file from before presets loads as the owner's own layout: %#v", legacy)
	}
}
