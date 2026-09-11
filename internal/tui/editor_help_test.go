package tui

import (
	"strings"
	"testing"
)

// ? is ordinary text in a Markdown editor; Ctrl+G is the help chord.
func TestQuestionMarkIsTextInTheEditorsWithVimOff(t *testing.T) {
	model := New()
	model.width, model.height = 100, 36
	model = vimPress(t, model, "n", "?")
	if model.helping {
		t.Fatal("? must be typed, not open help")
	}
	if !model.editing || !strings.Contains(model.editBody.Value(), "?") {
		t.Fatalf("? should land in the body: %q", model.editBody.Value())
	}
	model = vimPress(t, model, "ctrl+g")
	if !model.helping {
		t.Fatal("Ctrl+G should open help from the editor")
	}
	model = vimPress(t, model, "esc")
	if model.helping || !model.editing {
		t.Fatal("closing help should return to the editor")
	}
	if !strings.Contains(model.View().Content, "Ctrl+G help") {
		t.Fatal("the editor footer should name the help chord")
	}
}

func TestQuestionMarkOpensHelpOnlyFromVimNormal(t *testing.T) {
	model := vimPress(t, vimModel(t), "n", "i", "?")
	if model.helping || !strings.Contains(model.editBody.Value(), "?") {
		t.Fatalf("Insert mode should type ?: helping=%v body %q", model.helping, model.editBody.Value())
	}
	model = vimPress(t, model, "esc", "?")
	if !model.helping {
		t.Fatal("? in Normal mode should open help")
	}
	model = vimPress(t, model, "esc", "0", "r", "?")
	if model.helping {
		t.Fatal("r? should replace, not open help")
	}
	line := strings.Split(model.editBody.Value(), "\n")[model.editBody.Line()]
	if !strings.HasPrefix(line, "?") {
		t.Fatalf("r? should replace the first character: %q", line)
	}
	model = vimPress(t, model, "2", "?")
	if model.helping {
		t.Fatal("? after a pending count should not open help")
	}
	model = vimPress(t, model, "esc", ":", "?")
	if model.helping || !model.vim.commandL || model.vim.command != "?" {
		t.Fatalf("? on the command line is text: %q", model.vim.command)
	}
	model = vimPress(t, model, "esc", "i", "ctrl+g")
	if !model.helping {
		t.Fatal("Ctrl+G should open help from Insert mode")
	}
}

func TestQuestionMarkIsTextInThePrepTitle(t *testing.T) {
	model := vimPress(t, vimModel(t), "p", "?")
	if model.helping || !strings.Contains(model.planTitle.Value(), "?") {
		t.Fatalf("the prep title is a plain field: helping=%v title %q", model.helping, model.planTitle.Value())
	}
	model = vimPress(t, model, "ctrl+g")
	if !model.helping {
		t.Fatal("Ctrl+G should open help from the prep editor")
	}
}
