package domain

import "testing"

func TestRecordVisibleInIncludesEnabledLibrarySource(t *testing.T) {
	scope := Scope{WorldID: "test-world", WorldName: "Test World", CampaignID: "test-campaign", Campaign: "Test Campaign"}
	libraryCreature := Record{
		ID: "src-bst-rascals", Type: Creature, Title: "Cave Rascals",
		Authority: Canon, SourceID: "src-bst",
	}
	otherBook := Record{
		ID: "src-other-beast", Type: Creature, Title: "Other Beast",
		Authority: Canon, SourceID: "src-other",
	}
	campaignNPC := Record{
		ID: "npc-reed", Type: NPC, Title: "Reed",
		Authority: Canon, Scope: scope, SourceID: "src-adv",
	}
	enabled := []string{"src-bst", "src-adv"}
	if !RecordVisibleIn(libraryCreature, scope, enabled) {
		t.Fatal("enabled bestiary creature should be visible in the campaign")
	}
	if RecordVisibleIn(otherBook, scope, enabled) {
		t.Fatal("a source the campaign did not enable should stay hidden")
	}
	if !RecordVisibleIn(campaignNPC, scope, enabled) {
		t.Fatal("campaign records should remain visible")
	}
}

func TestEnableSourceIsIdempotent(t *testing.T) {
	ws := Workspace{
		Library: []WorldRef{{
			ID: "test-world", Name: "Test World",
			Campaigns: []CampaignRef{{ID: "test-campaign", Name: "Test Campaign"}},
		}},
	}
	ws.EnableSource("test-world", "test-campaign", "src-bst")
	ws.EnableSource("test-world", "test-campaign", "src-bst")
	got := ws.EnabledSourceIDs(Scope{WorldID: "test-world", CampaignID: "test-campaign"})
	if len(got) != 1 || got[0] != "src-bst" {
		t.Fatalf("enabled=%#v", got)
	}
	ws.DisableSource("test-world", "test-campaign", "src-bst")
	if len(ws.EnabledSourceIDs(Scope{WorldID: "test-world", CampaignID: "test-campaign"})) != 0 {
		t.Fatal("expected source to be disabled")
	}
}

func TestStripSourceContentLeavesOtherRecords(t *testing.T) {
	ws := Workspace{
		Records: []Record{
			{ID: "a", Type: Creature, Title: "Cave Rascals", SourceID: "src-bst"},
			{ID: "b", Type: NPC, Title: "Vale", SourceID: "src-adv"},
		},
		PlannedNotes: []PlannedNotes{
			{ID: "p1", Title: "The Ambush", SourceID: "src-adv"},
			{ID: "p2", Title: "Other", SourceID: "src-other"},
		},
	}
	ws.StripSourceContent("src-adv")
	if len(ws.Records) != 1 || ws.Records[0].ID != "a" {
		t.Fatalf("records=%#v", ws.Records)
	}
	if len(ws.PlannedNotes) != 1 || ws.PlannedNotes[0].ID != "p2" {
		t.Fatalf("plans=%#v", ws.PlannedNotes)
	}
}

func TestRemoveSourceDropsDocumentAndDisablesCampaigns(t *testing.T) {
	ws := Workspace{
		Library: []WorldRef{{
			ID: "test-world", Name: "Test World",
			Campaigns: []CampaignRef{
				{ID: "test-campaign", Name: "Test Campaign", EnabledSourceIDs: []string{"src-bst", "src-adv"}},
				{ID: "other-campaign", Name: "Other Campaign", EnabledSourceIDs: []string{"src-bst"}},
			},
		}},
		Sources: []SourceDocument{
			{ID: "src-bst", Title: "Test Bestiary (2025)", Kind: SourceBestiary},
			{ID: "src-adv", Title: "The Hollow Crown", Kind: SourceAdventure},
		},
		Records: []Record{
			{ID: "a", Type: Creature, Title: "Cave Rascals", SourceID: "src-bst"},
			{ID: "b", Type: NPC, Title: "Darran", SourceID: "src-adv"},
		},
		PlannedNotes: []PlannedNotes{
			{ID: "p1", Title: "The Ambush", SourceID: "src-adv"},
		},
		Sessions: []SessionRecord{
			{ID: "s1", Title: "Night 1", Links: []EntityLink{{RecordID: "a"}}},
		},
	}
	if !ws.RemoveSource("src-bst") {
		t.Fatal("expected src-bst to be removed")
	}
	if _, ok := ws.FindSource("src-bst"); ok {
		t.Fatal("source document should be gone")
	}
	if len(ws.Sources) != 1 || ws.Sources[0].ID != "src-adv" {
		t.Fatalf("sources=%#v", ws.Sources)
	}
	if len(ws.Records) != 1 || ws.Records[0].ID != "b" {
		t.Fatalf("records=%#v", ws.Records)
	}
	if len(ws.PlannedNotes) != 1 {
		t.Fatalf("plans=%#v", ws.PlannedNotes)
	}
	if got := ws.EnabledSourceIDs(Scope{WorldID: "test-world", CampaignID: "test-campaign"}); len(got) != 1 || got[0] != "src-adv" {
		t.Fatalf("campaign enabled=%#v", got)
	}
	if got := ws.EnabledSourceIDs(Scope{WorldID: "test-world", CampaignID: "other-campaign"}); len(got) != 0 {
		t.Fatalf("other campaign should have no sources, got %#v", got)
	}
	if len(ws.Sessions[0].Links) != 1 || ws.Sessions[0].Links[0].RecordID != "a" {
		t.Fatalf("session links should survive for re-ingest: %#v", ws.Sessions[0].Links)
	}
	if ws.RemoveSource("missing") {
		t.Fatal("missing source should not report removed")
	}
	doc, ok := ws.FindSource("The Hollow Crown")
	if !ok || doc.ID != "src-adv" {
		t.Fatalf("title lookup=%#v ok=%v", doc, ok)
	}
}
