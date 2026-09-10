package domain

import (
	"strings"
	"testing"
	"time"
)

func threadScope() Scope {
	return Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
}

func endedSit(id string, day int, links ...string) SessionRecord {
	start := time.Date(2026, 4, day, 20, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)
	sit := SessionRecord{ID: id, Title: "Sit " + id, Scope: threadScope(), StartedAt: start, EndedAt: &end}
	for _, link := range links {
		sit.Entries = append(sit.Entries, TranscriptEntry{
			ID: id + "-" + link, Text: "The party pressed on " + link, CreatedAt: start.Add(time.Hour),
			Links: []EntityLink{{RecordID: link}},
		})
	}
	return sit
}

func TestEffectiveThreadStatePrefersExplicitThenTagThenOpen(t *testing.T) {
	cases := []struct {
		record Record
		want   ThreadState
	}{
		{Record{Type: Thread, ThreadState: ThreadDormant, Tags: []string{"open"}}, ThreadDormant},
		{Record{Type: Thread, Tags: []string{"Resolved"}}, ThreadResolved},
		{Record{Type: Thread, Tags: []string{"open"}}, ThreadOpen},
		{Record{Type: Thread}, ThreadOpen},
		{Record{Type: Thread, ThreadState: "nonsense"}, ThreadOpen},
	}
	for _, test := range cases {
		if got := EffectiveThreadState(test.record); got != test.want {
			t.Fatalf("%#v = %s, want %s", test.record, got, test.want)
		}
	}
}

func TestSetThreadStateIsExplicitAndKeepsHistory(t *testing.T) {
	body := "Vale suspects @Merrow.\n\n> Session 3: the reliquary was seen at the docks\n"
	thread := Record{ID: "t", Type: Thread, Title: "Reliquary", Authority: Canon, Body: body, Tags: []string{"open"}}
	resolved, err := SetThreadState(thread, ThreadResolved)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Body != body || strings.Join(resolved.Tags, ",") != "open" || resolved.Authority != Canon {
		t.Fatalf("resolving must change only the state: %#v", resolved)
	}
	if EffectiveThreadState(resolved) != ThreadResolved {
		t.Fatal("explicit state must win over a legacy open tag")
	}
	if _, err := SetThreadState(Record{Type: NPC, Title: "Vale"}, ThreadOpen); err == nil {
		t.Fatal("only threads have states")
	}
	if _, err := SetThreadState(Record{Type: Thread, Title: "Idea", Authority: Proposal}, ThreadOpen); err == nil {
		t.Fatal("an AI proposal cannot be tracked until accepted")
	}
	if _, err := SetThreadState(thread, "bogus"); err == nil {
		t.Fatal("unknown states are refused")
	}
}

func TestParseThreadSectionsReadsStakesBeatsAndDevelopment(t *testing.T) {
	body := "Opening prose.\n\n## Stakes\nThe monastery falls if the relic is sold.\n\n## Next Beats\n- Merrow flees at dawn\n* A fence surfaces at the docks\n\n## Last development\nThe seal cracked.\n\n## Notes\n- not a beat\n"
	sections := ParseThreadSections(body)
	if sections.Stakes != "The monastery falls if the relic is sold." {
		t.Fatalf("stakes = %q", sections.Stakes)
	}
	if len(sections.NextBeats) != 2 || sections.NextBeats[1] != "A fence surfaces at the docks" {
		t.Fatalf("beats = %#v", sections.NextBeats)
	}
	if sections.LastDevelopment != "The seal cracked." {
		t.Fatalf("last development = %q", sections.LastDevelopment)
	}
}

