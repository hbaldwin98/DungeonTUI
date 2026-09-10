package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/ingest/fivetools"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

type failingStore struct{}

func (failingStore) Load() (domain.Workspace, error) {
	return domain.Workspace{}, errors.New("load failed")
}
func (failingStore) Save(domain.Workspace) error { return errors.New("disk full") }

func flushCmd(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected command")
	}
	deadline := time.Now().Add(5 * time.Second)
	for cmd != nil {
		if time.Now().After(deadline) {
			t.Fatal("command did not finish")
		}
		msg := cmd()
		if msg == nil {
			return model
		}
		updated, next := model.Update(msg)
		model = updated.(Model)
		if !model.importBusy {
			return model
		}
		cmd = next
	}
	return model
}

func TestLibraryPickerSelectsCampaign(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 80
	model.height = 24
	model.openPicker()
	if !model.picking {
		t.Fatal("expected picker")
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.pickerLevel != "campaign" {
		t.Fatalf("expected campaign list, got %q", model.pickerLevel)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.picking {
		t.Fatal("expected to enter campaign context")
	}
	if model.workspace.Scope.CampaignID != "ashen-crown" {
		t.Fatalf("scope=%#v", model.workspace.Scope)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'b', Text: "b"}))
	model = updated.(Model)
	if !model.picking || model.pickerLevel != "world" {
		t.Fatal("b should return to world picker")
	}
	if !strings.Contains(model.View().Content, "WORLDS") {
		t.Fatalf("expected library picker view: %q", model.View().Content)
	}
}

func TestLibraryPickerRenamesWorld(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 80
	model.height = 24
	model.openPicker()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}))
	model = updated.(Model)
	if !model.namingPicker {
		t.Fatal("expected rename prompt")
	}
	model.collectionName.SetValue("The Cinder Marches")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.namingPicker {
		t.Fatal("rename should close")
	}
	world, ok := model.workspace.FindWorld("ashen-realms")
	if !ok || world.Name != "The Cinder Marches" {
		t.Fatalf("world=%#v ok=%v", world, ok)
	}
	if model.workspace.Scope.WorldName != "The Cinder Marches" {
		t.Fatalf("scope still %q", model.workspace.Scope.WorldName)
	}
	if !strings.Contains(model.View().Content, "The Cinder Marches") {
		t.Fatalf("view missing rename: %q", model.View().Content)
	}
}

func TestLibraryPickerDeleteCampaignRequiresConfirm(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 80
	model.height = 24
	model.openPicker()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	if !model.deleteConfirm || model.confirmKind != "delete-campaign" {
		t.Fatalf("confirm=%v kind=%q", model.deleteConfirm, model.confirmKind)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model = updated.(Model)
	if model.deleteConfirm {
		t.Fatal("n should cancel")
	}
	if _, ok := model.workspace.ScopeFor("ashen-realms", "ashen-crown"); !ok {
		t.Fatal("cancel should keep campaign")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)
	if _, ok := model.workspace.ScopeFor("ashen-realms", "ashen-crown"); ok {
		t.Fatal("confirmed delete should drop campaign")
	}
	foundVale := false
	foundGreywatch := false
	for _, record := range model.workspace.Records {
		if record.Title == "Captain Vale" {
			foundVale = true
		}
		if record.Title == "Greywatch" {
			foundGreywatch = true
		}
	}
	if foundVale || !foundGreywatch {
		t.Fatalf("vale=%v greywatch=%v records=%d", foundVale, foundGreywatch, len(model.workspace.Records))
	}
}

func TestLibraryPickerDeletesWorld(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 80
	model.height = 24
	model.openPicker()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)
	if _, ok := model.workspace.FindWorld("barovia"); ok {
		t.Fatal("barovia should be gone")
	}
	if _, ok := model.workspace.FindWorld("ashen-realms"); !ok {
		t.Fatal("ashen-realms should remain")
	}
}

func TestLibraryPickerCreateAfterDeleteUsesUniqueID(t *testing.T) {
	ws := demoWorkspace()
	if err := ws.DeleteWorld("ashen-realms"); err != nil {
		t.Fatal(err)
	}
	model := newModel(ws, nil, nil)
	model.openPicker()
	updated, _ := model.createPickerItem()
	model = updated.(Model)
	seen := map[string]bool{}
	for _, world := range model.workspace.Library {
		if seen[world.ID] {
			t.Fatalf("duplicate world ID %q", world.ID)
		}
		seen[world.ID] = true
	}
}

func TestDeleteRollsBackWhenPersistenceFails(t *testing.T) {
	model := newModel(demoWorkspace(), failingStore{}, nil)
	model.enterDemoCampaign()
	record := model.selectedRecord()
	if record == nil {
		t.Fatal("expected selected record")
	}
	model.deleteSelected()
	found := false
	for _, candidate := range model.workspace.Records {
		found = found || candidate.ID == record.ID
	}
	if !found || !strings.Contains(model.status, "persistence failed") {
		t.Fatalf("found=%v status=%q", found, model.status)
	}
}

func TestEndSessionRemainsLiveWhenPersistenceFails(t *testing.T) {
	model := newModel(demoWorkspace(), failingStore{}, nil)
	model.enterDemoCampaign()
	started, _ := model.startSession()
	model = started.(Model)
	updated, _ := model.endSession()
	model = updated.(Model)
	if model.session == nil || model.session.EndedAt != nil {
		t.Fatalf("session = %#v", model.session)
	}
	if !strings.Contains(model.status, "persistence failed") {
		t.Fatalf("status = %q", model.status)
	}
}

func TestReconciliationRejectRollsBackWhenPersistenceFails(t *testing.T) {
	model := newModel(demoWorkspace(), failingStore{}, nil)
	model.workspace.Reconciliations = []domain.ReconciliationRecord{{
		SessionID: "session-1",
		Items:     []domain.ReconciliationItem{{Status: domain.ReconPending}},
	}}
	model.reconciling = true
	updated, _ := model.updateReconciliation(tea.KeyPressMsg{Code: 'x'})
	model = updated.(Model)
	if got := model.workspace.Reconciliations[0].Items[0].Status; got != domain.ReconPending {
		t.Fatalf("status = %q", got)
	}
}

func TestEnterScopeResumesUnfinishedSession(t *testing.T) {
	ws := demoWorkspace()
	ws.Sessions = append(ws.Sessions, domain.SessionRecord{
		ID: "session-live", Title: "Live session", Scope: ws.Scope, StartedAt: time.Now().UTC(),
	})
	model := newModel(ws, nil, nil)
	model.enterScope(ws.Scope)
	if model.session == nil || model.session.ID != "session-live" {
		t.Fatalf("session = %#v", model.session)
	}
	before := len(model.workspace.Sessions)
	model.selectedSessionID = "session-live"
	updated, _ := model.activateBrowserSelection()
	model = updated.(Model)
	if model.session == nil || model.session.ID != "session-live" || len(model.workspace.Sessions) != before {
		t.Fatalf("session=%#v sessions=%d", model.session, len(model.workspace.Sessions))
	}
}

func TestCreateCampaignUsesUniqueID(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.openPicker()
	model.pickerLevel = "campaign"
	model.pickerWorldID = model.workspace.Library[0].ID
	updated, _ := model.createPickerItem()
	model = updated.(Model)
	seen := map[string]bool{}
	for _, world := range model.workspace.Library {
		for _, campaign := range world.Campaigns {
			if seen[campaign.ID] {
				t.Fatalf("duplicate campaign ID %q", campaign.ID)
			}
			seen[campaign.ID] = true
		}
	}
}

func TestToggleImportSourceRollsBackWhenPersistenceFails(t *testing.T) {
	model := newModel(demoWorkspace(), failingStore{}, nil)
	model.workspace.Sources = []domain.SourceDocument{{ID: "source", Title: "Source", Kind: domain.SourceAdventure}}
	model.importFocus = "sources"
	model.importSourceCursor = 0
	updated, _ := model.toggleImportSource()
	model = updated.(Model)
	if model.workspace.SourceEnabled(model.workspace.Scope, "source") {
		t.Fatal("source remained enabled after failed persistence")
	}
}

func TestCorruptWorkspaceDisablesWrites(t *testing.T) {
	model := newLoadFailureModel(failingStore{}, nil, errors.New("decode workspace: unexpected EOF"))
	if model.store != nil {
		t.Fatal("corrupt load must disable writes")
	}
	if !strings.Contains(model.status, "writes disabled") {
		t.Fatalf("status=%q", model.status)
	}
}

func TestMissingWorkspaceStartsEmptyLibrary(t *testing.T) {
	model := newLoadFailureModel(failingStore{}, nil, fmt.Errorf("workspace not found: %w", os.ErrNotExist))
	if model.store == nil {
		t.Fatal("missing workspace should keep the store for first save")
	}
	if len(model.workspace.Library) != 0 {
		t.Fatalf("first run must not seed demo campaigns, library=%#v", model.workspace.Library)
	}
	if !model.picking {
		t.Fatal("expected library picker")
	}
	model = newLoadFailureModel(nil, nil, fmt.Errorf("workspace not found: %w", os.ErrNotExist))
	updated, _ := model.createPickerItem()
	model = updated.(Model)
	if len(model.workspace.Library) != 1 {
		t.Fatalf("n should create a world, library=%#v", model.workspace.Library)
	}
}

