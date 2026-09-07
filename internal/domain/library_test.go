package domain

import "testing"

func testLibraryWorkspace() Workspace {
	crown := Scope{WorldID: "ashen-realms", WorldName: "The Ashen Realms", CampaignID: "ashen-crown", Campaign: "The Ashen Crown"}
	embers := Scope{WorldID: "ashen-realms", WorldName: "The Ashen Realms", CampaignID: "embers-north", Campaign: "Embers in the North"}
	world := Scope{WorldID: "ashen-realms", WorldName: "The Ashen Realms"}
	barovia := Scope{WorldID: "barovia", WorldName: "Barovia", CampaignID: "ravenloft", Campaign: "Mists of Ravenloft"}
	return Workspace{
		Scope: crown,
		Library: []WorldRef{
			{ID: "ashen-realms", Name: "The Ashen Realms", Campaigns: []CampaignRef{
				{ID: "ashen-crown", Name: "The Ashen Crown"},
				{ID: "embers-north", Name: "Embers in the North"},
			}},
			{ID: "barovia", Name: "Barovia", Campaigns: []CampaignRef{
				{ID: "ravenloft", Name: "Mists of Ravenloft"},
			}},
		},
		Records: []Record{
			{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Scope: crown},
			{ID: "loc-greywatch", Type: Location, Title: "Greywatch", Scope: world},
			{ID: "npc-maren", Type: NPC, Title: "Maren", Scope: embers},
			{ID: "npc-strahd", Type: NPC, Title: "Strahd", Scope: barovia},
		},
		Sessions: []SessionRecord{
			{ID: "sit-crown", Title: "Crypt", Scope: crown},
			{ID: "sit-barovia", Title: "Castle", Scope: barovia},
		},
		PlannedNotes: []PlannedNotes{
			{ID: "plan-crown", Title: "Next sit", Scope: crown},
		},
		Collections: []Collection{
			{ID: "col-crown", Title: "Circle", Scope: crown},
		},
		Reconciliations: []ReconciliationRecord{
			{ID: "recon-crown", SessionID: "sit-crown", Title: "Reconcile Crypt"},
			{ID: "recon-barovia", SessionID: "sit-barovia", Title: "Reconcile Castle"},
		},
	}
}

func TestRenameWorldUpdatesMatchingScopes(t *testing.T) {
	ws := testLibraryWorkspace()
	if err := ws.RenameWorld("ashen-realms", "The Cinder Marches"); err != nil {
		t.Fatal(err)
	}
	if ws.Library[0].Name != "The Cinder Marches" {
		t.Fatalf("library name=%q", ws.Library[0].Name)
	}
	if ws.Scope.WorldName != "The Cinder Marches" || ws.Scope.Campaign != "The Ashen Crown" {
		t.Fatalf("scope=%#v", ws.Scope)
	}
	if ws.Records[0].Scope.WorldName != "The Cinder Marches" || ws.Records[1].Scope.WorldName != "The Cinder Marches" {
		t.Fatalf("records=%#v", ws.Records)
	}
	if ws.Records[3].Scope.WorldName != "Barovia" {
		t.Fatalf("other world renamed: %#v", ws.Records[3].Scope)
	}
}

func TestRenameCampaignUpdatesMatchingScopes(t *testing.T) {
	ws := testLibraryWorkspace()
	if err := ws.RenameCampaign("ashen-realms", "ashen-crown", "Crown of Ash"); err != nil {
		t.Fatal(err)
	}
	if ws.Library[0].Campaigns[0].Name != "Crown of Ash" {
		t.Fatalf("campaign=%#v", ws.Library[0].Campaigns[0])
	}
	if ws.Scope.Campaign != "Crown of Ash" {
		t.Fatalf("scope=%#v", ws.Scope)
	}
	if ws.Records[0].Scope.Campaign != "Crown of Ash" {
		t.Fatalf("crown record=%#v", ws.Records[0].Scope)
	}
	if ws.Records[2].Scope.Campaign != "Embers in the North" {
		t.Fatalf("sibling renamed: %#v", ws.Records[2].Scope)
	}
}

func TestDeleteCampaignDropsScopedDataKeepsWorldShared(t *testing.T) {
	ws := testLibraryWorkspace()
	if err := ws.DeleteCampaign("ashen-realms", "ashen-crown"); err != nil {
		t.Fatal(err)
	}
	world, ok := ws.FindWorld("ashen-realms")
	if !ok || len(world.Campaigns) != 1 || world.Campaigns[0].ID != "embers-north" {
		t.Fatalf("library=%#v", ws.Library)
	}
	if ws.Scope.CampaignID != "" || ws.Scope.WorldID != "ashen-realms" {
		t.Fatalf("active campaign should clear, world stay: %#v", ws.Scope)
	}
	titles := map[string]bool{}
	for _, record := range ws.Records {
		titles[record.Title] = true
	}
	if titles["Captain Vale"] || !titles["Greywatch"] || !titles["Maren"] || !titles["Strahd"] {
		t.Fatalf("records=%#v", titles)
	}
	if len(ws.Sessions) != 1 || ws.Sessions[0].ID != "sit-barovia" {
		t.Fatalf("sessions=%#v", ws.Sessions)
	}
	if len(ws.PlannedNotes) != 0 || len(ws.Collections) != 0 {
		t.Fatalf("prep/collections leftover")
	}
	if len(ws.Reconciliations) != 1 || ws.Reconciliations[0].SessionID != "sit-barovia" {
		t.Fatalf("recons=%#v", ws.Reconciliations)
	}
}

func TestDeleteWorldDropsEveryScopedRow(t *testing.T) {
	ws := testLibraryWorkspace()
	if err := ws.DeleteWorld("ashen-realms"); err != nil {
		t.Fatal(err)
	}
	if _, ok := ws.FindWorld("ashen-realms"); ok {
		t.Fatal("world still in library")
	}
	if ws.Scope.WorldID != "" {
		t.Fatalf("scope should clear: %#v", ws.Scope)
	}
	if len(ws.Records) != 1 || ws.Records[0].Title != "Strahd" {
		t.Fatalf("records=%#v", ws.Records)
	}
	if len(ws.Sessions) != 1 || ws.Sessions[0].ID != "sit-barovia" {
		t.Fatalf("sessions=%#v", ws.Sessions)
	}
}

func TestRenameRejectsEmptyName(t *testing.T) {
	ws := testLibraryWorkspace()
	if err := ws.RenameWorld("ashen-realms", "  "); err == nil {
		t.Fatal("expected empty world name error")
	}
	if err := ws.RenameCampaign("ashen-realms", "ashen-crown", ""); err == nil {
		t.Fatal("expected empty campaign name error")
	}
}
