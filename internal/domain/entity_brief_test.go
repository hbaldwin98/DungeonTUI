package domain

import (
	"strings"
	"testing"
	"time"
)

func briefScope() Scope {
	return Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
}

// briefWorkspace builds one campaign with a live sit, an ended sit, prep, and
// a review queue, so every brief section has something truthful to derive.
func briefWorkspace() Workspace {
	scope := briefScope()
	older := time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC)
	ended := older.Add(3 * time.Hour)
	newer := older.Add(24 * time.Hour)
	return Workspace{
		Scope: scope,
		Records: []Record{
			{ID: "npc", Type: NPC, Title: "Captain Vale", Summary: "Watch captain", Authority: Canon, Scope: scope, Source: "Session 16"},
			{ID: "loc", Type: Location, Title: "Watch House", Authority: Canon, Scope: scope},
			{ID: "thread", Type: Thread, Title: "Broken Seal", Authority: Canon, Scope: scope, Tags: []string{"open"}},
			{ID: "item", Type: Item, Title: "Ashen Crown", Authority: Canon, Scope: scope, Body: "Held by @Captain Vale."},
			{ID: "faction", Type: Faction, Title: "Greywatch", Authority: Canon, Scope: scope},
		},
		PlannedNotes: []PlannedNotes{
			{ID: "next", Title: "Crypt descent", Scope: scope, UpdatedAt: newer, LocationID: "loc", Links: []EntityLink{{RecordID: "npc"}}},
		},
		Sessions: []SessionRecord{
			{
				ID: "ended", Title: "Sit One", Scope: scope, StartedAt: older, EndedAt: &ended,
				Links: []EntityLink{{RecordID: "npc"}},
				Entries: []TranscriptEntry{
					{ID: "e1", Text: "Vale bars the door", Links: []EntityLink{{RecordID: "npc"}}},
					{ID: "e2", Text: "Vale names the seal", Links: []EntityLink{{RecordID: "npc"}}},
				},
			},
			{ID: "live", Title: "Sit Two", Scope: scope, StartedAt: newer, LocationID: "loc", Links: []EntityLink{{RecordID: "thread"}}},
		},
		Reconciliations: []ReconciliationRecord{{
			ID: "review", SessionID: "ended", CreatedAt: ended,
			Items: []ReconciliationItem{
				{ID: "i1", RecordID: "npc", Summary: "Cite Vale", Status: ReconPending, CreatedAt: ended},
				{ID: "i2", RecordID: "npc", Summary: "Promote Vale note", Status: ReconApproved, CreatedAt: ended.Add(time.Minute)},
			},
		}},
	}
}

func recordFor(t *testing.T, workspace Workspace, id string) Record {
	t.Helper()
	for _, record := range workspace.Records {
		if record.ID == id {
			return record
		}
	}
	t.Fatalf("no record %q", id)
	return Record{}
}

func TestEntityBriefAnswersTheTableQuestions(t *testing.T) {
	workspace := briefWorkspace()
	brief := DeriveEntityBrief(workspace, recordFor(t, workspace, "npc"), briefScope())

	if brief.LastSeen == nil || brief.LastSeen.SessionID != "ended" {
		t.Fatalf("last seen = %#v", brief.LastSeen)
	}
	label := brief.LastSeen.Label()
	if !strings.Contains(label, "Sit One") || !strings.Contains(label, "session cast") || !strings.Contains(label, "2 mentions") {
		t.Fatalf("last seen label = %q", label)
	}
	if !strings.Contains(label, "2 review") {
		t.Fatalf("last seen should count review items: %q", label)
	}
	if len(brief.Changes) != 2 || brief.Changes[0].Summary != "Promote Vale note" {
		t.Fatalf("changes = %#v", brief.Changes)
	}
	if len(brief.Open) != 1 || brief.Open[0].Kind != OpenReview {
		t.Fatalf("open = %#v", brief.Open)
	}
	joined := strings.Join(brief.WhyNow, " | ")
	if !strings.Contains(joined, "next prep cast") || !strings.Contains(joined, "waiting in review") {
		t.Fatalf("why now = %q", joined)
	}
	if !strings.Contains(joined, "most recent sit") {
		t.Fatalf("why now should name the latest sit: %q", joined)
	}
}