func TestReconciliationNavigationAndApproval(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.workspace.Reconciliations = []domain.ReconciliationRecord{{SessionID: "session", Items: []domain.ReconciliationItem{{Status: domain.ReconPending}, {Status: domain.ReconPending}}}}
	model.reconciling = true
	updated, _ := model.updateReconciliation(tea.KeyPressMsg{Code: 'a'})
	model = updated.(Model)
	if model.workspace.Reconciliations[0].Items[0].Status != domain.ReconApproved {
		t.Fatal("item was not approved")
	}
	updated, _ = model.updateReconciliation(tea.KeyPressMsg{Code: 'j'})
	model = updated.(Model)
	if model.reconCursor != 1 {
		t.Fatalf("cursor = %d", model.reconCursor)
	}
	updated, _ = model.updateReconciliation(tea.KeyPressMsg{Code: 'x'})
	model = updated.(Model)
	if model.workspace.Reconciliations[0].Items[1].Status != domain.ReconRejected {
		t.Fatal("item was not rejected")
	}
	updated, _ = model.updateReconciliation(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(Model)
	if model.reconciling {
		t.Fatal("reconciliation remained open")
	}
}

func TestReconciliationApproveRollsBackWhenPersistenceFails(t *testing.T) {
	model := newModel(demoWorkspace(), failingStore{}, nil)
	model.workspace.Reconciliations = []domain.ReconciliationRecord{{
		SessionID: "session-1",
		Items:     []domain.ReconciliationItem{{Status: domain.ReconPending}},
	}}
	model.reconciling = true
	updated, _ := model.updateReconciliation(tea.KeyPressMsg{Code: 'a'})
	model = updated.(Model)
	if got := model.workspace.Reconciliations[0].Items[0].Status; got != domain.ReconPending {
		t.Fatalf("status = %q", got)
	}
}

func TestReconciliationApplyCreatesNoteAndEditsMutation(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 100
	model.height = 36
	ended := time.Now().UTC()
	session := domain.SessionRecord{
		ID: "sit-1", Title: "Sit 1", Scope: model.workspace.Scope, EndedAt: &ended,
		Entries: []domain.TranscriptEntry{{ID: "e1", Text: "They buried the moonstone token."}},
	}
	recon := domain.BuildSessionReconciliation(session, model.workspace.Records)
	model.workspace.Sessions = append(model.workspace.Sessions, session)
	model.workspace.Reconciliations = []domain.ReconciliationRecord{recon}
	model.reconciling = true
	model.reconIndex = 0
	for index, item := range recon.Items {
		if item.Kind == domain.ReconTranscriptNote {
			model.reconCursor = index
			break
		}
	}
	updated, _ := model.updateReconciliation(tea.KeyPressMsg{Code: 'e'})
	model = updated.(Model)
	if !model.reconEditing {
		t.Fatal("e should edit the mutation")
	}
	model.reconEdit.SetValue("The moonstone was buried under the chapel.")
	updated, _ = model.updateReconEdit(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	model = updated.(Model)
	if model.reconciling && model.workspace.Reconciliations[0].Items[model.reconCursor].Mutation.Text != "The moonstone was buried under the chapel." {
		t.Fatalf("mutation=%#v", model.workspace.Reconciliations[0].Items[model.reconCursor])
	}
	updated, _ = model.updateReconciliation(tea.KeyPressMsg{Code: 'a'})
	model = updated.(Model)
	found := false
	for _, record := range model.workspace.Records {
		if record.ID == "note-e1" {
			found = true
			if record.Type != domain.Note || record.Authority != domain.Canon {
				t.Fatalf("note=%#v", record)
			}
			if record.Body != "The moonstone was buried under the chapel." {
				t.Fatalf("body=%q", record.Body)
			}
			if record.Source != "Sit 1" {
				t.Fatalf("source=%q", record.Source)
			}
		}
	}
	if !found {
		t.Fatalf("expected sourced note, records=%#v", model.workspace.Records)
	}
	if model.workspace.Sessions[len(model.workspace.Sessions)-1].Entries[0].Text != "They buried the moonstone token." {
		t.Fatal("transcript mutated")
	}
}

func TestReconciliationCiteRollsBackWhenPersistenceFails(t *testing.T) {
	model := newModel(demoWorkspace(), failingStore{}, nil)
	before := model.workspace.Records[0].Body
	target := model.workspace.Records[0].ID
	model.workspace.Reconciliations = []domain.ReconciliationRecord{{
		SessionID: "session-1",
		Items: []domain.ReconciliationItem{{
			Status:   domain.ReconPending,
			Kind:     domain.ReconLinkReview,
			RecordID: target,
			EntryID:  "e1",
			Mutation: domain.Mutation{Op: domain.MutationCite, RecordID: target, Text: "Cited from play."},
		}},
	}}
	model.reconciling = true
	updated, _ := model.updateReconciliation(tea.KeyPressMsg{Code: 'a'})
	model = updated.(Model)
	if model.workspace.Records[0].Body != before {
		t.Fatalf("body changed after failed save: %q", model.workspace.Records[0].Body)
	}
	if model.workspace.Reconciliations[0].Items[0].Status != domain.ReconPending {
		t.Fatal("status should roll back")
	}
}

func TestPlannedLinksUseOperationalScope(t *testing.T) {
	ws := demoWorkspace()
	ws.Records = append(ws.Records, domain.Record{
		ID: "foreign-vale", Type: domain.NPC, Title: "Foreign Vale", Aliases: []string{"Captain Vale"},
		Authority: domain.Canon, Scope: domain.Scope{WorldID: "barovia", WorldName: "Barovia", CampaignID: "curse-of-strahd", Campaign: "Curse of Strahd"},
	})
	model := newModel(ws, nil, nil)
	model.listScope = searchsvc.EntireLibrary
	model.tagFilter = "does-not-exist"
	model.collectionFilter = "missing-collection"
	updatedModel, _ := model.openPlannedNotes(true)
	model = updatedModel.(Model)
	model.planTitle.SetValue("Scoped prep")
	model.planBody.SetValue("Meet @Captain Vale")
	updated, _ := model.savePlannedNotes()
	model = updated.(Model)
	if len(model.workspace.PlannedNotes) == 0 || len(model.workspace.PlannedNotes[len(model.workspace.PlannedNotes)-1].Links) != 1 {
		t.Fatalf("plans = %#v", model.workspace.PlannedNotes)
	}
	link := model.workspace.PlannedNotes[len(model.workspace.PlannedNotes)-1].Links[0]
	if link.RecordID != "npc-captain-vale" {
		t.Fatalf("planned link ignored operational scope: %#v", link)
	}
}

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

func TestSearchFindsPrepAndOpensIt(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, domain.PlannedNotes{
		ID:    "plan-ambush",
		Title: "Crypt Ambush",
		Body:  "Vale waits at the east gate.",
		Scope: model.workspace.Scope,
	})
	model.rebuildSearch()
	model.searching = true
	model.searchInput.SetValue("ambush")
	model.refreshResults()

	if !searchHasKind(model.results, searchsvc.KindPrep, "plan-ambush") {
		t.Fatalf("expected prep hit, got %#v", model.results)
	}
	selectSearchKind(&model, searchsvc.KindPrep, "plan-ambush")
	view := model.View().Content
	if !strings.Contains(view, "prep") || !strings.Contains(view, "Crypt Ambush") {
		t.Fatalf("expected labeled prep result, got %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.searching {
		t.Fatal("search overlay should close")
	}
	if model.navKind != NavPrep || model.selectedPlanID != "plan-ambush" {
		t.Fatalf("expected prep branch, nav=%q plan=%q", model.navKind, model.selectedPlanID)
	}
}

func TestSearchFindsSessionTranscriptAndRecon(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 40
	ended := time.Now()
	model.workspace.Sessions = append(model.workspace.Sessions, domain.SessionRecord{
		ID: "sit-17", Title: "Greywatch Watch", Scope: model.workspace.Scope,
		EndedAt: &ended, LocationName: "Ruined Monastery",
		Entries: []domain.TranscriptEntry{
			{ID: "e-key", Text: "They pocketed the moonstone token."},
		},
	})
	model.workspace.Reconciliations = append(model.workspace.Reconciliations, domain.ReconciliationRecord{
		ID: "recon-sit-17", SessionID: "sit-17", Title: "Reconcile Greywatch Watch",
		Items: []domain.ReconciliationItem{{Summary: "Promote draft: Sister Elayne"}},
	})
	model.rebuildSearch()

	model.searching = true
	model.searchInput.SetValue("greywatch watch")
	model.refreshResults()
	if !searchHasKind(model.results, searchsvc.KindSession, "sit-17") {
		t.Fatalf("expected session hit, got %#v", model.results)
	}

	model.searchInput.SetValue("moonstone token")
	model.refreshResults()
	if !searchHasKind(model.results, searchsvc.KindTranscript, "e-key") {
		t.Fatalf("expected transcript hit, got %#v", model.results)
	}
	selectSearchKind(&model, searchsvc.KindTranscript, "e-key")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.navKind != NavSessions || model.selectedSessionID != "sit-17" {
		t.Fatalf("expected session focus, nav=%q session=%q", model.navKind, model.selectedSessionID)
	}

	model.searching = true
	model.searchInput.SetValue("sister elayne")
	model.refreshResults()
	if !searchHasKind(model.results, searchsvc.KindRecon, "recon-sit-17") {
		t.Fatalf("expected recon hit, got %#v", model.results)
	}
	selectSearchKind(&model, searchsvc.KindRecon, "recon-sit-17")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.reconciling || model.workspace.Reconciliations[model.reconIndex].ID != "recon-sit-17" {
		t.Fatalf("expected recon overlay, reconciling=%v index=%d", model.reconciling, model.reconIndex)
	}
}

func TestPersistSucceedsWhenSearchIndexCannotWrite(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "workspace.json")
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := newModel(demoWorkspace(), storage.NewJSON(wsPath), nil)
	model.searchPath = filepath.Join(blocked, "search.sqlite")
	model.rebuildSearch()
	if err := model.persistWorkspace(); err != nil {
		t.Fatalf("workspace save should ignore index failure: %v", err)
	}
	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected workspace.json: %v", err)
	}
	model.searching = true
	model.searchInput.SetValue("vale")
	model.refreshResults()
	if len(model.results) == 0 {
		t.Fatal("memory fallback should still find campaign records")
	}
}

func TestSQLiteStoreHoldsCampaignData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.sqlite")
	store := storage.NewSQLite(path)
	defer store.Close()
	model := newModel(demoWorkspace(), store, nil)
	if err := model.persistWorkspace(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "workspace.json")); !os.IsNotExist(err) {
		t.Fatal("sqlite store must not write workspace.json")
	}
	model.searching = true
	model.searchInput.SetValue("vale")
	model.rebuildSearch()
	model.refreshResults()
	if len(model.results) == 0 {
		t.Fatal("expected FTS hits from workspace.sqlite")
	}
	store.Close()
	loaded, err := storage.NewSQLite(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Records) == 0 {
		t.Fatal("expected campaign records in sqlite")
	}
}

func searchHasKind(results []searchsvc.Result, kind searchsvc.Kind, id string) bool {
	for _, result := range results {
		if result.Kind == kind && result.TargetID() == id {
			return true
		}
	}
	return false
}

