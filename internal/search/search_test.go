package search

import (
	"testing"
	"time"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

var testScope = domain.Scope{
	WorldID:    "ashen-realms",
	WorldName:  "The Ashen Realms",
	CampaignID: "ashen-crown",
	Campaign:   "The Ashen Crown",
}

func TestFindReturnsTypedFuzzyResults(t *testing.T) {
	service := New([]domain.Record{
		{ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale", Authority: domain.Canon, Scope: testScope},
		{ID: "item-key", Type: domain.Item, Title: "Silver Key", Authority: domain.Canon, Scope: testScope},
	})

	results := service.Find(Filter{
		Query:      "cptvl",
		Scope:      CurrentCampaign,
		WorldID:    testScope.WorldID,
		CampaignID: testScope.CampaignID,
	})

	if len(results) != 1 || results[0].Record.ID != "npc-vale" {
		t.Fatalf("expected Captain Vale, got %#v", results)
	}
}

func TestCampaignSearchIncludesWorldFactsButNotSiblingCampaign(t *testing.T) {
	worldRecord := domain.Record{ID: "world", Type: domain.Location, Title: "Greywatch", Authority: domain.Canon, Scope: domain.Scope{WorldID: testScope.WorldID}}
	siblingRecord := domain.Record{ID: "sibling", Type: domain.NPC, Title: "Greywatch Scout", Authority: domain.Canon, Scope: domain.Scope{WorldID: testScope.WorldID, CampaignID: "embers"}}
	service := New([]domain.Record{worldRecord, siblingRecord})

	results := service.Find(Filter{Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID})
	if len(results) != 1 || results[0].Record.ID != "world" {
		t.Fatalf("expected only the shared world record, got %#v", results)
	}
}

func TestProposalsAreExcludedUnlessRequested(t *testing.T) {
	service := New([]domain.Record{{
		ID: "idea", Type: domain.Thread, Title: "Church Investigator",
		Authority: domain.Proposal, Scope: testScope, IsAIContent: true,
	}})
	filter := Filter{Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID}

	if results := service.Find(filter); len(results) != 0 {
		t.Fatalf("expected proposals to be excluded, got %#v", results)
	}
	filter.IncludeProposals = true
	if results := service.Find(filter); len(results) != 1 {
		t.Fatalf("expected proposal when explicitly included, got %#v", results)
	}
}

func TestCampaignSearchIncludesEnabledSourceRecords(t *testing.T) {
	rascal := domain.Record{
		ID: "src-bst-rascals", Type: domain.Creature, Title: "Cave Rascals",
		Authority: domain.Canon, SourceID: "src-bst",
	}
	service := New([]domain.Record{rascal})
	hidden := service.Find(Filter{Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID})
	if len(hidden) != 0 {
		t.Fatalf("disabled source should be hidden, got %#v", hidden)
	}
	found := service.Find(Filter{
		Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID,
		EnabledSourceIDs: []string{"src-bst"},
	})
	if len(found) != 1 || found[0].Record.ID != "src-bst-rascals" {
		t.Fatalf("expected enabled cave rascals, got %#v", found)
	}
}

func TestTagFilterRequiresAllTags(t *testing.T) {
	service := New([]domain.Record{
		{ID: "a", Type: domain.NPC, Title: "Vale", Authority: domain.Canon, Scope: testScope, Tags: []string{"greywatch", "guard"}},
		{ID: "b", Type: domain.NPC, Title: "Merrow", Authority: domain.Canon, Scope: testScope, Tags: []string{"greywatch", "clergy"}},
	})
	results := service.Find(Filter{
		Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID,
		Tags: []string{"greywatch", "guard"},
	})
	if len(results) != 1 || results[0].Record.ID != "a" {
		t.Fatalf("expected Vale only, got %#v", results)
	}
}

func campaignFilter(query string) Filter {
	return Filter{
		Query:      query,
		Scope:      CurrentCampaign,
		WorldID:    testScope.WorldID,
		CampaignID: testScope.CampaignID,
	}
}

func TestFindReturnsMatchingPrepNotes(t *testing.T) {
	service := FromWorkspace(domain.Workspace{
		PlannedNotes: []domain.PlannedNotes{
			{ID: "plan-ambush", Title: "Crypt Ambush", Body: "Vale waits at the east gate.", Scope: testScope},
			{ID: "plan-other", Title: "Market day", Body: "Shopping list.", Scope: testScope},
			{ID: "plan-sibling", Title: "Crypt Ambush North", Body: "Wrong campaign.", Scope: domain.Scope{WorldID: testScope.WorldID, CampaignID: "embers"}},
		},
	})

	results := service.Find(campaignFilter("ambush"))
	if len(results) != 1 || results[0].Kind != KindPrep || results[0].TargetID() != "plan-ambush" {
		t.Fatalf("expected campaign prep hit, got %#v", results)
	}
	if results[0].TypeLabel() != "prep" || results[0].DisplayTitle() != "Crypt Ambush" {
		t.Fatalf("expected labeled prep title, got %#v", results[0])
	}
}

func TestFindReturnsMatchingSessionsAndTranscripts(t *testing.T) {
	ended := timePtr()
	service := FromWorkspace(domain.Workspace{
		Sessions: []domain.SessionRecord{
			{
				ID: "sit-17", Title: "Greywatch Watch", Scope: testScope, LocationName: "Ruined Monastery",
				EndedAt: ended,
				Entries: []domain.TranscriptEntry{
					{ID: "e1", Text: "The party found a silver key near the crypt seal."},
					{ID: "e2", Text: "undone whisper", Undone: true},
				},
			},
			{ID: "sit-ember", Title: "Greywatch Watch", Scope: domain.Scope{WorldID: testScope.WorldID, CampaignID: "embers"}},
		},
	})

	sessions := service.Find(campaignFilter("greywatch watch"))
	if !hasKind(sessions, KindSession, "sit-17") || hasKind(sessions, KindSession, "sit-ember") {
		t.Fatalf("expected campaign session only, got %#v", sessions)
	}

	notes := service.Find(campaignFilter("silver key"))
	if !hasKind(notes, KindTranscript, "e1") {
		t.Fatalf("expected transcript hit, got %#v", notes)
	}
	if hasKind(notes, KindTranscript, "e2") {
		t.Fatal("undone transcript entries should not appear")
	}
}

func TestFindReturnsMatchingReconciliation(t *testing.T) {
	service := FromWorkspace(domain.Workspace{
		Sessions: []domain.SessionRecord{
			{ID: "sit-17", Title: "Session 17", Scope: testScope},
		},
		Reconciliations: []domain.ReconciliationRecord{
			{
				ID: "recon-sit-17", SessionID: "sit-17", Title: "Reconcile Session 17",
				Items: []domain.ReconciliationItem{{Summary: "Promote draft: Sister Elayne"}},
			},
		},
	})

	results := service.Find(campaignFilter("sister elayne"))
	if !hasKind(results, KindRecon, "recon-sit-17") {
		t.Fatalf("expected recon hit, got %#v", results)
	}
}

func TestFindSkipsExtrasWhenQueryEmpty(t *testing.T) {
	service := FromWorkspace(domain.Workspace{
		Records:      []domain.Record{{ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale", Authority: domain.Canon, Scope: testScope}},
		PlannedNotes: []domain.PlannedNotes{{ID: "plan-ambush", Title: "Crypt Ambush", Scope: testScope}},
	})
	results := service.Find(campaignFilter(""))
	if len(results) != 1 || results[0].Kind != KindRecord {
		t.Fatalf("empty query should stay wiki-only, got %#v", results)
	}
}

func TestFindReturnsMatchingAdventureReferences(t *testing.T) {
	mira := domain.Record{
		ID: "ref:src-adv:mira-holt", Type: domain.Note, Title: "Mira Holt",
		Summary: "A scout from Greywatch.", Body: "Mira watches the east road.",
		Authority: domain.Canon, Source: "The Hollow Crown", SourceID: "src-adv",
		Aliases: []string{"Mira Holt"}, Tags: []string{"reference", "adventure"},
	}
	service := FromDocuments(append(
		DocumentsFromWorkspace(domain.Workspace{
			Records: []domain.Record{{ID: "npc-vale", Type: domain.NPC, Title: "Captain Vale", Authority: domain.Canon, Scope: testScope}},
		}),
		DocumentFromReference(mira),
	))

	found := service.Find(Filter{
		Query: "Mira", Scope: CurrentCampaign,
		WorldID: testScope.WorldID, CampaignID: testScope.CampaignID,
		EnabledSourceIDs: []string{"src-adv"},
	})
	if !hasKind(found, KindReference, mira.ID) {
		t.Fatalf("expected enabled reference hit, got %#v", found)
	}
	if found[0].TypeLabel() != "reference" {
		t.Fatalf("expected reference label, got %q", found[0].TypeLabel())
	}

	hidden := service.Find(campaignFilter("Mira"))
	if hasKind(hidden, KindReference, mira.ID) {
		t.Fatal("disabled adventure reference should be hidden")
	}

	empty := service.Find(Filter{
		Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID,
		EnabledSourceIDs: []string{"src-adv"},
	})
	if hasKind(empty, KindReference, mira.ID) {
		t.Fatal("empty query should stay wiki-only")
	}
}

func hasKind(results []Result, kind Kind, id string) bool {
	for _, result := range results {
		if result.Kind == kind && result.TargetID() == id {
			return true
		}
	}
	return false
}

func timePtr() *time.Time {
	now := time.Now()
	return &now
}
