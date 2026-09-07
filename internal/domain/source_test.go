package domain

import "testing"

func TestRecordVisibleInIncludesEnabledLibrarySource(t *testing.T) {
	scope := Scope{WorldID: "fr", WorldName: "Forgotten Realms", CampaignID: "lmop", Campaign: "Lost Mine"}
	libraryGoblin := Record{
		ID: "src-mm-goblins", Type: Creature, Title: "Goblins",
		Authority: Canon, SourceID: "src-mm",
	}
	otherBook := Record{
		ID: "src-other-orc", Type: Creature, Title: "Orc",
		Authority: Canon, SourceID: "src-other",
	}
	campaignNPC := Record{
		ID: "npc-sildar", Type: NPC, Title: "Sildar",
		Authority: Canon, Scope: scope, SourceID: "src-lmop",
	}
	enabled := []string{"src-mm", "src-lmop"}
	if !RecordVisibleIn(libraryGoblin, scope, enabled) {
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
			ID: "fr", Name: "Forgotten Realms",
			Campaigns: []CampaignRef{{ID: "lmop", Name: "Lost Mine"}},
		}},
	}
	ws.EnableSource("fr", "lmop", "src-mm")
	ws.EnableSource("fr", "lmop", "src-mm")
	got := ws.EnabledSourceIDs(Scope{WorldID: "fr", CampaignID: "lmop"})
	if len(got) != 1 || got[0] != "src-mm" {
		t.Fatalf("enabled=%#v", got)
	}
	ws.DisableSource("fr", "lmop", "src-mm")
	if len(ws.EnabledSourceIDs(Scope{WorldID: "fr", CampaignID: "lmop"})) != 0 {
		t.Fatal("expected source to be disabled")
	}
}

func TestStripSourceContentLeavesOtherRecords(t *testing.T) {
	ws := Workspace{
		Records: []Record{
			{ID: "a", Type: Creature, Title: "Goblins", SourceID: "src-mm"},
			{ID: "b", Type: NPC, Title: "Vale", SourceID: "src-lmop"},
		},
		PlannedNotes: []PlannedNotes{
			{ID: "p1", Title: "Goblin Arrows", SourceID: "src-lmop"},
			{ID: "p2", Title: "Other", SourceID: "src-other"},
		},
	}
	ws.StripSourceContent("src-lmop")
	if len(ws.Records) != 1 || ws.Records[0].ID != "a" {
		t.Fatalf("records=%#v", ws.Records)
	}
	if len(ws.PlannedNotes) != 1 || ws.PlannedNotes[0].ID != "p2" {
		t.Fatalf("plans=%#v", ws.PlannedNotes)
	}
}