func selectSearchKind(model *Model, kind searchsvc.Kind, id string) {
	for index, result := range model.results {
		if result.Kind == kind && result.TargetID() == id {
			model.selected = index
			return
		}
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
	model.setBrowserFocus(prefs.PaneList)

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
	model.setBrowserFocus(prefs.PaneList)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '-', Text: "-"}))
	model = updated.(Model)
	if !prefs.FindVisibleLeaf(model.layout.Browser.Root, prefs.PaneList) {
		t.Fatal("campaign tree panes must stay open")
	}
	if !strings.Contains(model.status, "stay open") {
		t.Fatalf("expected stay-open status, got %q", model.status)
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
		if region.Pane != prefs.PaneList || len(region.RecordTree) < 2 {
			continue
		}
		idx := -1
		for i, row := range region.RecordTree {
			if row.Kind == domain.RecordTreeRecord {
				if idx >= 0 {
					target = row.Record
					x = region.MinX + 2
					y = region.Offset + (i - region.WindowStart)
					found = true
					break
				}
				idx = i
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("expected list pane hit region with records")
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
	if !strings.Contains(content, "CAMPAIGN") || !strings.Contains(content, "Sessions") || !strings.Contains(content, "Prep") || !strings.Contains(content, "Sources") {
		t.Fatalf("expected campaign tree in browser view: %q", content)
	}
	if !strings.Contains(content, "NPCs") {
		t.Fatalf("expected NPC branch in campaign tree: %q", content)
	}
}

func TestCampaignTreeFiltersListBySection(t *testing.T) {
	model := New()
	if model.layout.Focus != prefs.PaneNav {
		t.Fatalf("expected nav focus by default, got %q", model.layout.Focus)
	}
	if model.currentNav().Type != domain.NPC {
		t.Fatalf("expected initial NPC section, got %#v", model.currentNav())
	}
	// Move nav to Locations (index 5: Sessions, Prep, Sources, NPCs, Characters, Locations)
	model.setNavCursor(5)
	if model.layout.Focus != prefs.PaneNav {
		t.Fatalf("nav cursor move must keep nav focus, got %q", model.layout.Focus)
	}
	if model.currentNav().Type != domain.Location {
		t.Fatalf("expected Locations, got %#v", model.currentNav())
	}
	for _, record := range model.listRecords() {
		if record.Type != domain.Location {
			t.Fatalf("location section leaked %q record", record.Type)
		}
	}
}

func TestCampaignTreeShowsPrepBranch(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, domain.PlannedNotes{
		ID:    "plan-test",
		Title: "Crypt approach",
		Scope: model.workspace.Scope,
		Body:  "Meet @Captain Vale",
	})
	model.setNavCursor(1) // Prep notes
	content := model.View().Content
	if !strings.Contains(content, "PREP NOTES") && !strings.Contains(content, "Crypt approach") {
		t.Fatalf("expected prep list content, got %q", content)
	}
	if model.selectedPlanID != "plan-test" {
		t.Fatalf("expected prep selection, got %q", model.selectedPlanID)
	}
}

func TestTypeFilterSeparatesRecords(t *testing.T) {
	model := New()
	if model.layout.Focus != prefs.PaneNav {
		t.Fatalf("expected initial nav focus, got %q", model.layout.Focus)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 't', Text: "t"}))
	model = updated.(Model)
	if model.layout.Focus != prefs.PaneList {
		t.Fatalf("expected list after Tab/t from nav, got focus=%q", model.layout.Focus)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	model = updated.(Model)
	if model.layout.Focus != prefs.PaneNav {
		t.Fatalf("expected nav after Shift+Tab, got focus=%q", model.layout.Focus)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model = updated.(Model)
	if model.layout.Focus != prefs.PaneDetail {
		t.Fatalf("expected detail after left from nav, got focus=%q", model.layout.Focus)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.layout.Focus != prefs.PaneNav {
		t.Fatalf("expected nav after right from detail, got focus=%q", model.layout.Focus)
	}
}

func TestDeleteSessionFromTree(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	model.workspace.Sessions = append(model.workspace.Sessions, domain.SessionRecord{
		ID:    "session-del",
		Title: "Throwaway sit",
		Scope: model.workspace.Scope,
	})
	model.setNavCursor(0) // Sessions
	model.setBrowserFocus(prefs.PaneList)
	if model.selectedSessionID != "session-del" {
		t.Fatalf("expected session selected, got %q", model.selectedSessionID)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	if !model.deleteConfirm || model.confirmKind != "delete-session" {
		t.Fatalf("expected session delete confirm, got confirm=%v kind=%q", model.deleteConfirm, model.confirmKind)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)
	for _, session := range model.workspace.Sessions {
		if session.ID == "session-del" {
			t.Fatal("session should be deleted")
		}
	}
}

func TestBrowserAddTypePane(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '+', Text: "+"}))
	model = updated.(Model)
	if !prefs.FindVisibleLeaf(model.layout.Browser.Root, prefs.PaneNav) {
		t.Fatal("campaign tree should remain after +")
	}
	if !strings.Contains(model.status, "campaign tree") && !strings.Contains(model.status, "Sections live") {
		t.Fatalf("expected tree guidance status, got %q", model.status)
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

func TestCreateWorldCanonEntityPersistsAndAppearsInSiblingCampaign(t *testing.T) {
	ws := demoWorkspace()
	for index := range ws.Library {
		if ws.Library[index].ID == ws.Scope.WorldID {
			ws.Library[index].Campaigns = append(ws.Library[index].Campaigns, domain.CampaignRef{
				ID: "sibling-campaign", Name: "Sibling Campaign",
			})
		}
	}
	store := storage.NewSQLite(filepath.Join(t.TempDir(), "workspace.sqlite"))
	defer store.Close()
	model := newModel(ws, store, nil)
	updated, _ := model.openEditor(true)
	model = updated.(Model)
	model.editBody.SetValue("type: LOCATION\nauthority: canon\nscope: world\n\n# Shared Keep\n\nVisible across the world.\n")
	updated, _ = model.saveEditor()
	model = updated.(Model)

	record := model.workspace.Records[len(model.workspace.Records)-1]
	if record.Authority != domain.Canon || record.Scope.WorldID != ws.Scope.WorldID || record.Scope.CampaignID != "" {
		t.Fatalf("saved record=%#v", record)
	}
	persisted, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := persisted.Records[len(persisted.Records)-1]; got.ID != record.ID || got.Authority != domain.Canon || got.Scope.CampaignID != "" {
		t.Fatalf("persisted record=%#v", got)
	}

	sibling, ok := model.workspace.ScopeFor(ws.Scope.WorldID, "sibling-campaign")
	if !ok {
		t.Fatal("expected sibling campaign scope")
	}
	model.enterScope(sibling)
	if !model.recordInListScope(record) {
		t.Fatal("world record should be visible in sibling campaign")
	}
	model.selectRecord(record)
	if detail := model.renderDetailWidth(80); !strings.Contains(detail, "WORLD SHARED · "+ws.Scope.WorldName) {
		t.Fatalf("detail does not identify shared world scope: %q", detail)
	}
}

func TestEditEntityAppliesAuthorityAndScope(t *testing.T) {
	model := New()
	record := model.workspace.Records[0]
	record.Authority = domain.Draft
	record.Scope = model.workspace.Scope
	model.workspace.Records[0] = record
	model.selectRecord(record)
	updated, _ := model.openEditor(false)
	model = updated.(Model)
	model.editBody.SetValue("type: " + string(record.Type) + "\nauthority: secret\nscope: world\n\n# " + record.Title + "\n\nChanged.\n")
	updated, _ = model.saveEditor()
	model = updated.(Model)
	got := model.workspace.Records[0]
	if got.Authority != domain.Secret || got.Scope.CampaignID != "" || got.Scope.WorldID != record.Scope.WorldID {
		t.Fatalf("edited record=%#v", got)
	}

	model.selectRecord(got)
	updated, _ = model.openEditor(false)
	model = updated.(Model)
	model.editBody.SetValue("type: " + string(got.Type) + "\nauthority: canon\nscope: campaign\n\n# " + got.Title + "\n\nChanged again.\n")
	updated, _ = model.saveEditor()
	model = updated.(Model)
	got = model.workspace.Records[0]
	if got.Authority != domain.Canon || got.Scope != model.workspace.Scope {
		t.Fatalf("campaign-scoped record=%#v", got)
	}
}

func TestEditWorldEntityRejectsCampaignScopeFromAnotherWorld(t *testing.T) {
	model := New()
	record := model.workspace.Records[0]
	record.Scope = domain.Scope{WorldID: "another-world", WorldName: "Another World"}
	model.workspace.Records[0] = record
	model.selectRecord(record)
	updated, _ := model.openEditor(false)
	model = updated.(Model)
	model.editBody.SetValue("type: " + string(record.Type) + "\nauthority: secret\nscope: campaign\n\n# " + record.Title + "\n\nShould not save.\n")
	updated, _ = model.saveEditor()
	model = updated.(Model)

	got := model.workspace.Records[0]
	if !reflect.DeepEqual(got, record) {
		t.Fatalf("record changed across worlds: got=%#v want=%#v", got, record)
	}
	if !model.editing || model.status != "Open a campaign in this record's world before changing its scope" {
		t.Fatalf("editing=%v status=%q", model.editing, model.status)
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

func TestMarkdownEditorSuggestsEntityReferences(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	model = updated.(Model)
	model.editBody.SetValue("type: NOTE\n\n# Hook\n\nMeet @Cap")
	// Cursor ends at end after SetValue; refresh suggestions as typing would.
	model.refreshEditorSuggestions()
	if len(model.suggestions) == 0 {
		t.Fatal("expected @ suggestions for Cap")
	}
	found := false
	for _, item := range model.suggestions {
		if item.Record != nil && item.Record.Title == "Captain Vale" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Captain Vale suggestion, got %#v", model.suggestions)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	model = updated.(Model)
	if !strings.Contains(model.editBody.Value(), "@Captain Vale") {
		t.Fatalf("Tab should insert suggestion, got %q", model.editBody.Value())
	}
}

func TestHelpOverlayListsBrowserCommands(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 24
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	model = updated.(Model)
	if !model.helping {
		t.Fatal("expected help overlay")
	}
	content := model.View().Content
	if !strings.Contains(content, "COMMANDS") || !strings.Contains(content, "Browser") {
		t.Fatalf("expected help content, got %q", content)
	}
	if !strings.Contains(content, "PgUp/PgDn") {
		t.Fatalf("expected detail scroll keys in help: %q", content)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.helping {
		t.Fatal("esc should close help")
	}
}

func TestTagAndScopeFiltersNarrowBrowserList(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	// NPCs section: Vale has greywatch+guard, Merrow has greywatch+clergy.
	before := len(model.listRecords())
	if before < 2 {
		t.Fatalf("expected multiple NPCs, got %d", before)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'f', Text: "f"}))
	model = updated.(Model)
	if model.tagFilter == "" {
		t.Fatal("expected first tag filter to engage")
	}
	for _, record := range model.listRecords() {
		if !containsTag(record, model.tagFilter) {
			t.Fatalf("tag filter leaked record %#v", record)
		}
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Text: "o"}))
	model = updated.(Model)
	if model.listScope != searchsvc.CurrentWorld {
		t.Fatalf("expected world scope after o, got %v", model.listScope)
	}
}

func TestOperationalRecordsIgnoreBrowserFilters(t *testing.T) {
	model := New()
	baselineNPCs := model.countByType(domain.NPC)
	baselineThreads := len(model.sessionThreads())
	baselineLocation := model.defaultSessionLocation()
	if baselineLocation == nil {
		t.Fatal("fixture should provide a default session location")
	}
	model.listScope = searchsvc.EntireLibrary
	model.tagFilter = "does-not-exist"
	model.collectionFilter = "missing-collection"

	if records := model.visibleRecords(); len(records) != 0 {
		t.Fatalf("browser filters should hide all records, got %#v", records)
	}
	if got := model.countByType(domain.NPC); got != baselineNPCs {
		t.Fatalf("browser filters changed operational NPC count: got %d want %d", got, baselineNPCs)
	}
	if got := len(model.sessionThreads()); got != baselineThreads {
		t.Fatalf("browser filters changed session threads: got %d want %d", got, baselineThreads)
	}
	if location := model.defaultSessionLocation(); location == nil || location.ID != baselineLocation.ID {
		t.Fatalf("browser filters changed default session location: %#v", location)
	}
	if location := model.findLocation(baselineLocation.Title); location == nil || location.ID != baselineLocation.ID {
		t.Fatalf("browser filters changed location lookup: %#v", location)
	}
	model.session = &domain.SessionRecord{LocationID: baselineLocation.ID}
	if location := model.sessionLocationRecord(); location == nil || location.ID != baselineLocation.ID {
		t.Fatalf("browser filters changed session location: %#v", location)
	}
	links := model.resolveLinks("@Captain Vale enters")
	if len(links) != 1 || links[0].RecordID != "npc-captain-vale" {
		t.Fatalf("filtered campaign record did not resolve: %#v", links)
	}
	suggestions := model.entitySuggestions("captain")
	if len(suggestions) == 0 || suggestions[0].Record == nil || suggestions[0].Record.ID != "npc-captain-vale" {
		t.Fatalf("filtered campaign record was absent from suggestions: %#v", suggestions)
	}
	if record := model.resolveReference("Review @Captain Vale."); record == nil || record.ID != "npc-captain-vale" {
		t.Fatalf("browser filters changed live reference resolution: %#v", record)
	}
	if record := model.resolveReferenceAtCursor("Review @Captain Vale.", len("Review @Captain")); record == nil || record.ID != "npc-captain-vale" {
		t.Fatalf("browser filters changed cursor reference resolution: %#v", record)
	}
	model.review = nil
	model.campaignCursor = 0 // NPC
	model.activateCampaignCursor()
	if model.review == nil || model.review.ID != "npc-captain-vale" {
		t.Fatalf("browser filters changed campaign keyboard selection: %#v", model.review)
	}
	model.review = nil
	updated, _ := model.applyHit(hitTarget{Action: hitCampaignType, EntityType: domain.NPC})
	model = updated.(Model)
	if model.review == nil || model.review.ID != "npc-captain-vale" {
		t.Fatalf("browser filters changed campaign mouse selection: %#v", model.review)
	}
}

func TestOperationalRecordsExcludeForeignCampaignRecords(t *testing.T) {
	model := New()
	foreign := domain.Record{
		ID: "foreign-oracle", Type: domain.NPC, Title: "Foreign Oracle", Authority: domain.Canon,
		Scope: domain.Scope{WorldID: model.workspace.Scope.WorldID, WorldName: model.workspace.Scope.WorldName, CampaignID: "other-campaign", Campaign: "Other Campaign"},
	}
	foreignLocation := foreign
	foreignLocation.ID = "foreign-location"
	foreignLocation.Type = domain.Location
	foreignLocation.Title = "Foreign Keep"
	foreignLocation.Tags = []string{"current-scene"}
	foreignThread := foreign
	foreignThread.ID = "foreign-thread"
	foreignThread.Type = domain.Thread
	foreignThread.Title = "Foreign Plot"
	model.workspace.Records = append(model.workspace.Records, foreign, foreignLocation, foreignThread)
	model.listScope = searchsvc.CurrentWorld
	model.typeFilter = ""

	visible := 0
	for _, record := range model.visibleRecords() {
		if record.ID == foreign.ID || record.ID == foreignLocation.ID || record.ID == foreignThread.ID {
			visible++
		}
	}
	if visible != 3 {
		t.Fatalf("world browser should display all foreign-campaign records, got %d", visible)
	}
	for _, record := range model.operationalRecords() {
		if record.Scope.CampaignID == "other-campaign" {
			t.Fatalf("foreign-campaign record leaked into operational scope: %#v", record)
		}
	}
	if record := model.resolveReference("Review @Foreign Oracle"); record != nil {
		t.Fatalf("foreign-campaign record resolved for live review: %#v", record)
	}
	if record := model.resolveReferenceAtCursor("Review @Foreign Oracle", len("Review @Foreign")); record != nil {
		t.Fatalf("foreign-campaign record resolved at cursor: %#v", record)
	}
	if links := model.resolveLinks("@Foreign Oracle appears"); len(links) != 0 {
		t.Fatalf("foreign-campaign record resolved in transcript: %#v", links)
	}
	if suggestions := model.entitySuggestions("foreign oracle"); len(suggestions) != 0 {
		t.Fatalf("foreign-campaign record appeared in suggestions: %#v", suggestions)
	}
	mentions := model.resolveMentions("Ask @Foreign Oracle")
	if len(mentions) != 1 || mentions[0].RecordID != "" {
		t.Fatalf("foreign-campaign prose mention should remain unresolved: %#v", mentions)
	}
	wantStyle := New().renderProseWithMentions("Ask @Foreign Oracle")
	if got := model.renderProseWithMentions("Ask @Foreign Oracle"); got != wantStyle {
		t.Fatal("foreign-campaign prose mention was styled as resolved")
	}
	if location := model.findLocation("Foreign Keep"); location != nil {
		t.Fatalf("foreign-campaign location was found operationally: %#v", location)
	}
	model.session = &domain.SessionRecord{
		LocationID: foreignLocation.ID,
		Links:      []domain.EntityLink{{Text: foreign.Title, RecordID: foreign.ID}},
	}
	if location := model.sessionLocationRecord(); location != nil {
		t.Fatalf("foreign-campaign location leaked into current scene: %#v", location)
	}
	if present := model.sessionPresent(); len(present) != 0 {
		t.Fatalf("foreign-campaign cast leaked into session: %#v", present)
	}
	for _, thread := range model.sessionThreads() {
		if thread.ID == foreignThread.ID {
			t.Fatalf("foreign-campaign thread leaked into session: %#v", thread)
		}
	}
}

func TestStartSessionIgnoresForeignPlannedContext(t *testing.T) {
	model := New()
	foreignScope := domain.Scope{
		WorldID: model.workspace.Scope.WorldID, WorldName: model.workspace.Scope.WorldName,
		CampaignID: "other-campaign", Campaign: "Other Campaign",
	}
	foreignNPC := domain.Record{ID: "foreign-npc", Type: domain.NPC, Title: "Foreign NPC", Authority: domain.Canon, Scope: foreignScope}
	foreignLocation := domain.Record{ID: "foreign-place", Type: domain.Location, Title: "Foreign Place", Authority: domain.Canon, Scope: foreignScope}
	model.workspace.Records = append(model.workspace.Records, foreignNPC, foreignLocation)
	plan := domain.PlannedNotes{
		ID: "stale-plan", Title: "Stale plan", Scope: model.workspace.Scope,
		LocationID: foreignLocation.ID, LocationName: foreignLocation.Title,
		Links: []domain.EntityLink{{Text: foreignNPC.Title, RecordID: foreignNPC.ID}},
	}
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)
	model.selectedPlanID = plan.ID
	model.review = &foreignNPC

	updated, _ := model.startSession()
	model = updated.(Model)
	if model.session == nil {
		t.Fatal("expected a session")
	}
	if model.session.LocationID == foreignLocation.ID || model.session.HasLink(foreignNPC.ID) {
		t.Fatalf("foreign planned context leaked into session: %#v", model.session)
	}
	if model.review != nil {
		t.Fatalf("foreign review survived session start: %#v", model.review)
	}
}

func TestCollectionFilterAddAndCreate(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	model.setNavCursor(3) // NPCs
	before := len(model.listRecords())
	if before < 3 {
		t.Fatalf("expected campaign NPCs including drafts, got %d", before)
	}
	detail := model.renderDetail()
	if !strings.Contains(detail, "COLLECTIONS") || !strings.Contains(detail, "Greywatch circle") {
		t.Fatalf("vale should belong to fixture collection: %q", detail)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c"}))
	model = updated.(Model)
	if model.collectionFilter == "" {
		t.Fatal("expected collection filter")
	}
	filtered := model.listRecords()
	if len(filtered) >= before {
		t.Fatalf("collection should narrow NPCs, before=%d after=%d", before, len(filtered))
	}
	for _, record := range filtered {
		if record.Title == "Sister Elayne" {
			t.Fatal("Elayne is not in Greywatch circle")
		}
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Text: "c"}))
	model = updated.(Model)
	if model.collectionFilter != "" {
		t.Fatal("cycling past last collection should clear filter")
	}
	for _, record := range model.listRecords() {
		if record.Title == "Sister Elayne" {
			model.selectRecord(record)
			break
		}
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	model = updated.(Model)
	col, ok := domain.FindCollection(model.workspace.Collections, "col-greywatch")
	if !ok || !col.Has("draft-sister-elayne") {
		t.Fatalf("a should add selected entity to last collection, col=%#v", col)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Text: "g"}))
	model = updated.(Model)
	if !model.namingCollection {
		t.Fatal("g should open collection name overlay")
	}
	model.collectionName.SetValue("Crypt pressure")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.namingCollection {
		t.Fatal("enter should save collection")
	}
	found := false
	for _, item := range model.workspace.Collections {
		if item.Title == "Crypt pressure" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected named collection, got %#v", model.workspace.Collections)
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
	if !model.session.HasLink(plan.Links[0].RecordID) {
		t.Fatalf("expected session cast seeded from plan, got %#v", model.session.Links)
	}
	if model.review == nil || model.review.Title != "Captain Vale" {
		t.Fatalf("expected review seeded from plan link, got %#v", model.review)
	}
	if !strings.Contains(model.View().Content, "Captain Vale") {
		t.Fatal("PRESENT should list seeded cast")
	}
	if view := testANSI.ReplaceAllString(model.View().Content, ""); !strings.Contains(view, "PREP · Crypt") || !strings.Contains(view, "Meet @Captain Vale") {
		t.Fatalf("live session should show its prepared run sheet, got:\n%s", view)
	}
}