func TestEntityBriefNamesTheLiveLocationAndPrepLocation(t *testing.T) {
	workspace := briefWorkspace()
	brief := DeriveEntityBrief(workspace, recordFor(t, workspace, "loc"), briefScope())
	joined := strings.Join(brief.WhyNow, " | ")
	if !strings.Contains(joined, "party is right now") {
		t.Fatalf("live location should be called out: %q", joined)
	}
	if !strings.Contains(joined, "Location of the next prep") {
		t.Fatalf("prep location should be called out: %q", joined)
	}
	if brief.LastSeen != nil {
		t.Fatalf("a location never named in a transcript has no sighting: %#v", brief.LastSeen)
	}
}

func TestEntityBriefMarksLiveSitAsInPlayNow(t *testing.T) {
	workspace := briefWorkspace()
	brief := DeriveEntityBrief(workspace, recordFor(t, workspace, "thread"), briefScope())
	if brief.LastSeen == nil || !brief.LastSeen.Live {
		t.Fatalf("thread in the live cast should read as live: %#v", brief.LastSeen)
	}
	if !strings.Contains(brief.LastSeen.Label(), "in play now") {
		t.Fatalf("label = %q", brief.LastSeen.Label())
	}
	if !strings.Contains(strings.Join(brief.WhyNow, " | "), "Open thread") {
		t.Fatalf("open-tagged thread should say so: %#v", brief.WhyNow)
	}
}

func TestEntityBriefEmptyStatesStayHonest(t *testing.T) {
	scope := briefScope()
	workspace := Workspace{Scope: scope, Records: []Record{{ID: "lonely", Type: NPC, Title: "Nobody", Authority: Canon, Scope: scope}}}
	brief := DeriveEntityBrief(workspace, workspace.Records[0], scope)
	if brief.LastSeen != nil || len(brief.Sightings) != 0 {
		t.Fatalf("unused entity has no sightings: %#v", brief)
	}
	if len(brief.Changes) != 0 || len(brief.Open) != 0 || len(brief.WhyNow) != 0 {
		t.Fatalf("unused entity has nothing pending: %#v", brief)
	}
	if brief.ReferenceOnly {
		t.Fatal("a canon NPC is not reference-only")
	}
}

func TestEntityBriefReportsBrokenReferencesAsOpen(t *testing.T) {
	scope := briefScope()
	workspace := Workspace{Scope: scope, Records: []Record{
		{ID: "npc", Type: NPC, Title: "Vale", Authority: Canon, Scope: scope, Body: "Distrusts @Nobody At All."},
	}}
	brief := DeriveEntityBrief(workspace, workspace.Records[0], scope)
	found := false
	for _, item := range brief.Open {
		if item.Kind == OpenReference && strings.Contains(item.Summary, "Nobody") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a broken @ mention belongs in OPEN: %#v", brief.Open)
	}
}

func TestEntityBriefFlagsReferenceOnlyAndProposalStates(t *testing.T) {
	scope := briefScope()
	reference := Record{ID: "ref", Type: Note, Title: "Imported", Authority: Reference, Scope: scope, SourceID: "book"}
	brief := DeriveEntityBrief(Workspace{Scope: scope}, reference, scope)
	if !brief.ReferenceOnly {
		t.Fatal("reference authority means reference-only")
	}

	proposal := Record{ID: "prop", Type: NPC, Title: "Guess", Authority: Proposal, Scope: scope}
	brief = DeriveEntityBrief(Workspace{Scope: scope}, proposal, scope)
	if len(brief.Open) != 1 || brief.Open[0].Kind != OpenAuthority {
		t.Fatalf("a proposal is unresolved until accepted: %#v", brief.Open)
	}
}

