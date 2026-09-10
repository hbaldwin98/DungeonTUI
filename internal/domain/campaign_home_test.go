package domain

import (
	"testing"
	"time"
)

func TestDeriveCampaignHomeSelectsCurrentWork(t *testing.T) {
	scope := Scope{WorldID: "world", CampaignID: "campaign"}
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	ended := newer.Add(time.Hour)
	workspace := Workspace{
		Scope: scope,
		Records: []Record{
			{ID: "npc", Type: NPC, Title: "Arden", Authority: Canon, Scope: scope},
			{ID: "thread", Type: Thread, Title: "Broken Seal", Authority: Secret, Scope: scope, Tags: []string{"OPEN"}},
			{ID: "proposal", Type: Thread, Title: "Ignore", Authority: Proposal, Scope: scope, Tags: []string{"open"}},
		},
		PlannedNotes: []PlannedNotes{
			{ID: "used", Title: "Used", Scope: scope, UpdatedAt: newer},
			{ID: "next", Title: "Next", Scope: scope, UpdatedAt: older, Links: []EntityLink{{RecordID: "npc"}}},
		},
		Sessions: []SessionRecord{
			{ID: "ended", Title: "Last Sit", Scope: scope, StartedAt: older, EndedAt: &ended, LocationName: "Old Keep", PlannedNotesID: "used"},
			{ID: "live", Title: "Live Sit", Scope: scope, StartedAt: newer, LocationName: "Moon Gate"},
		},
		Reconciliations: []ReconciliationRecord{{
			ID: "review", SessionID: "ended", CreatedAt: newer,
			Items: []ReconciliationItem{
				{Status: ReconPending, CreatedAt: newer},
				{Status: ReconDeferred, CreatedAt: newer},
				{Status: ReconApproved, RecordID: "npc", Summary: "Changed", CreatedAt: newer},
			},
		}},
	}

	home := DeriveCampaignHome(workspace, scope)
	if home.NextPlan == nil || home.NextPlan.ID != "next" {
		t.Fatalf("next plan = %#v", home.NextPlan)
	}
	if home.LatestSession == nil || home.LatestSession.ID != "ended" {
		t.Fatalf("latest session = %#v", home.LatestSession)
	}
	if home.CurrentLocation != "Moon Gate" || len(home.Cast) != 1 || home.Cast[0].ID != "npc" {
		t.Fatalf("location/cast = %q %#v", home.CurrentLocation, home.Cast)
	}
	if len(home.OpenThreads) != 1 || home.OpenThreads[0].ID != "thread" {
		t.Fatalf("threads = %#v", home.OpenThreads)
	}
	if home.UnresolvedReviews != 2 || home.NextReviewID != "review" {
		t.Fatalf("reviews = %d %q", home.UnresolvedReviews, home.NextReviewID)
	}
	if len(home.RecentChanges) != 1 || home.RecentChanges[0].Title != "Arden" {
		t.Fatalf("changes = %#v", home.RecentChanges)
	}
}

func TestDeriveCampaignHomeHandlesEmptyCampaign(t *testing.T) {
	home := DeriveCampaignHome(Workspace{}, Scope{WorldID: "world", CampaignID: "campaign"})
	if home.NextPlan != nil || home.LatestSession != nil || home.CurrentLocation != "" || len(home.Cast) != 0 || len(home.OpenThreads) != 0 || home.UnresolvedReviews != 0 || len(home.RecentChanges) != 0 {
		t.Fatalf("expected empty home, got %#v", home)
	}
}