func TestLiveSessionPrepPaneScrollsThroughRunSheet(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 24
	var body strings.Builder
	body.WriteString("# Opening beat\n\n")
	for index := 1; index <= 20; index++ {
		fmt.Fprintf(&body, "- Prepared beat %02d\n", index)
	}
	body.WriteString("\n# Final confrontation\n")
	plan := domain.PlannedNotes{
		ID:    "plan-scroll",
		Title: "The long night",
		Scope: model.workspace.Scope,
		Body:  body.String(),
	}
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)
	model.selectedPlanID = plan.ID

	started, _ := model.startSession()
	model = started.(Model)
	model.setSessionFocus(prefs.PaneCampaign)
	firstPage := testANSI.ReplaceAllString(renderContentLines(model.sessionCampaignContentLines(40, 8)), "")
	if !strings.Contains(firstPage, "Opening beat") || strings.Contains(firstPage, "Final confrontation") {
		t.Fatalf("expected first prep page, got:\n%s", firstPage)
	}

	updated, _ := model.updateSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	model = updated.(Model)
	lastPage := testANSI.ReplaceAllString(renderContentLines(model.sessionCampaignContentLines(40, 8)), "")
	if !strings.Contains(lastPage, "Final confrontation") || strings.Contains(lastPage, "Opening beat") {
		t.Fatalf("expected final prep page after End, scroll=%d, got:\n%s", model.prepScroll, lastPage)
	}

	updated, _ = model.updateSession(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	model = updated.(Model)
	maxScroll := model.sessionPrepMaxScroll()
	model.prepScroll = maxScroll
	for _, step := range []struct {
		key  tea.KeyPressMsg
		want int
	}{
		{key: tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), want: maxScroll},
		{key: tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}), want: maxScroll - model.sessionPrepPageSize()},
		{key: tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}), want: 0},
		{key: tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}), want: 0},
		{key: tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}), want: model.sessionPrepPageSize()},
		{key: tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), want: model.sessionPrepPageSize() + 1},
	} {
		updated, _ = model.updateSession(step.key)
		model = updated.(Model)
		if model.prepScroll != step.want {
			t.Fatalf("key %v: prep scroll=%d want %d", step.key.String(), model.prepScroll, step.want)
		}
	}

	updated, _ = model.updateSession(tea.KeyPressMsg(tea.Key{Code: 'z', Text: "z"}))
	model = updated.(Model)
	if model.sessionFocus() != prefs.PaneInput || model.sessionInput.Value() != "z" {
		t.Fatalf("typing from prep should return to capture input, focus=%s input=%q", model.sessionFocus(), model.sessionInput.Value())
	}
}

