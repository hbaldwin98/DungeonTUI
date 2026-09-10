package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

func threadRecord(model Model, id, title string, state domain.ThreadState, body string) domain.Record {
	return domain.Record{
		ID: id, Type: domain.Thread, Title: title, Authority: domain.Canon,
		Scope: model.workspace.Scope, Source: "notes", ThreadState: state, Body: body,
	}
}

func endedCampaignSit(model Model, id string, day int, links ...string) domain.SessionRecord {
	start := time.Date(2026, 5, day, 20, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)
	sit := domain.SessionRecord{ID: id, Title: "Sit " + id, Scope: model.workspace.Scope, StartedAt: start, EndedAt: &end}
	for _, link := range links {
		sit.Entries = append(sit.Entries, domain.TranscriptEntry{
			ID: id + "-" + link, Text: "Play touched " + link, CreatedAt: start.Add(time.Hour),
			Links: []domain.EntityLink{{RecordID: link}},
		})
	}
	return sit
}

func TestHomeListsActiveThreadsByUrgencyAndFlagsNeglect(t *testing.T) {
	model := New()
	model.width, model.height = 120, 40
	model.workspace.Records = append(model.workspace.Records,
		threadRecord(model, "thread-hot", "Hot Pursuit", domain.ThreadAdvancing, "## Next beats\n- The fence runs"),
		threadRecord(model, "thread-stale", "Stale Rumor", domain.ThreadOpen, ""),
		threadRecord(model, "thread-done", "Settled Feud", domain.ThreadResolved, ""),
	)
	model.workspace.Sessions = append(model.workspace.Sessions,
		endedCampaignSit(model, "s1", 1, "thread-stale"),
		endedCampaignSit(model, "s2", 2, "thread-hot"),
		endedCampaignSit(model, "s3", 3),
	)
	home := stripANSIForTest(model.renderCampaignHome())
	if !strings.Contains(home, "1 neglected") {
		t.Fatalf("neglect should be counted in the heading: %q", home)
	}
	hot := strings.Index(home, "Hot Pursuit")
	stale := strings.Index(home, "Stale Rumor")
	if hot < 0 || stale < 0 || hot > stale {
		t.Fatalf("advancing sorts above open: %q", home)
	}
	if !strings.Contains(home, "quiet 2 sits") || !strings.Contains(home, "next: The fence runs") {
		t.Fatalf("rows should say why they sort and what comes next: %q", home)
	}
	if strings.Contains(home, "Settled Feud") {
		t.Fatalf("resolved threads leave the active list: %q", home)
	}
	if strings.Contains(home, "Church Investigator Arrives") {
		t.Fatalf("an AI proposal must never read as a tracked thread: %q", home)
	}
}

func TestHomeThreadEmptyStateSaysWhereResolvedThreadsWent(t *testing.T) {
	model := New()
	model.width, model.height = 100, 30
	for index := range model.workspace.Records {
		if model.workspace.Records[index].Type == domain.Thread && model.workspace.Records[index].Authority != domain.Proposal {
			model.workspace.Records[index].ThreadState = domain.ThreadResolved
		}
	}
	home := stripANSIForTest(model.renderCampaignHome())
	if !strings.Contains(home, "ACTIVE THREADS · 0") || !strings.Contains(home, "resolved ones stay in Threads") {
		t.Fatalf("empty state = %q", home)
	}
}

func TestPaletteTransitionsTheSelectedThreadAndKeepsHistory(t *testing.T) {
	model := New()
	model.width, model.height = 120, 40
	var before domain.Record
	for _, record := range model.workspace.Records {
		if record.ID == "thread-reliquary" {
			before = record
			model.selectRecord(record)
		}
	}
	updated, _ := model.setSelectedThreadState(domain.ThreadResolved)
	model = updated.(Model)
	var after domain.Record
	for _, record := range model.workspace.Records {
		if record.ID == "thread-reliquary" {
			after = record
		}
	}
	if domain.EffectiveThreadState(after) != domain.ThreadResolved {
		t.Fatalf("state = %s", domain.EffectiveThreadState(after))
	}
	if after.Body != before.Body || after.Authority != before.Authority || after.Summary != before.Summary {
		t.Fatal("a transition must change only the state")
	}
	if !strings.Contains(model.status, "open → resolved") || !strings.Contains(model.status, "history kept") {
		t.Fatalf("status = %q", model.status)
	}

	again, _ := model.setSelectedThreadState(domain.ThreadResolved)
	if !strings.Contains(again.(Model).status, "already resolved") {
		t.Fatalf("a no-op transition says so: %q", again.(Model).status)
	}

	// A resolved thread still opens with its full detail.
	detail := stripANSIForTest(model.renderDetail())
	if !strings.Contains(detail, "Missing Reliquary") || !strings.Contains(detail, "Vale suspects @Father Merrow") {
		t.Fatalf("resolved thread detail lost its history: %q", detail)
	}
	if !strings.Contains(detail, "THREAD  ● CANON  · resolved") {
		t.Fatalf("detail should say the thread is resolved: %q", detail)
	}
}