func TestThreadStateRoundTripsThroughMarkdown(t *testing.T) {
	thread := Record{Type: Thread, Title: "Reliquary", Authority: Canon, ThreadState: ThreadDormant, Body: "## Stakes\nHigh.\n"}
	parsed, err := ParseEntityMarkdown(FormatEntityMarkdown(thread), Note)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Record.ThreadState != ThreadDormant || !strings.Contains(parsed.Record.Body, "## Stakes") {
		t.Fatalf("round trip = %#v", parsed.Record)
	}
	if _, err := ParseEntityMarkdown("type: THREAD\nstate: someday\n\n# X\n", Note); err == nil {
		t.Fatal("an unknown state should be an editor error, not silent data")
	}
	npc, err := ParseEntityMarkdown("type: NPC\nstate: resolved\n\n# Vale\n", Note)
	if err != nil || npc.Record.ThreadState != "" {
		t.Fatalf("state is dropped on non-threads: %#v %v", npc.Record, err)
	}
	if strings.Contains(FormatEntityMarkdown(Record{Type: Thread, Title: "Legacy"}), "state:") {
		t.Fatal("a legacy thread should not grow a state line it never had")
	}
}

func TestDeriveThreadStatusUsesLinkedSessionsForLastDevelopment(t *testing.T) {
	scope := threadScope()
	workspace := Workspace{
		Scope: scope,
		Records: []Record{
			{ID: "t", Type: Thread, Title: "Reliquary", Authority: Canon, Scope: scope, Body: "Involves @Vale.\n\n## Last development\nOwner note.\n"},
			{ID: "vale", Type: NPC, Title: "Vale", Authority: Canon, Scope: scope},
		},
		Sessions: []SessionRecord{endedSit("a", 1, "t"), endedSit("b", 2), endedSit("c", 3)},
	}
	status := DeriveThreadStatus(workspace, workspace.Records[0], scope)
	if status.LastDevelopment != "The party pressed on t" || status.LastSessionID != "a" {
		t.Fatalf("transcript development should win: %#v", status)
	}
	if status.QuietSits != 2 || !status.Neglected {
		t.Fatalf("two quiet sits means neglected: %#v", status)
	}
	if len(status.Involved) != 1 || status.Involved[0].ID != "vale" {
		t.Fatalf("involved = %#v", status.Involved)
	}
	if !strings.Contains(status.Attention(), "quiet 2 sits") {
		t.Fatalf("attention = %q", status.Attention())
	}

	untouched := DeriveThreadStatus(Workspace{Scope: scope, Records: workspace.Records}, workspace.Records[0], scope)
	if untouched.LastDevelopment != "Owner note." || !untouched.NeverInPlay || untouched.Neglected {
		t.Fatalf("with no play the owner's section stands and nothing is neglected: %#v", untouched)
	}
}

func TestDormantThreadsAreNeverNeglected(t *testing.T) {
	scope := threadScope()
	workspace := Workspace{
		Scope:    scope,
		Records:  []Record{{ID: "t", Type: Thread, Title: "Parked", Authority: Canon, Scope: scope, ThreadState: ThreadDormant}},
		Sessions: []SessionRecord{endedSit("a", 1, "t"), endedSit("b", 2), endedSit("c", 3), endedSit("d", 4)},
	}
	if DeriveThreadStatus(workspace, workspace.Records[0], scope).Neglected {
		t.Fatal("dormant is a deliberate park, not neglect")
	}
}

func TestCampaignThreadsSortByUrgencyAndHideProposalsAndResolved(t *testing.T) {
	scope := threadScope()
	workspace := Workspace{
		Scope: scope,
		Records: []Record{
			{ID: "dormant", Type: Thread, Title: "A Dormant", Authority: Canon, Scope: scope, ThreadState: ThreadDormant},
			{ID: "fresh", Type: Thread, Title: "B Fresh", Authority: Canon, Scope: scope},
			{ID: "stale", Type: Thread, Title: "C Stale", Authority: Canon, Scope: scope},
			{ID: "hot", Type: Thread, Title: "D Hot", Authority: Canon, Scope: scope, ThreadState: ThreadAdvancing},
			{ID: "done", Type: Thread, Title: "E Done", Authority: Canon, Scope: scope, ThreadState: ThreadResolved},
			{ID: "ai", Type: Thread, Title: "F Idea", Authority: Proposal, Scope: scope, IsAIContent: true, Body: "## Next beats\n- invented"},
			{ID: "old", Type: Thread, Title: "G Old", Authority: Superseded, Scope: scope},
		},
		Sessions: []SessionRecord{endedSit("a", 1, "stale"), endedSit("b", 2, "hot"), endedSit("c", 3, "fresh"), endedSit("d", 4)},
	}
	var got []string
	for _, thread := range CampaignThreads(workspace, scope, false) {
		got = append(got, thread.Record.ID)
	}
	if strings.Join(got, ",") != "hot,stale,fresh,dormant" {
		t.Fatalf("urgency order = %v", got)
	}
	all := CampaignThreads(workspace, scope, true)
	if len(all) != 5 || all[len(all)-1].Record.ID != "done" {
		t.Fatalf("resolved threads stay listable last: %#v", all)
	}
}