func TestLiveSessionEmptyPrepHasUsefulPlaceholder(t *testing.T) {
	model := New()
	plan := domain.PlannedNotes{ID: "empty-plan", Title: "Unwritten", Scope: model.workspace.Scope}
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, plan)
	model.selectedPlanID = plan.ID

	started, _ := model.startSession()
	model = started.(Model)
	got := testANSI.ReplaceAllString(renderContentLines(model.sessionCampaignContentLines(40, 8)), "")
	if !strings.Contains(got, "PREP · Unwritten") || !strings.Contains(got, "No prepared notes") {
		t.Fatalf("empty prep pane should explain its state, got:\n%s", got)
	}
}

func TestHelpSectionsDescribeEachActiveMode(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*Model)
		heading string
	}{
		{name: "editor", prepare: func(m *Model) { m.editing = true }, heading: "Markdown editor"},
		{name: "planned notes", prepare: func(m *Model) { m.planning = true }, heading: "Planned notes"},
		{name: "session", prepare: func(m *Model) { m.session = &domain.SessionRecord{} }, heading: "Session"},
		{name: "preview", prepare: func(m *Model) { m.preview = &previewBuf{} }, heading: "Link preview"},
		{name: "playback", prepare: func(m *Model) { m.playingBack = true }, heading: "Playback"},
		{name: "rename vault", prepare: func(m *Model) { m.namingPicker = true }, heading: "Rename vault"},
		{name: "library", prepare: func(m *Model) { m.picking = true }, heading: "Library"},
		{name: "session folder", prepare: func(m *Model) { m.namingFolder = true }, heading: "Session folder"},
		{name: "collection", prepare: func(m *Model) { m.namingCollection = true }, heading: "New collection"},
		{name: "import", prepare: func(m *Model) { m.importing = true }, heading: "Import"},
		{name: "reconciliation edit", prepare: func(m *Model) { m.reconciling, m.reconEditing = true, true }, heading: "Edit mutation"},
		{name: "reconciliation", prepare: func(m *Model) { m.reconciling = true }, heading: "Reconciliation"},
		{name: "search", prepare: func(m *Model) { m.searching = true }, heading: "Search"},
		{name: "browser", prepare: func(m *Model) {}, heading: "Browser"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := New()
			test.prepare(&model)
			sections := model.helpSections()
			if len(sections) != 1 || len(sections[0]) == 0 || sections[0][0] != test.heading {
				t.Fatalf("help sections=%v, want heading %q", sections, test.heading)
			}
			if test.name == "session" && !strings.Contains(strings.Join(sections[0], "\n"), "prep pane j/k") {
				t.Fatalf("session help does not explain prep scrolling: %v", sections[0])
			}
		})
	}
}

func TestOnePrepCoversMultipleLiveSits(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Text: "p"}))
	model = updated.(Model)
	model.planTitle.SetValue("Crypt arc")
	model.planBody.SetValue("Meet @Captain Vale\n#location Greywatch\n")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	model = updated.(Model)
	planID := model.workspace.PlannedNotes[0].ID

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	firstID := model.session.ID
	if model.session.Title != "Crypt arc" || model.session.PlannedNotesID != planID {
		t.Fatalf("first sit should keep plan title, got %#v", model.session)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	if model.session.ID == firstID {
		t.Fatal("second sit must be a new live session")
	}
	if model.session.PlannedNotesID != planID {
		t.Fatalf("second sit should still seed from the same prep, got %q", model.session.PlannedNotesID)
	}
	if model.session.Title == "Crypt arc" {
		t.Fatalf("later sit should disambiguate title, got %q", model.session.Title)
	}
	if !strings.HasPrefix(model.session.Title, "Crypt arc · ") {
		t.Fatalf("later sit title=%q", model.session.Title)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)

	seeded := domain.SessionsSeededFrom(model.workspace.Sessions, planID)
	if len(seeded) != 2 {
		t.Fatalf("prep should cover two live sits, got %#v", seeded)
	}
	if len(model.workspace.PlannedNotes) != 1 {
		t.Fatal("starting live must not consume or clone the prep document")
	}

	model.selectedPlanID = planID
	model.setNavCursor(1) // Prep branch
	detail := model.renderTreeDetail()
	if !strings.Contains(detail, "live ·") || !strings.Contains(detail, "Crypt arc") {
		t.Fatalf("prep detail should list live sits: %q", detail)
	}
}

func TestNewPrepAttachesEndedSessionContext(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("@Captain Vale searches the crypt")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if len(model.workspace.Sessions) == 0 || model.workspace.Sessions[0].EndedAt == nil {
		t.Fatal("expected ended session")
	}
	priorTitle := model.workspace.Sessions[0].Title
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Text: "p"}))
	model = updated.(Model)
	if !model.planning {
		t.Fatal("expected planned notes editor")
	}
	if len(model.planDraft.PriorSessionIDs) != 1 || model.planDraft.PriorSessionIDs[0] != model.workspace.Sessions[0].ID {
		t.Fatalf("expected new prep to attach ended sit, got %#v", model.planDraft.PriorSessionIDs)
	}
	view := model.View().Content
	if !strings.Contains(view, "PRIOR SITS") || !strings.Contains(view, priorTitle) {
		t.Fatalf("prep overlay should show prior sit context: %q", view)
	}
	if !strings.Contains(model.planBody.Value(), priorTitle) {
		t.Fatalf("new prep body should follow up from last sit, got %q", model.planBody.Value())
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if len(model.planDraft.PriorSessionIDs) != 0 {
		t.Fatalf("Ctrl+P should cycle off prior sits, got %#v", model.planDraft.PriorSessionIDs)
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

func TestEndedSessionPlaybackScrubsDerivedBeats(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("@Captain Vale finds the key")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	original := model.session.Entries[0].Text
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)

	model.setNavCursor(0)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.playingBack {
		t.Fatal("Enter on an ended session should open playback")
	}
	view := model.View().Content
	if !strings.Contains(view, "SESSION PLAYBACK") {
		t.Fatalf("expected playback overlay: %q", view)
	}
	if !strings.Contains(view, "Captain Vale") {
		t.Fatalf("playback should surface associated entity: %q", view)
	}
	if model.workspace.Sessions[0].Entries[0].Text != original {
		t.Fatal("playback must leave transcript text intact")
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	model = updated.(Model)
	if model.playbackCursor != 1 {
		t.Fatalf("→ should fast-forward, cursor=%d", model.playbackCursor)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	model = updated.(Model)
	if model.playbackCursor != 0 {
		t.Fatalf("← should rewind, cursor=%d", model.playbackCursor)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.playingBack {
		t.Fatal("Esc should close playback")
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
	if !model.session.HasLink(model.session.Entries[0].Links[0].RecordID) {
		t.Fatalf("expected auto-associate into session Links, got %#v", model.session.Links)
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
	if !model.session.HasLink(created.ID) {
		t.Fatalf("$ create should auto-associate, got %#v", model.session.Links)
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

func TestEntityCoherenceBacklinksHistoryAndPeek(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	model.sessionInput.SetValue("@Captain Vale searches the crypt")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if len(model.session.Links) != 1 {
		t.Fatalf("expected association, got %#v", model.session.Links)
	}
	valeID := model.session.Links[0].RecordID
	npcCountBefore := 0
	for _, record := range model.workspace.Records {
		if record.Type == domain.NPC && record.Authority != domain.Proposal {
			npcCountBefore++
		}
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if len(model.workspace.Sessions) == 0 || !model.workspace.Sessions[0].HasLink(valeID) {
		t.Fatal("ended session should retain cast Links")
	}
	npcCountAfter := 0
	for _, record := range model.workspace.Records {
		if record.Type == domain.NPC && record.Authority != domain.Proposal {
			npcCountAfter++
		}
	}
	if npcCountAfter != npcCountBefore {
		t.Fatalf("associating must not shrink wiki NPC list (%d → %d)", npcCountBefore, npcCountAfter)
	}

	for _, record := range model.workspace.Records {
		if record.ID == valeID {
			model.selectRecord(record)
			break
		}
	}
	detail := model.renderDetail()
	if !strings.Contains(detail, "LINKED") || !strings.Contains(detail, "session ·") {
		t.Fatalf("expected backlinks section: %q", detail)
	}
	if !strings.Contains(detail, "HISTORY") {
		t.Fatalf("expected history section: %q", detail)
	}
	model.layout.Focus = prefs.PaneDetail
	for index, hop := range model.detailHops() {
		if hop.Kind == hopHistory {
			model.historyCursor = index
			break
		}
	}
	detail = model.renderDetail()
	if !strings.Contains(detail, "Associated with session cast") && !strings.Contains(detail, "Review @") {
		t.Fatalf("expected expandable history events: %q", detail)
	}

	model.sessionInput.SetValue("@Cap")
	// reopen live to test peek without needing session; use editor instead
	updated, _ = model.openEditor(false)
	model = updated.(Model)
	model.editBody.SetValue("Meet @Captain Vale later")
	model.refreshPeek() // may miss until cursor is set
	record := model.resolveReferenceAtCursor(model.editBody.Value(), strings.Index(model.editBody.Value(), "Vale"))
	if record == nil || record.Title != "Captain Vale" {
		t.Fatalf("expected peek for @Captain Vale, got %#v", record)
	}
	model.peek = record
	if !strings.Contains(model.renderPeekPanel(6), "PEEK") {
		t.Fatal("expected peek panel chrome")
	}
}

func TestSessionFoldersGroupCollapseAndFile(t *testing.T) {
	ended := time.Date(2026, 3, 2, 21, 0, 0, 0, time.UTC)
	end := ended
	model := New()
	model.width = 100
	model.height = 36
	model.workspace.Sessions = []domain.SessionRecord{
		{
			ID: "crypt-2", Title: "Crypt night 2", Folder: "Greywatch/Crypt",
			Scope: model.workspace.Scope, StartedAt: ended, EndedAt: &end,
		},
		{
			ID: "crypt-1", Title: "Crypt night 1", Folder: "Greywatch/Crypt",
			Scope: model.workspace.Scope, StartedAt: ended.Add(-24 * time.Hour), EndedAt: &end,
		},
		{
			ID: "loose", Title: "Loose march sit",
			Scope: model.workspace.Scope, StartedAt: time.Date(2026, 3, 8, 18, 0, 0, 0, time.UTC), EndedAt: &end,
		},
	}
	model.setNavCursor(0)
	model.setBrowserFocus(prefs.PaneList)
	list := model.renderListPane(40)
	if !strings.Contains(list, "Greywatch") || !strings.Contains(list, "Crypt") || !strings.Contains(list, "Crypt night 2") {
		t.Fatalf("expected nested folders: %q", list)
	}
	if pad := leadingPad(list, "Greywatch"); pad >= leadingPad(list, "Crypt night 2") {
		t.Fatalf("sits should indent under their folder, greywatch=%d sit=%d\n%s", pad, leadingPad(list, "Crypt night 2"), list)
	}
	if pad := leadingPad(list, "Greywatch"); pad >= leadingPad(list, "Crypt ·") {
		t.Fatalf("nested folder should indent under parent, greywatch=%d crypt=%d\n%s", pad, leadingPad(list, "Crypt ·"), list)
	}
	if !strings.Contains(list, "2026-03") {
		t.Fatalf("unfiled sit should bucket by month: %q", list)
	}

	model.applySessionTreeCursor(0)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	list = model.renderListPane(40)
	if strings.Contains(list, "Crypt night 2") {
		t.Fatalf("enter on folder should collapse sits: %q", list)
	}
	detail := model.renderTreeDetail()
	if !strings.Contains(detail, "FOLDER") || !strings.Contains(detail, "Greywatch") {
		t.Fatalf("folder detail: %q", detail)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	selectSessionRow(&model, "crypt-2")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.playingBack {
		t.Fatal("enter on ended sit should open playback")
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)

	selectSessionRow(&model, "loose")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'm', Text: "m"}))
	model = updated.(Model)
	if !model.namingFolder {
		t.Fatal("m should open folder overlay")
	}
	model.collectionName.SetValue("Greywatch/Crypt")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	found := false
	for _, session := range model.workspace.Sessions {
		if session.ID == "loose" && session.Folder == "Greywatch/Crypt" {
			found = true
		}
	}
	if !found {
		t.Fatal("loose sit should be filed under Greywatch/Crypt")
	}
	if strings.Contains(model.renderListPane(40), "2026-03") {
		t.Fatal("month bucket should disappear once its sits are filed")
	}

	selectSessionRow(&model, "crypt-2")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	model = updated.(Model)
	if model.session == nil || model.session.Folder != "Greywatch/Crypt" {
		t.Fatalf("new sit should inherit folder, got %#v", model.session)
	}
}

func TestWikiMentionsFollowAndBreak(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	var vale domain.Record
	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			vale = record
			model.selectRecord(record)
			break
		}
	}
	model.layout.Focus = prefs.PaneDetail
	detail := model.renderDetail()
	if !strings.Contains(detail, "REFERENCES") || !strings.Contains(detail, "Father Merrow") {
		t.Fatalf("expected outgoing wiki ref: %q", detail)
	}
	if !strings.Contains(detail, "wiki ·") {
		t.Fatalf("expected wiki backlink chrome: %q", detail)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview == nil {
		t.Fatal("Enter on a wiki hop should open a preview, not jump")
	}
	if model.selectedID != "npc-captain-vale" {
		t.Fatalf("preview should keep place, selected=%q", model.selectedID)
	}
	view := model.View().Content
	if !strings.Contains(view, "PREVIEW") || !strings.Contains(view, "Father Merrow") {
		t.Fatalf("preview overlay missing target: %q", view)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("Esc should close preview")
	}
	if model.selectedID != "npc-captain-vale" {
		t.Fatalf("Esc should not navigate, selected=%q", model.selectedID)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("second Enter should jump and close preview")
	}
	if model.selectedID != "npc-father-merrow" {
		t.Fatalf("Enter in preview should open @Father Merrow, selected=%q", model.selectedID)
	}

	kept := make([]domain.Record, 0, len(model.workspace.Records))
	for _, record := range model.workspace.Records {
		if record.ID == "npc-father-merrow" {
			continue
		}
		kept = append(kept, record)
	}
	model.workspace.Records = kept
	model.selectRecord(vale)
	_, broken := domain.EntityOutgoingRefs(vale, model.workspace.Records)
	if len(broken) == 0 {
		t.Fatal("deleted target should leave a broken @ mention")
	}
	detail = model.renderDetail()
	if !strings.Contains(detail, "missing · @") {
		t.Fatalf("expected missing-ref row: %q", detail)
	}
	model.layout.Focus = prefs.PaneDetail
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("broken mentions should not open a preview")
	}
	if !strings.Contains(model.status, "Missing @") {
		t.Fatalf("broken mention should warn, status=%q", model.status)
	}
}

func TestDetailTrailAndLinkRelationGuideFollowing(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			model.selectRecord(record)
			break
		}
	}
	model.layout.Focus = prefs.PaneDetail
	detail := model.renderDetail()
	for _, want := range []string{
		"PATH  The Ashen Crown / NPCs / Captain Vale",
		"(outgoing @)",
		"(backlink)",
		"j/k select · Enter preview · Enter again follow",
	} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail should explain %q: %q", want, detail)
		}
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	frame := model.previewFrame()
	for _, want := range []string{
		"Relation · outgoing @ mention",
		"Enter again follows this row",
		"Enter follow",
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("preview should explain %q: %q", want, frame)
		}
	}
}

func TestBrowserBackspaceRestoresPreviousLocation(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	var first, second domain.Record
	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			first = record
		}
		if record.ID == "npc-father-merrow" {
			second = record
		}
	}
	if first.ID == "" || second.ID == "" {
		t.Fatal("demo records should include both navigation targets")
	}
	model.selectRecord(first)
	model.layout.Focus = prefs.PaneDetail
	model.pushBrowserLocation()
	model.selectRecord(second)

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	model = updated.(Model)
	if model.selectedID != first.ID {
		t.Fatalf("backspace should restore %q, selected=%q", first.ID, model.selectedID)
	}
	if len(model.browserHistory) != 0 {
		t.Fatalf("backspace should consume the location, history=%d", len(model.browserHistory))
	}
	if !strings.Contains(model.status, "Back to "+first.Title) {
		t.Fatalf("backspace status=%q", model.status)
	}
}