func TestThreadTransitionsRefuseProposalsAndNonThreads(t *testing.T) {
	model := New()
	for _, record := range model.workspace.Records {
		if record.ID == "proposal-investigator" {
			model.selectRecord(record)
		}
	}
	updated, _ := model.setSelectedThreadState(domain.ThreadAdvancing)
	if !strings.Contains(updated.(Model).status, "Accept the AI proposal") {
		t.Fatalf("status = %q", updated.(Model).status)
	}
	for _, record := range model.workspace.Records {
		if record.ID == "proposal-investigator" && record.ThreadState != "" {
			t.Fatal("a proposal must not gain a state")
		}
	}

	for _, record := range model.workspace.Records {
		if record.ID == "npc-captain-vale" {
			model.selectRecord(record)
		}
	}
	commands := model.paletteCommands()
	for _, command := range commands {
		if command.ID == "thread.resolved" && commandEnabled(command) {
			t.Fatal("thread commands are disabled with a reason when an NPC is selected")
		}
		if command.ID == "thread.resolved" && command.Reason != "Select a thread" {
			t.Fatalf("reason = %q", command.Reason)
		}
	}
}

func TestEditorStateLineIsAnExplicitTransition(t *testing.T) {
	model := New()
	model.width, model.height = 120, 40
	for _, record := range model.workspace.Records {
		if record.ID == "thread-reliquary" {
			model.selectRecord(record)
		}
	}
	updated, _ := model.openEditor(false)
	model = updated.(Model)
	doc := model.editBody.Value()
	model.editBody.SetValue(strings.Replace(doc, "type: THREAD\n", "type: THREAD\nstate: dormant\n", 1))
	updated, _ = model.saveEditor()
	model = updated.(Model)
	for _, record := range model.workspace.Records {
		if record.ID == "thread-reliquary" && record.ThreadState != domain.ThreadDormant {
			t.Fatalf("editor state line should persist: %#v (status %q)", record.ThreadState, model.status)
		}
	}
}

func TestPrepDetailShowsThreadsItsCastCarries(t *testing.T) {
	model := New()
	model.width, model.height = 120, 40
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, domain.PlannedNotes{
		ID: "plan-crypt", Title: "Crypt", Scope: model.workspace.Scope,
		Links: []domain.EntityLink{{RecordID: "npc-father-merrow", Text: "Father Merrow"}},
	})
	model.focusPrep("plan-crypt")
	detail := stripANSIForTest(model.renderTreeDetailWidth(60))
	if !strings.Contains(detail, "THREADS IN THIS PREP") || !strings.Contains(detail, "Missing Reliquary") {
		t.Fatalf("Merrow is named in the reliquary thread: %q", detail)
	}

	model.workspace.PlannedNotes[len(model.workspace.PlannedNotes)-1].Links = nil
	detail = stripANSIForTest(model.renderTreeDetailWidth(60))
	if !strings.Contains(detail, "none touch this cast · 1 active on Home") {
		t.Fatalf("an unrelated prep points to Home: %q", detail)
	}
}

func TestLiveContextLeadsWithThreadsTheSceneCarries(t *testing.T) {
	model := New()
	model.width, model.height = 120, 40
	model.workspace.Records = append(model.workspace.Records,
		threadRecord(model, "thread-a", "Alpha Urgent", domain.ThreadAdvancing, ""),
		threadRecord(model, "thread-done", "Settled", domain.ThreadResolved, "@Captain Vale"),
	)
	updated, _ := model.startSession()
	model = updated.(Model)
	model.session.Links = append(model.session.Links, domain.EntityLink{RecordID: "npc-father-merrow", Text: "Father Merrow"})

	statuses := model.sessionThreadStatuses()
	if len(statuses) < 2 || statuses[0].Record.ID != "thread-reliquary" {
		t.Fatalf("the thread naming the present cast leads, above a more urgent unrelated one: %#v", statuses)
	}
	for _, status := range statuses {
		if status.Record.ID == "thread-done" || status.Record.Authority == domain.Proposal {
			t.Fatalf("live context shows neither resolved threads nor proposals: %#v", status.Record)
		}
	}
	lines := model.contextContentLines()
	found := false
	for _, line := range lines {
		if strings.Contains(line.Text, "Missing Reliquary · open") {
			found = true
		}
	}
	if !found {
		t.Fatal("live thread rows should carry their state")
	}
}

func TestPostSessionReviewProposesThreadAdvanceForOwnerApproval(t *testing.T) {
	model := New()
	model.width, model.height = 120, 40
	updated, _ := model.startSession()
	model = updated.(Model)
	model.sessionInput.SetValue("The @Missing Reliquary surfaces at the docks")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	updated, _ = model.endSession()
	model = updated.(Model)

	var item domain.ReconciliationItem
	for _, recon := range model.workspace.Reconciliations {
		for _, candidate := range recon.Items {
			if candidate.Kind == domain.ReconThreadAdvance {
				item = candidate
			}
		}
	}
	if item.ID == "" || item.RecordID != "thread-reliquary" || item.Status != domain.ReconPending {
		t.Fatalf("ending play should queue a pending thread proposal: %#v", model.workspace.Reconciliations)
	}
	for _, record := range model.workspace.Records {
		if record.ID == "thread-reliquary" && domain.EffectiveThreadState(record) != domain.ThreadOpen {
			t.Fatal("no thread changes state until the owner accepts")
		}
	}
	if !strings.Contains(model.reconciliationProvenance(item), "thread open") {
		t.Fatalf("evidence should show the thread's current state: %q", model.reconciliationProvenance(item))
	}
}