func TestEntityBriefSurfacesUnfiledCaptures(t *testing.T) {
	scope := briefScope()
	capture, err := NewCaptureNote("the seal is cracked", CaptureContext{Origin: "live play"}, scope)
	if err != nil {
		t.Fatal(err)
	}
	brief := DeriveEntityBrief(Workspace{Scope: scope, Records: []Record{capture}}, capture, scope)
	if len(brief.Open) == 0 || brief.Open[0].Kind != OpenFiling {
		t.Fatalf("an unfiled capture is unresolved: %#v", brief.Open)
	}
	if !strings.Contains(strings.Join(brief.WhyNow, " | "), "Unfiled capture") {
		t.Fatalf("why now = %#v", brief.WhyNow)
	}
}

func TestEntityBriefOrderVariesByTypeButKeepsMetadataLast(t *testing.T) {
	types := []EntityType{NPC, Character, Creature, Location, Item, Faction, Thread, Note, Rule}
	for _, entityType := range types {
		order := EntityBriefOrder(entityType)
		if len(order) != 6 {
			t.Fatalf("%s order = %#v", entityType, order)
		}
		if order[0] != BriefWhyNow {
			t.Fatalf("%s should lead with relevance, got %s", entityType, order[0])
		}
		if order[len(order)-1] != BriefMeta {
			t.Fatalf("%s must not end anywhere but metadata, got %s", entityType, order[len(order)-1])
		}
		seen := map[EntityBriefSection]bool{}
		for _, section := range order {
			if seen[section] {
				t.Fatalf("%s repeats %s", entityType, section)
			}
			seen[section] = true
		}
	}
	thread := EntityBriefOrder(Thread)
	if indexOfSection(thread, BriefChanged) > indexOfSection(thread, BriefConnected) {
		t.Fatalf("a thread leads with its development: %#v", thread)
	}
	item := EntityBriefOrder(Item)
	if indexOfSection(item, BriefConnected) > indexOfSection(item, BriefLastSeen) {
		t.Fatalf("an item leads with who holds it: %#v", item)
	}
	npc := EntityBriefOrder(NPC)
	if indexOfSection(npc, BriefLastSeen) > indexOfSection(npc, BriefConnected) {
		t.Fatalf("an NPC leads with where it appeared: %#v", npc)
	}
}

func indexOfSection(order []EntityBriefSection, want EntityBriefSection) int {
	for index, section := range order {
		if section == want {
			return index
		}
	}
	return -1
}

func TestEntityBriefBoundsLongChangeLists(t *testing.T) {
	scope := briefScope()
	at := time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC)
	items := make([]ReconciliationItem, 0, 20)
	for index := 0; index < 20; index++ {
		items = append(items, ReconciliationItem{
			ID: string(rune('a' + index)), RecordID: "npc", Summary: "change", Status: ReconApproved,
			CreatedAt: at.Add(time.Duration(index) * time.Minute),
		})
	}
	workspace := Workspace{
		Scope:           scope,
		Records:         []Record{{ID: "npc", Type: NPC, Title: "Vale", Authority: Canon, Scope: scope}},
		Sessions:        []SessionRecord{{ID: "s", Title: "Sit", Scope: scope, StartedAt: at}},
		Reconciliations: []ReconciliationRecord{{ID: "r", SessionID: "s", Items: items}},
	}
	brief := DeriveEntityBrief(workspace, workspace.Records[0], scope)
	if len(brief.Changes) != 6 {
		t.Fatalf("changes should stay bounded, got %d", len(brief.Changes))
	}
	if !brief.Changes[0].At.After(brief.Changes[1].At) {
		t.Fatal("changes should read newest first")
	}
}

func TestDeriveEntityBriefToleratesAnEmptyRecord(t *testing.T) {
	brief := DeriveEntityBrief(briefWorkspace(), Record{}, briefScope())
	if brief.LastSeen != nil || len(brief.Changes) != 0 || len(brief.Open) != 0 {
		t.Fatalf("an empty record derives nothing: %#v", brief)
	}
}