func TestImportSourceStateGuidanceIsVisible(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 32
	model.openImport()
	sources := model.renderImportSources(80, 12)
	if !strings.Contains(sources, "● enabled · ○ cached, not active · e toggles selected source") {
		t.Fatalf("source pane should explain its states: %q", sources)
	}
	model.width = 180
	view := model.renderImport()
	if !strings.Contains(view, "e enable/disable selected source") {
		t.Fatalf("import footer should explain source toggling: %q", view)
	}
	model.status = "Cached Tome as reference · Sources selected · press e to enable for this campaign"
	if !strings.Contains(model.renderImport(), "press e to enable for this campaign") {
		t.Fatalf("cached-source status should remain actionable: %q", model.renderImport())
	}
}

func TestSearchResultsShowContextAndStaySingleLine(t *testing.T) {
	model := New()
	model.width = 64
	model.height = 20
	record := domain.Record{
		ID:        "search-context-record",
		Type:      domain.NPC,
		Title:     "Contextual Search Target",
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Folder:    "lore/deep",
		Source:    "A source title that is long enough to be truncated",
	}
	result := searchsvc.Result{Kind: searchsvc.KindRecord, ID: record.ID, Title: record.Title, Record: record}
	context := model.searchResultContext(result)
	if !strings.Contains(context, "lore/deep") || !strings.Contains(context, "A source title") {
		t.Fatalf("search context=%q", context)
	}
	model.searching = true
	model.searchInput.SetValue("Contextual")
	model.results = []searchsvc.Result{result}
	view := testANSI.ReplaceAllString(model.renderSearchOverlay(), "")
	var resultLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Contextual Search") {
			resultLine = line
			break
		}
	}
	if resultLine == "" {
		t.Fatalf("search result row missing: %q", view)
	}
	if !strings.Contains(resultLine, "…") {
		t.Fatalf("search context should truncate on narrow terminals: %q", resultLine)
	}
}

func TestCtrlOPeekPreviewPreservesActiveModes(t *testing.T) {
	recordID := "npc-captain-vale"
	setPeek := func(model *Model) {
		record := model.workspace.Records[0]
		for _, candidate := range model.workspace.Records {
			if candidate.ID == recordID {
				record = candidate
				break
			}
		}
		model.peek = &record
	}
	cases := []struct {
		name   string
		active func(*Model)
		check  func(Model) bool
	}{
		{name: "editor", active: func(model *Model) {
			model.selectRecord(*model.peek)
			updated, _ := model.openEditor(false)
			*model = updated.(Model)
		}, check: func(model Model) bool { return model.editing }},
		{name: "session", active: func(model *Model) {
			session := domain.SessionRecord{ID: "test-session", Scope: model.workspace.Scope}
			model.session = &session
		}, check: func(model Model) bool { return model.session != nil }},
		{name: "planned notes", active: func(model *Model) {
			updated, _ := model.openPlannedNotes(false)
			*model = updated.(Model)
		}, check: func(model Model) bool { return model.planning }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			model := New()
			model.width = 100
			model.height = 36
			setPeek(&model)
			test.active(&model)
			updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'o', Mod: tea.ModCtrl}))
			model = updated.(Model)
			if model.preview == nil || model.preview.Hop.RecordID != recordID {
				t.Fatalf("ctrl+o should preview the peeked record: %#v", model.preview)
			}
			if !strings.Contains(model.View().Content, "PREVIEW") {
				t.Fatalf("ctrl+o preview should be visible in %s mode: %q", test.name, model.View().Content)
			}
			if !test.check(model) {
				t.Fatalf("ctrl+o should preserve %s mode", test.name)
			}
		})
	}
}

func TestHistoryFollowTargetsPlaybackEntry(t *testing.T) {
	model := newModel(ShowcaseWorkspace(), nil, nil)
	model.enterDemoCampaign()
	var sessionID, entryID string
	targetIndex := -1
	for _, session := range model.workspace.Sessions {
		if session.EndedAt == nil {
			continue
		}
		_, frames, ok := domain.SessionPlayback(model.workspace, session.ID)
		if !ok {
			continue
		}
		for index, frame := range frames {
			if frame.EntryID == "" {
				continue
			}
			sessionID = session.ID
			entryID = frame.EntryID
			targetIndex = index
			break
		}
		if targetIndex >= 0 {
			break
		}
	}
	if targetIndex < 0 {
		t.Fatal("demo should provide an ended session playback frame with an entry ID")
	}
	updated, _ := model.jumpToHop(detailHop{Kind: hopHistory, SessionID: sessionID, EntryID: entryID, Label: "history target"})
	model = updated.(Model)
	if !model.playingBack {
		t.Fatalf("history follow should open playback, status=%q", model.status)
	}
	if model.playbackCursor != targetIndex {
		t.Fatalf("history follow cursor=%d, want %d", model.playbackCursor, targetIndex)
	}
	if len(model.browserHistory) != 1 {
		t.Fatalf("history follow should preserve the origin, history=%d", len(model.browserHistory))
	}
}

func TestSourceBreadcrumbKeepsLiteralSegments(t *testing.T) {
	model := New()
	trail := testANSI.ReplaceAllString(model.renderDetailTrailParts(
		"Sources",
		[]string{"Guide/../Volume", "Chapter/One"},
		"Hit/Name",
	), "")
	want := "PATH  The Ashen Crown / Sources / Guide/../Volume / Chapter/One / Hit/Name"
	if !strings.Contains(trail, want) {
		t.Fatalf("source breadcrumb should preserve literal segments: %q", trail)
	}
}

func TestLinkPreviewClickOutsideDismisses(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			model.selectRecord(record)
			break
		}
	}
	model.layout.Focus = prefs.PaneDetail
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview == nil {
		t.Fatal("expected preview")
	}
	updated, _ = model.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.preview != nil {
		t.Fatal("click outside should close preview")
	}
	if model.selectedID != "npc-captain-vale" {
		t.Fatalf("dismiss should keep place, selected=%q", model.selectedID)
	}
}

