package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

func commandKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func TestCommandPaletteOpensSearchesExecutesAndRestoresFocus(t *testing.T) {
	model := New()
	model.width, model.height = 100, 30
	model.layout.Focus = prefs.PaneDetail

	updated, _ := model.Update(commandKey(':'))
	palette := updated.(Model)
	if !palette.commands.Open {
		t.Fatal("expected command palette to open")
	}
	if palette.layout.Focus != prefs.PaneDetail {
		t.Fatalf("opening palette changed focus to %q", palette.layout.Focus)
	}
	if content := palette.View().Content; !strings.Contains(content, "COMMAND PALETTE") || !strings.Contains(content, "Create entity") {
		t.Fatalf("palette did not render over browser:\n%s", content)
	}

	palette.commands.Query = "new wiki"
	results := palette.filteredPaletteCommands()
	if len(results) == 0 || results[0].ID != "entity.new" {
		t.Fatalf("query result = %#v, want entity.new first", results)
	}
	updated, _ = palette.updateCommandPalette(tea.KeyPressMsg{Code: tea.KeyEnter})
	created := updated.(Model)
	if !created.editing || created.commands.Open {
		t.Fatal("executing Create entity did not open the existing editor")
	}

	model.helping = true
	updated, _ = model.Update(commandKey(':'))
	palette = updated.(Model)
	updated, _ = palette.updateCommandPalette(tea.KeyPressMsg{Code: tea.KeyEscape})
	cancelled := updated.(Model)
	if !cancelled.helping || cancelled.layout.Focus != prefs.PaneDetail {
		t.Fatal("cancel did not restore help and exact originating pane")
	}
}

func TestCommandPaletteLeavesColonForTextEntryAndLiveCapture(t *testing.T) {
	model := New()
	model.width, model.height = 100, 30

	updated, _ := model.openEditor(true)
	editor := updated.(Model)
	before := editor.editBody.Value()
	updated, _ = editor.Update(commandKey(':'))
	editor = updated.(Model)
	if editor.commands.Open || editor.editBody.Value() == before {
		t.Fatal("colon should remain editable Markdown, not open the palette")
	}

	model = New()
	updated, _ = model.startSession()
	live := updated.(Model)
	updated, _ = live.Update(commandKey(':'))
	live = updated.(Model)
	if live.commands.Open || !strings.Contains(live.sessionInput.Value(), ":") {
		t.Fatal("colon should remain live capture text")
	}
}

func TestCommandPaletteDisabledCommandStaysOpenWithReason(t *testing.T) {
	model := New()
	model.selectedID = ""
	model.selectedPlanID = ""
	model.commands = commandPalette{Open: true, Query: "edit selected entity"}

	updated, _ := model.executePaletteSelection()
	result := updated.(Model)
	if !result.commands.Open {
		t.Fatal("disabled command closed the palette")
	}
	if result.status != "Select a wiki entity" {
		t.Fatalf("status = %q", result.status)
	}
}

func TestCommandMatcherSupportsAliasesAndFuzzyInput(t *testing.T) {
	model := New()
	model.commands.Query = "5e path"
	results := model.filteredPaletteCommands()
	if len(results) == 0 || results[0].ID != "settings.open" {
		t.Fatalf("5e path result = %#v", results)
	}
	model.commands.Query = "swcmp"
	results = model.filteredPaletteCommands()
	if len(results) == 0 || results[0].ID != "library.open" {
		t.Fatalf("fuzzy switch-campaign result = %#v", results)
	}
}

func TestCommandPaletteInventoryHasAnExecutorInEveryContext(t *testing.T) {
	contexts := []Model{
		New(),
		func() Model { m := New(); m.picking = true; return m }(),
		func() Model { m := New(); m.preview = &previewBuf{}; return m }(),
		func() Model { m := New(); m.playingBack = true; return m }(),
		func() Model { m := New(); m.reconciling = true; return m }(),
	}
	executors := paletteCommandExecutors()
	for _, model := range contexts {
		for _, command := range model.paletteCommands() {
			if executors[command.ID] == nil {
				t.Errorf("context %q command %q has no executor", model.commandContext(), command.ID)
			}
		}
	}
}

func TestCommandPaletteKeyboardEditingAndNavigation(t *testing.T) {
	model := New()
	model.commands = commandPalette{Open: true}

	updated, _ := model.updateCommandPalette(commandKey('é'))
	model = updated.(Model)
	if model.commands.Query != "é" {
		t.Fatalf("query = %q, want Unicode input", model.commands.Query)
	}
	updated, _ = model.updateCommandPalette(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model = updated.(Model)
	if model.commands.Query != "" {
		t.Fatalf("backspace query = %q", model.commands.Query)
	}

	updated, _ = model.updateCommandPalette(tea.KeyPressMsg{Code: tea.KeyDown})
	model = updated.(Model)
	if model.commands.Selected != 1 {
		t.Fatalf("down selected = %d, want 1", model.commands.Selected)
	}
	updated, _ = model.updateCommandPalette(tea.KeyPressMsg{Code: tea.KeyUp})
	model = updated.(Model)
	if model.commands.Selected != 0 {
		t.Fatalf("up selected = %d, want 0", model.commands.Selected)
	}
}

func TestCommandPaletteMouseOutsideCancelsAndInsideExecutes(t *testing.T) {
	model := New()
	model.width, model.height = 100, 30
	model.helping = true
	updated, _ := model.openCommandPalette()
	model = updated.(Model)

	updated, _ = model.updateCommandPaletteClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0})
	cancelled := updated.(Model)
	if cancelled.commands.Open || !cancelled.helping {
		t.Fatal("outside click did not cancel and restore help")
	}

	updated, _ = cancelled.openCommandPalette()
	model = updated.(Model)
	left, _, _, _, rowY, _, _ := model.commandPaletteGeometry()
	updated, _ = model.updateCommandPaletteClick(tea.MouseClickMsg{Button: tea.MouseLeft, X: left + 1, Y: rowY})
	executed := updated.(Model)
	if executed.commands.Open || !executed.helping {
		t.Fatal("clicking first command did not execute keyboard reference")
	}
}
