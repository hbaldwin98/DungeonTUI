package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func TestSearchStartsCampaignScopedAndFactsOnly(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	model = updated.(Model)
	view := model.View().Content

	if model.searchScope != 0 {
		t.Fatalf("expected campaign search scope, got %v", model.searchScope)
	}
	if model.includeIdeas {
		t.Fatal("expected facts-only search")
	}
	if strings.Contains(view, "Church Investigator Arrives") {
		t.Fatal("AI proposal appeared before proposals were explicitly enabled")
	}
}

func TestSearchCanExplicitlyIncludeAIProposals(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40
	model.searching = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	model = updated.(Model)
	view := model.View().Content

	if !model.includeIdeas {
		t.Fatal("expected proposal-inclusive search")
	}
	if !strings.Contains(view, "Church Investigator Arrives") {
		t.Fatal("expected AI proposal after proposals were explicitly enabled")
	}
}

func TestViewEnablesMouseTracking(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40

	view := model.View()
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected cell-motion mouse support, got %v", view.MouseMode)
	}
}

func TestMouseWheelNavigatesRecords(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40

	updated, _ := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(Model)
	if model.cursor != 1 {
		t.Fatalf("expected mouse wheel to select record 1, got %d", model.cursor)
	}
}

func TestMouseClickSelectsRecord(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40

	updated, _ := model.Update(tea.MouseClickMsg{X: 6, Y: 7, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.cursor != 2 {
		t.Fatalf("expected clicked record 2, got %d", model.cursor)
	}
}

func TestViewFillsTerminal(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 24

	content := model.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != model.height {
		t.Fatalf("expected %d rendered rows, got %d", model.height, len(lines))
	}
	for index, line := range lines {
		if width := lipgloss.Width(line); width != model.width {
			t.Fatalf("row %d: expected width %d, got %d", index, model.width, width)
		}
	}
}

func TestCreateDraftEntity(t *testing.T) {
	model := New()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model = updated.(Model)
	if !model.editing || !model.creating {
		t.Fatal("expected new draft editor")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.editing {
		t.Fatal("expected editor to close after saving")
	}
	if got := model.workspace.Records[len(model.workspace.Records)-1]; got.Authority != domain.Draft || got.Type != domain.NPC {
		t.Fatalf("expected saved NPC draft, got %#v", got)
	}
}

func TestTypeFilterSeparatesRecords(t *testing.T) {
	model := New()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	model = updated.(Model)
	if model.typeFilter != domain.NPC {
		t.Fatalf("expected NPC filter, got %q", model.typeFilter)
	}
	for _, record := range model.visibleRecords() {
		if record.Type != domain.NPC {
			t.Fatalf("type filter leaked %q record", record.Type)
		}
	}
}

func TestSessionCaptureAndEntityReview(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 30
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	if model.session == nil {
		t.Fatal("expected active session")
	}
	model.sessionInput.SetValue("@Cap")
	model.refreshSuggestions()
	if len(model.suggestions) == 0 {
		t.Fatal("expected entity autosuggestions while typing a reference")
	}
	model.sessionInput.SetValue("@Captain Vale searches the reliquary")
	model.review = model.resolveReference(model.sessionInput.Value())
	if model.review == nil || model.review.Title != "Captain Vale" {
		t.Fatalf("expected Captain Vale review, got %#v", model.review)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Mod: tea.ModCtrl}))
	model = updated.(Model)
	if len(model.session.Entries) != 1 || len(model.session.Entries[0].Links) != 1 {
		t.Fatalf("expected one linked transcript entry, got %#v", model.session.Entries)
	}
	if !strings.Contains(model.View().Content, "CURRENT SCENE") || !strings.Contains(model.View().Content, "Captain Vale") {
		t.Fatal("session view should include the live scene context pane")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.session != nil {
		t.Fatal("expected session to end with Ctrl+E")
	}
}

func TestSessionViewFillsTerminal(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 24
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	lines := strings.Split(model.View().Content, "\n")
	if len(lines) != model.height {
		t.Fatalf("expected %d session rows, got %d", model.height, len(lines))
	}
	for index, line := range lines {
		if lipgloss.Width(line) != model.width {
			t.Fatalf("row %d: expected width %d, got %d", index, model.width, lipgloss.Width(line))
		}
	}
}

func TestSessionCommandsCreateDrafts(t *testing.T) {
	model := New()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("$npc Sister Elayne: A church investigator")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Mod: tea.ModCtrl}))
	model = updated.(Model)
	created := model.workspace.Records[len(model.workspace.Records)-1]
	if created.Type != domain.NPC || created.Authority != domain.Draft || created.Title != "Sister Elayne" {
		t.Fatalf("unexpected session-created entity: %#v", created)
	}
	if model.review == nil || model.review.ID != created.ID {
		t.Fatal("expected created entity to open in the review context")
	}
	model.sessionInput.SetValue("#random item")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.workspace.Records[len(model.workspace.Records)-1].Type != domain.Item {
		t.Fatal("expected #random item to create an item draft")
	}
}