func TestPreviewOverlayHasBorderAndNoBackgroundPaint(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			model.selectRecord(record)
			break
		}
	}
	model.layout.Focus = prefs.PaneDetail
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.preview == nil {
		t.Fatal("expected preview")
	}
	frame := model.previewFrame()
	stripped := testANSI.ReplaceAllString(frame, "")
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("preview frame too short: %q", stripped)
	}
	top := lines[0]
	if !strings.ContainsAny(top, "╔═┌─") {
		t.Fatalf("expected a top border, first line %q", top)
	}
	bottom := lines[len(lines)-1]
	if !strings.ContainsAny(bottom, "╚═┘─") {
		t.Fatalf("expected a bottom border, last line %q", bottom)
	}
	// Color is reserved for authority, state, warning, and focus: painting a
	// surface behind the overlay caused terminal artifacts, so the frame must
	// carry no background at all.
	if strings.Contains(frame, "\x1b[48;2;") {
		t.Fatalf("preview must not paint a background, got %q", frame)
	}
}

func selectSessionRow(model *Model, id string) {
	for index, row := range model.sessionTreeRows() {
		if row.Kind == domain.SessionTreeSession && row.Session.ID == id {
			model.applySessionTreeCursor(index)
			return
		}
	}
}

var testANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func leadingPad(view, needle string) int {
	for _, line := range strings.Split(testANSI.ReplaceAllString(view, ""), "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		return len(line) - len(strings.TrimLeft(line, " "))
	}
	return -1
}

func TestImportOpensDedicatedScreenAndLoadsMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bst.md")
	markdown := "# Test Bestiary (2025)\n\n## Cave Rascals\n\nRaiders in packs.\n\n## Night Hunters\n\nHairy hunters who hunt at night.\n"
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	model := New()
	model.width = 100
	model.height = 32
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'I', Text: "I"}))
	model = updated.(Model)
	if !model.importing {
		t.Fatal("expected dedicated import screen")
	}
	if model.picking {
		t.Fatal("import should leave the library picker")
	}
	view := model.View().Content
	if !strings.Contains(view, "IMPORT") || !strings.Contains(view, "FILES") || !strings.Contains(view, "SOURCES") {
		t.Fatalf("expected import screen, got %q", view)
	}
	if !strings.Contains(view, "5E.TOOLS") {
		t.Fatalf("expected 5e.tools catalog pane: %q", view)
	}
	if strings.Contains(view, "Captain Vale") {
		t.Fatal("import should not render the campaign wiki")
	}

	model.importDir = dir
	model.reloadImportFiles()
	model.importFocus = "files"
	foundFile := false
	for index, entry := range model.importFiles {
		if entry.Name == "bst.md" {
			model.importFileCursor = index
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatalf("expected bst.md in file list: %#v", model.importFiles)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	model = flushCmd(t, model, cmd)
	if !model.importing {
		t.Fatal("import screen should stay open after a successful import")
	}
	if len(model.workspace.Sources) == 0 {
		t.Fatalf("expected a source document; status=%s", model.status)
	}
	found := false
	for _, record := range model.workspace.Records {
		if record.Title == "Cave Rascals" && record.Type == domain.Creature {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected imported Cave Rascals; status=%s records=%d", model.status, len(model.workspace.Records))
	}
	if !strings.Contains(model.View().Content, "Test Bestiary") {
		t.Fatalf("expected imported source listed: %q", model.View().Content)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.importing {
		t.Fatal("esc should leave the import screen")
	}
	if strings.Contains(model.View().Content, "Captain Vale") == false {
		t.Fatalf("expected to return to campaign wiki: %q", model.View().Content)
	}
}

func TestImportRemoveSourceAllowsReingest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bst.md")
	markdown := "# Test Bestiary (2025)\n\n## Cave Rascals\n\nRaiders in packs.\n"
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	model := New()
	model.width = 100
	model.height = 32
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'I', Text: "I"}))
	model = updated.(Model)
	model.importDir = dir
	model.reloadImportFiles()
	model.importFocus = "files"
	for index, entry := range model.importFiles {
		if entry.Name == "bst.md" {
			model.importFileCursor = index
			break
		}
	}
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	model = flushCmd(t, model, cmd)
	if len(model.workspace.Sources) != 1 {
		t.Fatalf("expected one source after ingest, got %d status=%s", len(model.workspace.Sources), model.status)
	}
	sourceID := model.workspace.Sources[0].ID
	model.importFocus = "sources"
	model.importSourceCursor = 0
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Text: "d"}))
	model = updated.(Model)
	if !model.deleteConfirm || model.confirmKind != "delete-source" {
		t.Fatalf("expected remove confirm, got confirm=%v kind=%q", model.deleteConfirm, model.confirmKind)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"}))
	model = updated.(Model)
	if model.deleteConfirm {
		t.Fatal("confirm should clear after remove")
	}
	if len(model.workspace.Sources) != 0 {
		t.Fatalf("source document should be gone: %#v", model.workspace.Sources)
	}
	for _, record := range model.workspace.Records {
		if record.SourceID == sourceID {
			t.Fatalf("ingested record still present: %#v", record)
		}
	}
	if !strings.Contains(model.status, "Removed") {
		t.Fatalf("status=%s", model.status)
	}

	model.importFocus = "files"
	for index, entry := range model.importFiles {
		if entry.Name == "bst.md" {
			model.importFileCursor = index
			break
		}
	}
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	model = flushCmd(t, model, cmd)
	if len(model.workspace.Sources) != 1 {
		t.Fatalf("re-ingest should restore the source, got %d status=%s", len(model.workspace.Sources), model.status)
	}
	found := false
	for _, record := range model.workspace.Records {
		if record.Title == "Cave Rascals" && record.SourceID == model.workspace.Sources[0].ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("re-ingest should restore Cave Rascals")
	}
}

func TestImportShowsProgressWhileBusy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bst.md")
	if err := os.WriteFile(path, []byte("# Test Bestiary (2025)\n\n## Cave Rascals\n\nRaiders in packs.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := New()
	model.width = 100
	model.height = 32
	model.store = storage.NewJSON(filepath.Join(dir, "workspace.json"))
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'I', Text: "I"}))
	model = updated.(Model)
	model.importDir = dir
	model.reloadImportFiles()
	model.importFocus = "files"
	for index, entry := range model.importFiles {
		if entry.Name == "bst.md" {
			model.importFileCursor = index
			break
		}
	}
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.importBusy {
		t.Fatal("expected ingest to mark the import screen busy")
	}
	view := model.View().Content
	if !strings.Contains(view, "INGESTING") || !strings.Contains(view, "bst.md") {
		t.Fatalf("expected live ingest progress, got %q", view)
	}
	model = flushCmd(t, model, cmd)
	if model.importBusy {
		t.Fatal("ingest should finish")
	}
	if !strings.Contains(model.status, "Imported") && !strings.Contains(model.status, "records") {
		t.Fatalf("expected finished import status, got %q", model.status)
	}
}

func TestImportFromLibraryPickerReturnsToPicker(t *testing.T) {
	model := newModel(demoWorkspace(), nil, nil)
	model.width = 80
	model.height = 24
	model.openPicker()
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'I', Text: "I"}))
	model = updated.(Model)
	if !model.importing || model.picking {
		t.Fatalf("importing=%v picking=%v", model.importing, model.picking)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.importing || !model.picking {
		t.Fatalf("esc should return to picker; importing=%v picking=%v", model.importing, model.picking)
	}
	if !strings.Contains(model.View().Content, "WORLDS") {
		t.Fatalf("expected library picker: %q", model.View().Content)
	}
}

func TestImportFiveEToolsCatalogIngestsSelectedBook(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 32
	model.toolsFetcher = adventureTestFetcher()
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'I', Text: "I"}))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected catalog load command")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	view := model.View().Content
	if !strings.Contains(view, "The Hollow Crown") || !strings.Contains(view, "5E.TOOLS") {
		t.Fatalf("expected 5e.tools catalog: %q", view)
	}
	if strings.Contains(view, "Test Bestiary") || strings.Contains(view, "harness") {
		t.Fatalf("catalog should list adventures only, no harness chrome: %q", view)
	}
	model.importFocus = "tools"
	found := false
	for index, entry := range model.filteredTools() {
		if entry.ID == "ADV" {
			model.importToolsCursor = index
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected ADV in catalog")
	}
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected ingest command")
	}
	model = flushCmd(t, model, cmd)
	for _, record := range model.workspace.Records {
		if record.SourceID == "src-5e-adv" {
			t.Fatalf("adventure leaked into wiki: %#v", record)
		}
	}
	if !strings.Contains(model.status, "reference") {
		t.Fatalf("status should mention reference: %s", model.status)
	}
	enabled := false
	for _, id := range model.workspace.EnabledSourceIDs(model.workspace.Scope) {
		if id == "src-5e-adv" {
			enabled = true
			break
		}
	}
	if !enabled {
		t.Fatal("ingested adventure should be enabled for the campaign")
	}
	rec, ok := model.lookupReference("Mira Holt")
	if !ok || rec.Title != "Mira Holt" || !fivetools.IsReferenceID(rec.ID) {
		t.Fatalf("expected adventure lookup after ingest; status=%s ok=%v %#v", model.status, ok, rec)
	}

	model.setNavCursor(2)
	reader := model.View().Content
	if !strings.Contains(reader, "The Hollow Crown") || !strings.Contains(reader, "SOURCE") {
		t.Fatalf("expected Sources reader: %q", reader)
	}

	model.refreshResults()
	model.searchInput.SetValue("Mira")
	model.refreshResults()
	labeled := false
	for _, result := range model.results {
		if fivetools.IsReferenceID(result.Record.ID) && result.Record.Title == "Mira Holt" {
			labeled = true
			break
		}
	}
	if !labeled {
		t.Fatalf("search should include reference hit for Mira Holt: %#v", model.results)
	}
}

func TestImportedSourceRecordsCollapseIntoFolders(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 32
	model.workspace.Sources = append(model.workspace.Sources, domain.SourceDocument{
		ID: "src-5e-bst", Title: "Test Bestiary (2025)", Kind: domain.SourceBestiary,
	})
	model.workspace.EnableSource(model.workspace.Scope.WorldID, model.workspace.Scope.CampaignID, "src-5e-bst")
	for i := 0; i < 40; i++ {
		title := fmt.Sprintf("Monster %02d", i)
		model.workspace.Records = append(model.workspace.Records, domain.Record{
			ID:        fmt.Sprintf("src-5e-bst-creature-%d", i),
			Type:      domain.Creature,
			Title:     title,
			SourceID:  "src-5e-bst",
			Source:    "Test Bestiary (2025)",
			Folder:    "Test Bestiary (2025)/Creatures",
			Authority: domain.Canon,
		})
	}
	model.setNavCursor(8) // Creatures
	view := model.View().Content
	if !strings.Contains(view, "Test Bestiary (2025)") || !strings.Contains(view, "· 40") {
		t.Fatalf("expected collapsed source folder, got %q", view)
	}
	if strings.Contains(view, "Monster 00") {
		t.Fatal("collapsed source folder should not dump creature rows into the campaign list")
	}
}

