package tui

import "github.com/hbaldwin98/dungeon/internal/domain"

func demoWorkspace() domain.Workspace {
	scope := domain.Scope{
		WorldID:    "ashen-realms",
		WorldName:  "The Ashen Realms",
		CampaignID: "ashen-crown",
		Campaign:   "The Ashen Crown",
	}

	records := []domain.Record{
		{
			ID: "npc-captain-vale", Type: domain.NPC, Title: "Captain Vale",
			Summary:   "Captain of the Greywatch Guard; wary of the party.",
			Body:      "Vale is investigating the broken crypt seal and openly distrusts Father Merrow.",
			Authority: domain.Canon, Scope: scope, Source: "Session 16",
			Aliases: []string{"Alaric Vale"}, Tags: []string{"greywatch", "guard"},
		},
		{
			ID: "npc-father-merrow", Type: domain.NPC, Title: "Father Merrow",
			Summary:   "A priest who arrived in Greywatch six months ago.",
			Body:      "His purpose at the abandoned monastery remains deliberately unresolved.",
			Authority: domain.Secret, Scope: scope, Source: "DM notes",
			Tags: []string{"greywatch", "clergy"},
		},
		{
			ID: "location-monastery", Type: domain.Location, Title: "Ruined Monastery",
			Summary:   "An abandoned monastery beneath Greywatch.",
			Body:      "The party entered through the collapsed eastern transept.",
			Authority: domain.Canon, Scope: scope, Source: "Session 17",
			Tags: []string{"current-scene", "greywatch"},
		},
		{
			ID: "item-silver-key", Type: domain.Item, Title: "Silver Key",
			Summary:   "Recovered by the party; what it opens is unknown.",
			Body:      "The key was found near the broken crypt seal.",
			Authority: domain.Unknown, Scope: scope, Source: "Session 17",
			Tags: []string{"reliquary", "crypt"},
		},
		{
			ID: "thread-reliquary", Type: domain.Thread, Title: "Missing Reliquary",
			Summary:   "The monastery reliquary is missing.",
			Body:      "Vale suspects Merrow, but no evidence establishes responsibility.",
			Authority: domain.Canon, Scope: scope, Source: "Session 16",
			Tags: []string{"open", "greywatch"},
		},
		{
			ID: "draft-sister-elayne", Type: domain.NPC, Title: "Sister Elayne",
			Summary:   "A possible church investigator being prepared by the DM.",
			Body:      "This entity is a DM-authored draft. It is organized campaign data, but not yet established as true.",
			Authority: domain.Draft, Scope: scope, Source: "DM draft",
			Tags: []string{"draft", "church"},
		},
		{
			ID: "proposal-investigator", Type: domain.Thread, Title: "Church Investigator Arrives",
			Summary:   "The church could send an investigator after hearing of the disturbed reliquary.",
			Body:      "Generated possibility. This has not happened and is not campaign truth.",
			Authority: domain.Proposal, Scope: scope, Source: "AI brainstorm",
			IsAIContent: true, Tags: []string{"idea", "church"},
		},
		{
			ID: "world-greywatch", Type: domain.Location, Title: "Greywatch",
			Summary:   "A poor village distrustful of the established church.",
			Body:      "Locals still bury their dead in the old monastery cemetery.",
			Authority: domain.Canon,
			Scope:     domain.Scope{WorldID: scope.WorldID, WorldName: scope.WorldName},
			Source:    "World canon",
		},
		{
			ID: "sibling-npc", Type: domain.NPC, Title: "Maren of the Embers",
			Summary:   "An NPC from a different campaign in the same world.",
			Body:      "This record demonstrates campaign search isolation.",
			Authority: domain.Canon,
			Scope:     domain.Scope{WorldID: scope.WorldID, WorldName: scope.WorldName, CampaignID: "embers-north", Campaign: "Embers in the North"},
			Source:    "Embers in the North",
		},
	}

	workspace, err := domain.NewWorkspace(scope, records)
	if err != nil {
		panic(err)
	}
	return workspace
}
