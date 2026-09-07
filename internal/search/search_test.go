package search

import (
	"testing"

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
	goblin := domain.Record{
		ID: "src-mm-goblins", Type: domain.Creature, Title: "Goblins",
		Authority: domain.Canon, SourceID: "src-mm",
	}
	service := New([]domain.Record{goblin})
	hidden := service.Find(Filter{Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID})
	if len(hidden) != 0 {
		t.Fatalf("disabled source should be hidden, got %#v", hidden)
	}
	found := service.Find(Filter{
		Scope: CurrentCampaign, WorldID: testScope.WorldID, CampaignID: testScope.CampaignID,
		EnabledSourceIDs: []string{"src-mm"},
	})
	if len(found) != 1 || found[0].Record.ID != "src-mm-goblins" {
		t.Fatalf("expected enabled goblin, got %#v", found)
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
