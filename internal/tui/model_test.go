package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
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

func TestDeleteSelectedEntity(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	if model.selectedID == "" {
		t.Fatal("expected a selected record")
	}
	target := model.selectedID
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	if !model.deleteConfirm || model.confirmKind != "delete" {
		t.Fatal("first d should arm delete confirmation")
	}
	// Second d alone must not delete — requires explicit y.
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	for _, record := range model.workspace.Records {
		if record.ID == target {
			goto stillPresent
		}
	}
	t.Fatal("delete must not proceed without y")
stillPresent:
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)
	for _, record := range model.workspace.Records {
		if record.ID == target {
			t.Fatalf("expected record %q to be deleted after y", target)
		}
	}
	if model.selectedID == target {
		t.Fatal("selection should move off deleted record")
	}
}

func TestSupersedeSelectedCanonEntity(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	// Captain Vale is canon in fixtures
	for _, record := range model.workspace.Records {
		if record.Title == "Captain Vale" {
			model.selectRecord(record)
			break
		}
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Text: "x"}))
	model = updated.(Model)
	if !model.deleteConfirm || model.confirmKind != "supersede" {
		t.Fatal("x should arm supersede confirmation")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)
	record := model.selectedRecord()
	if record == nil || record.Authority != domain.Superseded {
		t.Fatalf("expected superseded canon entity after y, got %#v", record)
	}
}

func TestCloseFocusedBrowserPane(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	model.setBrowserFocus(prefs.PaneFaction)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '-', Text: "-"}))
	model = updated.(Model)
	if prefs.FindVisibleLeaf(model.layout.Browser.Root, prefs.PaneFaction) {
		t.Fatal("expected faction pane closed")
	}
}

func TestMouseClickSelectsRecord(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40

	var target domain.Record
	var x, y int
	found := false
	for _, region := range model.browserRegions() {
		if region.Pane != prefs.PaneNPC || len(region.Rows) < 2 {
			continue
		}
		target = region.Rows[1]
		x = region.MinX + 2
		y = region.Offset + 1
		found = true
		break
	}
	if !found {
		t.Fatal("expected NPC pane hit region")
	}
	updated, _ := model.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.selectedID != target.ID {
		t.Fatalf("expected clicked record %q, got %q", target.ID, model.selectedID)
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
	if !strings.Contains(content, "NPC") || !strings.Contains(content, "LOCATION") {
		t.Fatalf("expected typed section panes in browser view: %q", content)
	}
}

func TestCreateDraftEntity(t *testing.T) {
	model := New()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model = updated.(Model)
	if !model.editing || !model.creating {
		t.Fatal("expected new draft editor")
	}
	if !strings.Contains(model.editBody.Value(), "# New NPC") {
		t.Fatalf("expected markdown template, got %q", model.editBody.Value())
	}
	model.editBody.SetValue("type: LOCATION\n\n# Quiet Alcove\n\nA side chamber.\n\nDust and old chalk.\n")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.editing {
		t.Fatal("expected editor to close after saving")
	}
	got := model.workspace.Records[len(model.workspace.Records)-1]
	if got.Authority != domain.Draft || got.Type != domain.Location || got.Title != "Quiet Alcove" {
		t.Fatalf("expected saved location draft from markdown, got %#v", got)
	}
	if got.Summary != "A side chamber." {
		t.Fatalf("summary=%q", got.Summary)
	}
}

func TestEditEntityUsesMarkdownDocument(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	for _, record := range model.workspace.Records {
		if record.Title == "Captain Vale" {
			model.selectRecord(record)
			break
		}
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}))
	model = updated.(Model)
	if !model.editing || model.creating {
		t.Fatal("expected edit markdown editor")
	}
	doc := model.editBody.Value()
	if !strings.Contains(doc, "type: NPC") || !strings.Contains(doc, "# Captain Vale") {
		t.Fatalf("expected entity markdown document, got %q", doc)
	}
	if strings.Contains(model.View().Content, "Title:") && strings.Contains(model.View().Content, "Summary:") {
		t.Fatal("form fields should not appear in markdown editor")
	}
}

func TestTypeFilterSeparatesRecords(t *testing.T) {
	model := New()
	if model.layout.Focus != prefs.PaneNPC {
		t.Fatalf("expected initial NPC section focus, got %q", model.layout.Focus)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	model = updated.(Model)
	if model.layout.Focus != prefs.PaneLocation || model.typeFilter != domain.Location {
		t.Fatalf("expected location section after Tab/t, got focus=%q type=%q", model.layout.Focus, model.typeFilter)
	}
	for _, record := range model.recordsForPane(model.layout.Focus) {
		if record.Type != domain.Location {
			t.Fatalf("location pane leaked %q record", record.Type)
		}
	}
}

func TestBrowserAddTypePane(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	model.layout.CycleBrowserTypeVisibility() // core 4
	before := len(prefs.VisibleLeaves(model.layout.Browser.Root))
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '+', Text: "+"}))
	model = updated.(Model)
	after := len(prefs.VisibleLeaves(model.layout.Browser.Root))
	if after <= before {
		t.Fatalf("expected + to add a type pane, before=%d after=%d", before, after)
	}
}