func TestRecordTreeCursorMovesToNextCreature(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 32
	model.workspace.Sources = append(model.workspace.Sources, domain.SourceDocument{
		ID: "src-5e-bst", Title: "Test Bestiary (2025)", Kind: domain.SourceBestiary,
	})
	model.workspace.EnableSource(model.workspace.Scope.WorldID, model.workspace.Scope.CampaignID, "src-5e-bst")
	for i := 0; i < 5; i++ {
		title := fmt.Sprintf("Monster %02d", i)
		model.workspace.Records = append(model.workspace.Records, domain.Record{
			ID:        fmt.Sprintf("src-5e-bst-creature-%d", i),
			Type:      domain.Creature,
			Title:     title,
			SourceID:  "src-5e-bst",
			Source:    "Test Bestiary (2025)",
			Folder:    "Test Bestiary (2025)/Creatures",
			Authority: domain.Canon,
		})
	}
	model.setNavCursor(8) // Creatures
	model.expandRecordFolderPath("Test Bestiary (2025)/Creatures")
	model.setBrowserFocus(prefs.PaneList)

	rows := model.recordTreeRows()
	first := -1
	second := -1
	for index := 0; index+1 < len(rows); index++ {
		if rows[index].Kind == domain.RecordTreeRecord && rows[index+1].Kind == domain.RecordTreeRecord {
			first = index
			second = index + 1
			break
		}
	}
	if first < 0 || second < 0 {
		t.Fatalf("expected two creature rows after expanding folders, got %#v", rows)
	}

	model.applyRecordTreeCursor(first)
	if model.selectedID != rows[first].Record.ID {
		t.Fatalf("expected first creature %q, got %q", rows[first].Record.ID, model.selectedID)
	}
	if model.cursor != first {
		t.Fatalf("cursor should stay on tree row %d, got %d", first, model.cursor)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model = updated.(Model)
	if model.cursor != second {
		t.Fatalf("j should move to the next creature row %d, got %d (selected %q)", second, model.cursor, model.selectedID)
	}
	if model.selectedID != rows[second].Record.ID {
		t.Fatalf("j selected %q, want %q", model.selectedID, rows[second].Record.ID)
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	model = updated.(Model)
	if model.cursor != first || model.selectedID != rows[first].Record.ID {
		t.Fatalf("k should return to row %d (%q), got cursor=%d selected=%q", first, rows[first].Record.ID, model.cursor, model.selectedID)
	}
}

func TestWindowLinesScrollsInsteadOfClippingFromTop(t *testing.T) {
	got, used := windowLines("a\nb\nc\nd", 2, 1)
	if got != "b\nc" || used != 1 {
		t.Fatalf("got %q used=%d", got, used)
	}
	got, used = windowLines("a\nb", 5, 9)
	if got != "a\nb" || used != 0 {
		t.Fatalf("short content should ignore scroll: %q used=%d", got, used)
	}
	got, used = windowLines("a\nb\nc\nd", 2, 99)
	if got != "c\nd" || used != 2 {
		t.Fatalf("scroll should clamp to last window: %q used=%d", got, used)
	}
}

func longOverflowBody() string {
	var body strings.Builder
	body.WriteString("DETAIL_HEAD_MARKER\n\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&body, "overflow line %d\n\n", i)
	}
	body.WriteString("DETAIL_TAIL_MARKER\n")
	return body.String()
}

func TestDetailPaneScrollsWhenContentOverflows(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 16
	record := domain.Record{
		ID:        "note-overflow",
		Type:      domain.Note,
		Title:     "Overflow Note",
		Body:      longOverflowBody(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "test",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	model.layout.Focus = prefs.PaneDetail

	h := &Harness{Model: model}
	frame := h.Frame()
	if !frame.Contains("DETAIL_HEAD_MARKER") {
		t.Fatalf("expected head in detail pane:\n%s", frame.Plain)
	}
	if frame.Contains("DETAIL_TAIL_MARKER") {
		t.Fatalf("tail should be below the pane until scroll:\n%s", frame.Plain)
	}
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("layout fill failed: %s\n%s", strings.Join(errs, "; "), frame.Plain)
	}

	var scrolled Frame
	found := false
	for i := 0; i < 24; i++ {
		scrolled = h.Key("pgdown")
		if scrolled.Contains("DETAIL_TAIL_MARKER") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tail after PgDn:\n%s", scrolled.Plain)
	}
	if scrolled.Contains("DETAIL_HEAD_MARKER") {
		t.Fatalf("head should scroll away:\n%s", scrolled.Plain)
	}
	if errs := scrolled.FillErrors(); len(errs) > 0 {
		t.Fatalf("scrolled layout fill failed: %s\n%s", strings.Join(errs, "; "), scrolled.Plain)
	}

	top := h.Key("home")
	if !top.Contains("DETAIL_HEAD_MARKER") || top.Contains("DETAIL_TAIL_MARKER") {
		t.Fatalf("Home should return to the top:\n%s", top.Plain)
	}

	h.Key("end")
	if h.Model.selectedRecord() == nil {
		t.Fatal("expected selected overflow note")
	}
	other := model.workspace.Records[0]
	h.Model.selectRecord(other)
	reset := h.Frame()
	if reset.Contains("DETAIL_TAIL_MARKER") {
		t.Fatalf("changing record should reset detail scroll:\n%s", reset.Plain)
	}
}

func TestDetailBodyViewInvalidatesWhenRecordChanges(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 24
	record := domain.Record{
		ID:        "detail-cache",
		Type:      domain.Note,
		Title:     "Cached Detail",
		Body:      "CACHE_HEAD\n\nA long line that must still fit inside the detail pane.",
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "test",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)

	first, firstLines := model.detailBodyView(36)
	firstPlain := testANSI.ReplaceAllString(first, "")
	if !strings.Contains(firstPlain, "CACHE_HEAD") {
		t.Fatalf("expected initial detail body: %q", first)
	}
	for index, line := range firstLines {
		if width := lipgloss.Width(line); width > 36 {
			t.Fatalf("initial line %d width %d > 36: %q", index, width, line)
		}
	}

	for index := range model.workspace.Records {
		if model.workspace.Records[index].ID == record.ID {
			model.workspace.Records[index].Body = "CACHE_UPDATED\n\nUpdated detail text."
			break
		}
	}
	updated, updatedLines := model.detailBodyView(36)
	updatedPlain := testANSI.ReplaceAllString(updated, "")
	if updated == first || strings.Contains(updatedPlain, "CACHE_HEAD") || !strings.Contains(updatedPlain, "CACHE_UPDATED") {
		t.Fatalf("detail cache did not invalidate after body change: %q", updated)
	}
	for index, line := range updatedLines {
		if width := lipgloss.Width(line); width > 36 {
			t.Fatalf("updated line %d width %d > 36: %q", index, width, line)
		}
	}
}

func TestDetailPaneWheelScrollsWhenFocused(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 16
	record := domain.Record{
		ID:        "note-overflow-wheel",
		Type:      domain.Note,
		Title:     "Overflow Wheel",
		Body:      longOverflowBody(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "test",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	model.layout.Focus = prefs.PaneDetail
	h := &Harness{Model: model}
	h.Frame()
	before := h.Model.detailView.offset
	updated, _ := h.Model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	h.Model = updated.(Model)
	if h.Model.detailView.offset <= before {
		t.Fatalf("wheel should scroll detail when focused, offset %d -> %d", before, h.Model.detailView.offset)
	}
}

func TestDetailPgDnDoesNotMoveHopCursor(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	if rec := model.selectedRecord(); rec == nil {
		t.Fatal("expected demo record")
	}
	model.layout.Focus = prefs.PaneDetail
	if len(model.detailHops()) == 0 {
		t.Fatal("expected hops on demo record")
	}
	model.historyCursor = 0
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	model = updated.(Model)
	if model.historyCursor != 0 {
		t.Fatalf("PgDn should scroll the body, not hops, got cursor %d", model.historyCursor)
	}
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	model = updated.(Model)
	if model.historyCursor != 1 {
		t.Fatalf("j should still move hop cursor, got %d", model.historyCursor)
	}
}

func liveSessionModel(t *testing.T) Model {
	t.Helper()
	model := New()
	model.width = 120
	model.height = 40
	updated, _ := model.startSession()
	model = updated.(Model)
	if model.session == nil {
		t.Fatal("expected a live session")
	}
	return model
}

func TestLiveSearchOpensWithoutEndingCapture(t *testing.T) {
	model := liveSessionModel(t)
	model.sessionInput.SetValue("the party enters")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	model = updated.(Model)

	if !model.searching {
		t.Fatal("/ should open search during a live session")
	}
	if model.session == nil {
		t.Fatal("search must not end live capture")
	}
	if model.sessionInput.Value() != "the party enters" {
		t.Fatalf("captured input should survive search: %q", model.sessionInput.Value())
	}
	if strings.Contains(model.renderSearchOverlay(), "Esc close") {
		t.Fatal("live search footer should return to capture, not close")
	}
	if !strings.Contains(model.renderSearchOverlay(), "Esc back to capture") {
		t.Fatal("expected live search footer wording")
	}
}

func TestLiveSearchEscapeRestoresCaptureFocus(t *testing.T) {
	model := liveSessionModel(t)
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	model = updated.(Model)

	if model.searching {
		t.Fatal("esc should close live search")
	}
	if model.session == nil {
		t.Fatal("esc must not end the session")
	}
	if !model.sessionInput.Focused() {
		t.Fatal("capture input should regain focus after live search")
	}
}

func TestLiveSearchResultInspectsInPreview(t *testing.T) {
	model := liveSessionModel(t)
	model.searching = true
	model.sessionInput.Blur()
	model.searchInput.SetValue("vale")
	model.refreshResults()
	if len(model.results) == 0 {
		t.Fatal("expected search hits for the fixture campaign")
	}
	before := len(model.session.Entries)

	updated, _ := model.openSearchResult(model.results[0])
	model = updated.(Model)

	if model.searching {
		t.Fatal("opening a result should close the search overlay")
	}
	if model.preview == nil {
		t.Fatal("a live search result should open a read-only preview")
	}
	if model.session == nil {
		t.Fatal("inspecting a result must not end live capture")
	}
	if len(model.session.Entries) != before {
		t.Fatal("inspection must not write transcript entries")
	}
	if !strings.Contains(model.View().Content, "SESSION") {
		t.Fatal("preview should float over the live session view")
	}
}

func TestLivePreviewEnterReturnsToCapture(t *testing.T) {
	model := liveSessionModel(t)
	model.searching = true
	model.sessionInput.Blur()
	model.searchInput.SetValue("vale")
	model.refreshResults()
	updated, _ := model.openSearchResult(model.results[0])
	model = updated.(Model)
	if model.preview == nil {
		t.Fatal("expected a preview to inspect")
	}
	navCursor := model.navCursor

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)

	if model.preview != nil {
		t.Fatal("enter should dismiss the live preview")
	}
	if model.session == nil {
		t.Fatal("enter must not navigate away from live capture")
	}
	if model.navCursor != navCursor {
		t.Fatal("live preview must not move the browser navigator")
	}
	if !model.sessionInput.Focused() {
		t.Fatal("capture input should regain focus after inspection")
	}
}

func TestLiveSearchRefusesReconciliationResults(t *testing.T) {
	model := liveSessionModel(t)
	if _, ok := model.searchResultPreviewHop(searchsvc.Result{Kind: searchsvc.KindRecon}); ok {
		t.Fatal("reconciliation review is not inspectable during live capture")
	}
	updated, _ := model.openSearchResult(searchsvc.Result{Kind: searchsvc.KindRecon})
	model = updated.(Model)
	if model.preview != nil || model.reconciling {
		t.Fatal("reconciliation must not open during live capture")
	}
	if model.session == nil {
		t.Fatal("refusing a result must not end the session")
	}
	if !strings.Contains(model.status, "End live capture") {
		t.Fatalf("expected an explanatory status, got %q", model.status)
	}
}