func TestThreadsTouchingFindsThreadsByInvolvedEntity(t *testing.T) {
	scope := threadScope()
	workspace := Workspace{
		Scope: scope,
		Records: []Record{
			{ID: "t1", Type: Thread, Title: "Reliquary", Authority: Canon, Scope: scope, Body: "@Vale knows."},
			{ID: "t2", Type: Thread, Title: "Docks", Authority: Canon, Scope: scope},
			{ID: "t3", Type: Thread, Title: "Closed", Authority: Canon, Scope: scope, Body: "@Vale", ThreadState: ThreadResolved},
			{ID: "vale", Type: NPC, Title: "Vale", Authority: Canon, Scope: scope},
		},
	}
	touching := ThreadsTouching(workspace, scope, []string{"vale", ""})
	if len(touching) != 1 || touching[0].Record.ID != "t1" {
		t.Fatalf("touching = %#v", touching)
	}
	if direct := ThreadsTouching(workspace, scope, []string{"t2"}); len(direct) != 1 {
		t.Fatalf("a thread linked directly touches itself: %#v", direct)
	}
}

func TestReconciliationProposesAdvancingANamedThreadOnlyOnce(t *testing.T) {
	scope := threadScope()
	records := []Record{
		{ID: "t", Type: Thread, Title: "Reliquary", Authority: Canon, Scope: scope, ThreadState: ThreadDormant},
		{ID: "hot", Type: Thread, Title: "Hot", Authority: Canon, Scope: scope, ThreadState: ThreadAdvancing},
		{ID: "ai", Type: Thread, Title: "Idea", Authority: Proposal, Scope: scope, IsAIContent: true},
	}
	sit := endedSit("a", 1, "t", "hot", "ai")
	sit.Entries = append(sit.Entries, TranscriptEntry{ID: "again", Text: "Again", Links: []EntityLink{{RecordID: "t"}}})
	recon := BuildSessionReconciliation(sit, records)
	var advance []ReconciliationItem
	for _, item := range recon.Items {
		if item.Kind == ReconThreadAdvance {
			advance = append(advance, item)
		}
	}
	if len(advance) != 1 || advance[0].RecordID != "t" || advance[0].Status != ReconPending {
		t.Fatalf("one pending proposal for the dormant thread only: %#v", advance)
	}
	if !strings.Contains(advance[0].Summary, "was dormant") {
		t.Fatalf("the proposal should say what it changes: %q", advance[0].Summary)
	}
	if EffectiveThreadState(records[0]) != ThreadDormant {
		t.Fatal("building the inbox must not change the thread")
	}

	approved, out, err := ApplyReconItem(advance[0], records, sit)
	if err != nil || approved.Status != ReconApproved {
		t.Fatalf("approve: %#v %v", approved, err)
	}
	if EffectiveThreadState(out[0]) != ThreadAdvancing || EffectiveThreadState(records[0]) != ThreadDormant {
		t.Fatal("only the accepted copy changes state")
	}

	edited := advance[0]
	edited.Mutation.Text = "resolved"
	_, out, err = ApplyReconItem(edited, records, sit)
	if err != nil || EffectiveThreadState(out[0]) != ThreadResolved {
		t.Fatalf("the owner may edit the proposed state: %v", err)
	}
	edited.Mutation.Text = "maybe"
	if _, _, err := ApplyReconItem(edited, records, sit); err == nil {
		t.Fatal("an edited nonsense state must be refused")
	}
}