func TestPlannedNotesSaveAndSeedLiveSession(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Text: "p"}))
	model = updated.(Model)
	if !model.planning {
		t.Fatal("expected planned notes editor")
	}
	model.planTitle.SetValue("Crypt approach")
	model.planBody.SetValue("Meet @Captain Vale\n#location Greywatch\n- Relic pressure\n")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.planning {
		t.Fatal("expected editor to close after save")
	}
	if len(model.workspace.PlannedNotes) != 1 {
		t.Fatalf("expected saved planned notes, got %d", len(model.workspace.PlannedNotes))
	}
	plan := model.workspace.PlannedNotes[0]
	if plan.LocationName == "" || len(plan.Links) == 0 {
		t.Fatalf("expected resolved location and links, got %#v", plan)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	if model.session == nil {
		t.Fatal("expected live session")
	}
	if model.session.PlannedNotesID != plan.ID {
		t.Fatalf("live session should reference planned notes, got %q", model.session.PlannedNotesID)
	}
	if model.session.LocationName != plan.LocationName {
		t.Fatalf("expected location from plan %q, got %q", plan.LocationName, model.session.LocationName)
	}
	if model.review == nil || model.review.Title != "Captain Vale" {
		t.Fatalf("expected review seeded from plan link, got %#v", model.review)
	}
}

func TestSessionReconciliationPreservesTranscript(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 30
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("@Captain Vale finds the key")
	model.review = model.resolveReference(model.sessionInput.Value())
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.session == nil || len(model.session.Entries) == 0 {
		t.Fatal("expected transcript entry")
	}
	original := model.session.Entries[0].Text
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.session != nil {
		t.Fatal("expected session to end")
	}
	if len(model.workspace.Reconciliations) == 0 {
		t.Fatal("expected reconciliation record after ending session")
	}
	if model.workspace.Sessions[0].Entries[0].Text != original {
		t.Fatal("ending session must leave transcript text intact")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Text: "r"}))
	model = updated.(Model)
	if !model.reconciling {
		t.Fatal("expected reconciliation overlay")
	}
	if !strings.Contains(model.View().Content, "SESSION RECONCILIATION") {
		t.Fatalf("expected reconciliation UI: %q", model.View().Content)
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
	if !strings.Contains(model.View().Content, "Type a transcript entry") && !strings.Contains(model.View().Content, "@ entity reference") {
		t.Fatalf("empty session input should explain how to enter and submit text: %q", model.View().Content)
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

func TestPlainEnterSubmitsTranscript(t *testing.T) {
	model := New()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("The party enters the crypt")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Text: "\r"}))
	model = updated.(Model)
	if len(model.session.Entries) != 1 || model.session.Entries[0].Text != "The party enters the crypt" {
		t.Fatalf("plain Enter did not submit transcript: %#v", model.session.Entries)
	}
}

func TestShiftEnterAddsMultilineTextWithoutSubmitting(t *testing.T) {
	model := New()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("first line")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter, Mod: tea.ModShift}))
	model = updated.(Model)
	if len(model.session.Entries) != 0 {
		t.Fatal("Shift+Enter must not submit a transcript")
	}
	if model.sessionInput.Value() != "first line\n" {
		t.Fatalf("expected multiline input, got %q", model.sessionInput.Value())
	}
}

func TestSessionTranscriptScrollsAndPanesCycle(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 30
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	for index := 0; index < 30; index++ {
		model.session.Entries = append(model.session.Entries, domain.TranscriptEntry{Text: "event", CreatedAt: time.Unix(int64(index), 0)})
	}
	model.refreshTranscriptViewport()
	if !model.transcriptView.AtBottom() {
		t.Fatal("transcript should start at the newest entry")
	}
	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	model = updated.(Model)
	if model.transcriptView.AtBottom() {
		t.Fatal("mouse wheel should scroll the transcript")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.layout.SessionLeafVisible(prefs.PaneCampaign) || !model.layout.SessionLeafVisible(prefs.PaneContext) {
		t.Fatalf("expected context-only after Ctrl+P, got %s", model.layout.Session.Root.Describe())
	}
}

func TestMouseDragResizesBrowserAndSessionGutters(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 30
	divider := model.splitWidth(model.width)
	updated, _ := model.Update(tea.MouseClickMsg{X: divider, Y: 10, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: 52, Y: 10, Button: tea.MouseLeft})
	model = updated.(Model)
	if absRatio(model.layout.VerticalRatio()-0.52) > 0.03 {
		t.Fatalf("expected browser gutter ratio near 0.52, got %v", model.layout.VerticalRatio())
	}
	updated, _ = model.Update(tea.MouseReleaseMsg{X: 52, Y: 10, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.draggingSplit {
		t.Fatal("expected browser gutter drag to end on release")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	divider = model.splitWidth(model.width)
	updated, _ = model.Update(tea.MouseClickMsg{X: divider, Y: 4, Button: tea.MouseLeft})
	model = updated.(Model)
	if !model.draggingSplit {
		t.Fatal("expected session gutter drag to start")
	}
	upper := model.sessionUpperHeight()
	updated, _ = model.Update(tea.MouseReleaseMsg{X: divider, Y: 4, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseClickMsg{X: 70, Y: upper + 1, Button: tea.MouseLeft})
	model = updated.(Model)
	if !model.draggingSplit || model.dragAxis != "horizontal" {
		t.Fatal("expected horizontal gutter drag to start")
	}
	beforeTranscript := model.sessionTranscriptHeight()
	beforeUpperRatio := model.layout.SessionUpperRatio()
	updated, _ = model.Update(tea.MouseMotionMsg{X: 70, Y: upper + 4, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.layout.SessionUpperRatio() <= beforeUpperRatio {
		t.Fatalf("expected upper ratio to grow, before=%v after=%v", beforeUpperRatio, model.layout.SessionUpperRatio())
	}
	if model.sessionTranscriptHeight() >= beforeTranscript {
		t.Fatalf("expected transcript to give space to upper pane, before=%d after=%d", beforeTranscript, model.sessionTranscriptHeight())
	}
}
