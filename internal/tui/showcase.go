package tui

import (
	"time"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

// ShowcaseWorkspace is a populated Ashen Crown library meant to exercise
// folders, collections, prep arcs, playback, tags, and typed wiki sections.
func ShowcaseWorkspace() domain.Workspace {
	crown := domain.Scope{
		WorldID:    "ashen-realms",
		WorldName:  "The Ashen Realms",
		CampaignID: "ashen-crown",
		Campaign:   "The Ashen Crown",
	}
	embers := domain.Scope{
		WorldID:    "ashen-realms",
		WorldName:  "The Ashen Realms",
		CampaignID: "embers-north",
		Campaign:   "Embers in the North",
	}
	world := domain.Scope{WorldID: crown.WorldID, WorldName: crown.WorldName}
	barovia := domain.Scope{
		WorldID:    "barovia",
		WorldName:  "Barovia",
		CampaignID: "ravenloft",
		Campaign:   "Mists of Ravenloft",
	}

	at := func(y int, m time.Month, d, h int) time.Time {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
	}
	end := func(t time.Time) *time.Time {
		u := t.Add(3 * time.Hour)
		return &u
	}
	link := func(id, text string) domain.EntityLink {
		return domain.EntityLink{RecordID: id, Text: text}
	}

	records := []domain.Record{
		rec("npc-captain-vale", domain.NPC, "Captain Vale",
			"Captain of the Greywatch Guard; wary of the party.",
			"Vale is investigating the broken crypt seal and openly distrusts @Father Merrow. He will not leave the watch house unmanned at night.",
			domain.Canon, crown, "Session 16", []string{"Alaric Vale"}, []string{"greywatch", "guard"}),
		rec("npc-father-merrow", domain.NPC, "Father Merrow",
			"A priest who arrived in Greywatch six months ago.",
			"His purpose at the abandoned monastery remains deliberately unresolved. He keeps the vestry locked and answers questions with liturgy. @Captain Vale does not have proof.",
			domain.Secret, crown, "DM notes", nil, []string{"greywatch", "clergy", "church"}),
		rec("npc-osric-pell", domain.NPC, "Osric Pell",
			"Dockmaster who knows which boats skip the harbor tax.",
			"Osric sells information more readily than cargo. He owes Vale a favor from a winter drowning.",
			domain.Canon, crown, "Session 14", nil, []string{"docks", "greywatch"}),
		rec("npc-mother-branna", domain.NPC, "Mother Branna",
			"Keeps the Salted Hearth and hears every rumor twice.",
			"Branna boards the party cheaply in exchange for news from the monastery road.",
			domain.Canon, crown, "Session 13", nil, []string{"docks", "greywatch"}),
		rec("draft-sister-elayne", domain.NPC, "Sister Elayne",
			"A possible church investigator being prepared by the DM.",
			"DM-authored draft. Not yet established as true. If she arrives, she will ask after the reliquary before she asks after the dead.",
			domain.Draft, crown, "DM draft", nil, []string{"draft", "church"}),
		rec("char-irelda", domain.Character, "Irelda Thorne",
			"Ranger of the East Road; tracks by ash and broken ice.",
			"Irelda wants the crypt mapped before the church can seal it. She carries the silver key.",
			domain.Canon, crown, "Character sheet", []string{"Irelda"}, []string{"party"}),
		rec("char-brannok", domain.Character, "Brannok",
			"Village-born cleric who still prays the old Greywatch rites.",
			"Brannok will not accuse Merrow in public. He has begun copying the burial hymns from the monastery walls.",
			domain.Canon, crown, "Character sheet", nil, []string{"party", "clergy"}),
		rec("location-monastery", domain.Location, "Ruined Monastery",
			"An abandoned monastery beneath Greywatch.",
			"The party entered through the collapsed eastern transept. Frost never fully leaves the nave. @Captain Vale arrived an hour later.",
			domain.Canon, crown, "Session 17", nil, []string{"current-scene", "greywatch", "crypt"}),
		rec("location-crypt", domain.Location, "Crypt of Saint Caldren",
			"Lower ossuary under the monastery; the seal is broken.",
			"The silver key does not fit the inner gate. Something else was meant to open it. @Irelda Thorne carries the @Silver Key.",
			domain.Canon, crown, "Session 18", nil, []string{"crypt", "greywatch"}),
		rec("location-docks", domain.Location, "Greywatch Docks",
			"A short pier, two tax-skiffs, and a warehouse that never locks.",
			"Night traffic is mostly salt fish and, lately, church crates with no bill of lading.",
			domain.Canon, crown, "Session 14", nil, []string{"docks", "greywatch"}),
		rec("location-hearth", domain.Location, "The Salted Hearth",
			"Greywatch's only inn; rooms smell of tar and bread.",
			"Branna keeps a back table for the guard. The party sleeps upstairs.",
			domain.Canon, crown, "Session 13", nil, []string{"docks", "greywatch"}),
		rec("location-east-road", domain.Location, "East Road",
			"The cart track from Emberhold downs into Greywatch.",
			"Ash from the northern fires still sits in the ruts from last autumn.",
			domain.Canon, crown, "Session 12", nil, []string{"travel"}),
		rec("faction-guard", domain.Faction, "Greywatch Guard",
			"Understrength watch under Captain Vale.",
			"Eight effectives, two on the docks, the rest rotating the monastery road.",
			domain.Canon, crown, "World notes", nil, []string{"greywatch", "guard"}),
		rec("faction-church", domain.Faction, "Church of the Ashen Crown",
			"Established church; Greywatch does not trust them.",
			"They want the reliquary recovered and the crypt resealed. They have not said why both must happen in that order.",
			domain.Canon, crown, "World notes", nil, []string{"church"}),
		rec("faction-hands", domain.Faction, "Dockside Hands",
			"Loose stevedore kinship that moves unmarked crates.",
			"Osric can name three Hands; he will not name the fourth.",
			domain.Secret, crown, "DM notes", nil, []string{"docks"}),
		rec("item-silver-key", domain.Item, "Silver Key",
			"Recovered by the party; what it opens is unknown.",
			"Found near the broken crypt seal. Irelda wears it on a cord.",
			domain.Unknown, crown, "Session 17", nil, []string{"reliquary", "crypt"}),
		rec("item-seal-shard", domain.Item, "Seal Shard",
			"A palm-sized piece of the crypt's wax-and-iron seal.",
			"Still cold hours after being pocketed. The church sigil is incomplete.",
			domain.Canon, crown, "Session 18", nil, []string{"crypt", "church"}),
		rec("item-casket", domain.Item, "Reliquary Casket",
			"Empty. The lining is cut from the inside.",
			"Merrow has not been told the party found it. Vale has.",
			domain.Secret, crown, "Session 19", nil, []string{"reliquary", "crypt"}),
		rec("creature-wight", domain.Creature, "Crypt Wight",
			"A seated dead thing that stood when the seal cracked.",
			"Turned by Brannok's old hymn, not by the church rite. It will not stay down if the inner gate opens.",
			domain.Canon, crown, "Session 18", nil, []string{"crypt"}),
		rec("thread-reliquary", domain.Thread, "Missing Reliquary",
			"The monastery reliquary is missing.",
			"Vale suspects @Father Merrow, but no evidence establishes responsibility. The empty @Reliquary Casket deepens the question.",
			domain.Canon, crown, "Session 16", nil, []string{"open", "greywatch", "church"}),
		rec("thread-smuggling", domain.Thread, "Unmarked Church Crates",
			"Night boats are landing church crates with no papers.",
			"Osric saw the same wax seal as the crypt. He will talk for coin and a promise Vale is not told.",
			domain.Canon, crown, "Session 14", nil, []string{"open", "docks", "church"}),
		rec("proposal-investigator", domain.Thread, "Church Investigator Arrives",
			"The church could send an investigator after hearing of the disturbed reliquary.",
			"Generated possibility. This has not happened and is not campaign truth. If used, Sister Elayne is the likely face.",
			domain.Proposal, crown, "AI brainstorm", nil, []string{"idea", "church"}, true),
		rec("event-seal-broke", domain.Event, "Crypt seal broken",
			"The party found the ossuary seal already cracked.",
			"Frost had crept through the break. Vale arrived an hour later and blamed the party anyway.",
			domain.Canon, crown, "Session 17", nil, []string{"crypt", "greywatch"}),
		rec("event-vale-accusation", domain.Event, "Vale accuses Merrow",
			"Publicly, in the Hearth, with half the village listening.",
			"No proof. Branna threw them both out. The accusation is now village canon whether it is true or not.",
			domain.Canon, crown, "Session 16", nil, []string{"greywatch"}),
		rec("note-burials", domain.Note, "Greywatch burial customs",
			"Locals still bury their dead in the old monastery cemetery.",
			"The church graveyard on the hill has three occupants, all outsiders. The village will not use it.",
			domain.Canon, crown, "World notes", nil, []string{"greywatch", "clergy"}),
		rec("note-merrow-cover", domain.Note, "Merrow is a simple parish priest",
			"Early working theory; no longer used.",
			"Superseded after the empty casket and the crate seal matched.",
			domain.Superseded, crown, "Session 14", nil, []string{"church"}),
		rec("world-greywatch", domain.Location, "Greywatch",
			"A poor village distrustful of the established church.",
			"Locals still bury their dead in the old monastery cemetery. The docks freeze before the river does.",
			domain.Canon, world, "World canon", nil, []string{"greywatch"}),
		rec("sibling-npc", domain.NPC, "Maren of the Embers",
			"An NPC from a different campaign in the same world.",
			"This record demonstrates campaign search isolation. Maren has never been to Greywatch.",
			domain.Canon, embers, "Embers in the North", nil, []string{"embers"}),
		rec("embers-hold", domain.Location, "Emberhold",
			"A fortified ash-town north of Greywatch.",
			"The North campaign's home base. Smoke is constant; the forges never bank.",
			domain.Canon, embers, "Embers in the North", nil, []string{"embers"}),
		rec("barovia-village", domain.Location, "Village of Barovia",
			"Mist-closed village under the castle's shadow.",
			"Separate world in the library picker so vault-switching has somewhere to land.",
			domain.Canon, barovia, "World canon", nil, []string{"barovia"}),
	}

	workspace, err := domain.NewWorkspace(crown, records)
	if err != nil {
		panic(err)
	}
	workspace.Library = []domain.WorldRef{
		{
			ID:   "ashen-realms",
			Name: "The Ashen Realms",
			Campaigns: []domain.CampaignRef{
				{ID: "ashen-crown", Name: "The Ashen Crown"},
				{ID: "embers-north", Name: "Embers in the North"},
			},
		},
		{
			ID:   "barovia",
			Name: "Barovia",
			Campaigns: []domain.CampaignRef{
				{ID: "ravenloft", Name: "Mists of Ravenloft"},
			},
		},
	}

	planDocks := domain.PlannedNotes{
		ID:           "plan-docks",
		Title:        "Docks night",
		Scope:        crown,
		LocationID:   "location-docks",
		LocationName: "Greywatch Docks",
		Body:         "# Docks night\n\n#location Greywatch Docks\n\nPressure @Osric Pell about the unmarked crates.\nKeep @Captain Vale off the pier until you have a name.\nRoom at @The Salted Hearth if the watch turns up.\n",
		Links: []domain.EntityLink{
			link("npc-osric-pell", "Osric Pell"),
			link("npc-captain-vale", "Captain Vale"),
			link("location-hearth", "The Salted Hearth"),
			link("location-docks", "Greywatch Docks"),
			link("thread-smuggling", "Unmarked Church Crates"),
		},
		CreatedAt: at(2025, 12, 10, 16),
		UpdatedAt: at(2025, 12, 14, 12),
	}
	planCrypt := domain.PlannedNotes{
		ID:           "plan-crypt",
		Title:        "Crypt arc",
		Scope:        crown,
		LocationID:   "location-monastery",
		LocationName: "Ruined Monastery",
		Body:         "# Crypt arc\n\n#location Ruined Monastery\n\nReturn through the transept. @Captain Vale will be on the road.\n@Father Merrow must not see the @Silver Key until the inner gate is tried.\nIf the dead rise, @Brannok uses the old hymn — not the church rite.\n",
		Links: []domain.EntityLink{
			link("npc-captain-vale", "Captain Vale"),
			link("npc-father-merrow", "Father Merrow"),
			link("item-silver-key", "Silver Key"),
			link("char-brannok", "Brannok"),
			link("location-monastery", "Ruined Monastery"),
			link("location-crypt", "Crypt of Saint Caldren"),
		},
		PriorSessionIDs: []string{"session-docks"},
		CreatedAt:       at(2026, 1, 8, 15),
		UpdatedAt:       at(2026, 1, 19, 23),
	}
	planNext := domain.PlannedNotes{
		ID:           "plan-after-crypt",
		Title:        "After the crypt",
		Scope:        crown,
		LocationID:   "location-hearth",
		LocationName: "The Salted Hearth",
		Body:         "# After the crypt\n\n#location The Salted Hearth\n\nThe casket is empty. Decide who hears that: @Captain Vale, @Father Merrow, or neither.\n@Sister Elayne is still a draft — do not put her on the road unless the table asks for church pressure.\nFollow up the crates with @Osric Pell; the wax matched the seal.\n",
		Links: []domain.EntityLink{
			link("npc-captain-vale", "Captain Vale"),
			link("npc-father-merrow", "Father Merrow"),
			link("draft-sister-elayne", "Sister Elayne"),
			link("npc-osric-pell", "Osric Pell"),
			link("item-casket", "Reliquary Casket"),
			link("location-hearth", "The Salted Hearth"),
		},
		PriorSessionIDs: []string{"session-crypt-2", "session-crypt-1"},
		CreatedAt:       at(2026, 1, 20, 10),
		UpdatedAt:       at(2026, 8, 22, 21),
	}
	workspace.PlannedNotes = []domain.PlannedNotes{planCrypt, planNext, planDocks}

	roadStart := at(2025, 11, 8, 18)
	docksStart := at(2025, 12, 14, 19)
	crypt1Start := at(2026, 1, 12, 19)
	crypt2Start := at(2026, 1, 19, 19)
	marketStart := at(2026, 3, 2, 16)
	watchStart := at(2026, 8, 22, 18)

	workspace.Sessions = []domain.SessionRecord{
		{
			ID: "session-road", Title: "Road to Greywatch", Scope: crown,
			StartedAt: roadStart, EndedAt: end(roadStart),
			LocationID: "location-east-road", LocationName: "East Road",
			Links: []domain.EntityLink{
				link("char-irelda", "Irelda Thorne"),
				link("char-brannok", "Brannok"),
				link("location-east-road", "East Road"),
			},
			Entries: []domain.TranscriptEntry{
				entry("session-road-e1", roadStart.Add(20*time.Minute), "The East Road is ash-rutted. @Irelda Thorne picks a campsite short of the village lights.", []domain.EntityLink{link("char-irelda", "Irelda Thorne")}, nil),
				entry("session-road-e2", roadStart.Add(50*time.Minute), "Frost on the monastery hill already. Greywatch does not look like a welcome.", nil, nil),
			},
		},
		{
			ID: "session-docks", Title: "Docks night", Scope: crown,
			StartedAt: docksStart, EndedAt: end(docksStart),
			LocationID: "location-docks", LocationName: "Greywatch Docks",
			PlannedNotesID: planDocks.ID, Folder: "Greywatch/Docks",
			Links: []domain.EntityLink{
				link("npc-osric-pell", "Osric Pell"),
				link("npc-mother-branna", "Mother Branna"),
				link("npc-captain-vale", "Captain Vale"),
				link("location-docks", "Greywatch Docks"),
				link("thread-smuggling", "Unmarked Church Crates"),
			},
			Entries: []domain.TranscriptEntry{
				entry("session-docks-e1", docksStart.Add(15*time.Minute), "@Osric Pell names three Hands and stops. The fourth crate had church wax.", []domain.EntityLink{link("npc-osric-pell", "Osric Pell"), link("thread-smuggling", "Unmarked Church Crates")}, nil),
				entry("session-docks-e2", docksStart.Add(40*time.Minute), "Perception vs the pier dark.", nil, []domain.RollResult{{
					ID: "session-docks-r1", Label: "perception", Expression: "d20+4", Total: 17, Detail: "13+4", Rolls: []int{13},
				}}),
				entry("session-docks-e3", docksStart.Add(70*time.Minute), "Back at @The Salted Hearth. @Mother Branna has already heard a version.", []domain.EntityLink{link("location-hearth", "The Salted Hearth"), link("npc-mother-branna", "Mother Branna")}, nil),
			},
		},
		{
			ID: "session-crypt-1", Title: "Crypt arc", Scope: crown,
			StartedAt: crypt1Start, EndedAt: end(crypt1Start),
			LocationID: "location-monastery", LocationName: "Ruined Monastery",
			PlannedNotesID: planCrypt.ID, Folder: "Greywatch/Crypt",
			Links: []domain.EntityLink{
				link("npc-captain-vale", "Captain Vale"),
				link("char-irelda", "Irelda Thorne"),
				link("char-brannok", "Brannok"),
				link("location-monastery", "Ruined Monastery"),
				link("item-silver-key", "Silver Key"),
				link("creature-wight", "Crypt Wight"),
			},
			Entries: []domain.TranscriptEntry{
				entry("session-crypt-1-e1", crypt1Start.Add(10*time.Minute), "@Captain Vale meets the party on the monastery road and does not follow them in.", []domain.EntityLink{link("npc-captain-vale", "Captain Vale")}, nil),
				entry("session-crypt-1-e2", crypt1Start.Add(35*time.Minute), "The @Silver Key will not turn in the inner gate. @Irelda Thorne pockets a @Seal Shard.", []domain.EntityLink{link("item-silver-key", "Silver Key"), link("char-irelda", "Irelda Thorne"), link("item-seal-shard", "Seal Shard")}, nil),
				entry("session-crypt-1-e3", crypt1Start.Add(80*time.Minute), "The @Crypt Wight stands. @Brannok uses the old hymn.", []domain.EntityLink{link("creature-wight", "Crypt Wight"), link("char-brannok", "Brannok")}, []domain.RollResult{{
					ID: "session-crypt-1-r1", Label: "turn", Expression: "d20+3", Total: 18, Detail: "15+3", Rolls: []int{15},
				}}),
			},
		},
		{
			ID: "session-crypt-2", Title: "Crypt arc · 2026-01-19", Scope: crown,
			StartedAt: crypt2Start, EndedAt: end(crypt2Start),
			LocationID: "location-crypt", LocationName: "Crypt of Saint Caldren",
			PlannedNotesID: planCrypt.ID, Folder: "Greywatch/Crypt",
			Links: []domain.EntityLink{
				link("npc-father-merrow", "Father Merrow"),
				link("char-irelda", "Irelda Thorne"),
				link("char-brannok", "Brannok"),
				link("location-crypt", "Crypt of Saint Caldren"),
				link("item-casket", "Reliquary Casket"),
				link("thread-reliquary", "Missing Reliquary"),
			},
			Entries: []domain.TranscriptEntry{
				entry("session-crypt-2-e1", crypt2Start.Add(25*time.Minute), "Inner chamber: the @Reliquary Casket is empty, lining cut from inside.", []domain.EntityLink{link("item-casket", "Reliquary Casket"), link("thread-reliquary", "Missing Reliquary")}, nil),
				entry("session-crypt-2-e2", crypt2Start.Add(55*time.Minute), "@Father Merrow is in the nave when they climb out. He asks if they found peace. He does not ask about a casket.", []domain.EntityLink{link("npc-father-merrow", "Father Merrow")}, nil),
				entry("session-crypt-2-e3", crypt2Start.Add(90*time.Minute), "Party agrees not to tell Vale tonight. The empty casket stays in Irelda's pack.", nil, nil),
			},
		},
		{
			ID: "session-market", Title: "Market day", Scope: crown,
			StartedAt: marketStart, EndedAt: end(marketStart),
			LocationID: "world-greywatch", LocationName: "Greywatch",
			Links: []domain.EntityLink{
				link("npc-mother-branna", "Mother Branna"),
				link("npc-captain-vale", "Captain Vale"),
				link("char-irelda", "Irelda Thorne"),
			},
			Entries: []domain.TranscriptEntry{
				entry("session-market-e1", marketStart.Add(30*time.Minute), "Greywatch market. @Mother Branna prices salt like a threat. @Captain Vale watches from the well.", []domain.EntityLink{link("npc-mother-branna", "Mother Branna"), link("npc-captain-vale", "Captain Vale")}, nil),
			},
		},
		{
			ID: "session-watch", Title: "Watch rotation", Scope: crown,
			StartedAt: watchStart, EndedAt: end(watchStart),
			LocationID: "location-hearth", LocationName: "The Salted Hearth",
			Links: []domain.EntityLink{
				link("npc-captain-vale", "Captain Vale"),
				link("faction-guard", "Greywatch Guard"),
				link("char-brannok", "Brannok"),
			},
			Entries: []domain.TranscriptEntry{
				entry("session-watch-e1", watchStart.Add(15*time.Minute), "@Captain Vale asks @Brannok to walk a watch rotation. The guard is two short.", []domain.EntityLink{link("npc-captain-vale", "Captain Vale"), link("char-brannok", "Brannok"), link("faction-guard", "Greywatch Guard")}, nil),
			},
		},
	}

	reconAt := crypt2Start.Add(4 * time.Hour)
	workspace.Reconciliations = []domain.ReconciliationRecord{
		{
			ID: "recon-session-crypt-2", SessionID: "session-crypt-2",
			Title: "Reconcile Crypt arc · 2026-01-19", CreatedAt: reconAt,
			Items: []domain.ReconciliationItem{
				{
					ID: "recon-session-crypt-2-link-casket", EntryID: "session-crypt-2-e1",
					RecordID: "item-casket", Kind: domain.ReconLinkReview,
					Summary: "Review @Reliquary Casket from transcript", Status: domain.ReconApproved, CreatedAt: reconAt,
				},
				{
					ID: "recon-session-crypt-2-note", EntryID: "session-crypt-2-e3",
					Kind: domain.ReconTranscriptNote, Summary: "Party is withholding the empty casket from Vale",
					Status: domain.ReconPending, CreatedAt: reconAt,
				},
				{
					ID: "recon-session-crypt-2-draft", EntryID: "session-crypt-2-e2",
					RecordID: "draft-sister-elayne", Kind: domain.ReconPromoteDraft,
					Summary: "Consider promoting Sister Elayne if the church answers the crypt",
					Status:  domain.ReconDeferred, CreatedAt: reconAt,
				},
			},
		},
		{
			ID: "recon-session-docks", SessionID: "session-docks",
			Title: "Reconcile Docks night", CreatedAt: docksStart.Add(4 * time.Hour),
			Items: []domain.ReconciliationItem{
				{
					ID: "recon-session-docks-link-osric", EntryID: "session-docks-e1",
					RecordID: "npc-osric-pell", Kind: domain.ReconLinkReview,
					Summary: "Review @Osric Pell from transcript", Status: domain.ReconApproved,
					CreatedAt: docksStart.Add(4 * time.Hour),
				},
			},
		},
	}

	workspace.Collections = []domain.Collection{
		{
			ID: "col-greywatch", Title: "Greywatch circle", Scope: crown,
			RecordIDs: []string{
				"npc-captain-vale", "npc-father-merrow", "location-monastery",
				"thread-reliquary", "faction-guard", "world-greywatch",
			},
		},
		{
			ID: "col-crypt", Title: "Crypt pressure", Scope: crown,
			RecordIDs: []string{
				"npc-captain-vale", "draft-sister-elayne", "item-silver-key",
				"location-crypt", "creature-wight", "item-casket", "thread-reliquary",
			},
		},
		{
			ID: "col-party", Title: "The table", Scope: crown,
			RecordIDs: []string{"char-irelda", "char-brannok"},
		},
		{
			ID: "col-church", Title: "Church mystery", Scope: crown,
			RecordIDs: []string{
				"npc-father-merrow", "draft-sister-elayne", "proposal-investigator",
				"faction-church", "thread-reliquary", "item-casket",
			},
		},
	}

	return workspace
}

func rec(id string, typ domain.EntityType, title, summary, body string, auth domain.Authority, scope domain.Scope, source string, aliases, tags []string, ai ...bool) domain.Record {
	isAI := false
	if len(ai) > 0 {
		isAI = ai[0]
	}
	return domain.Record{
		ID: id, Type: typ, Title: title, Summary: summary, Body: body,
		Authority: auth, Scope: scope, Source: source, Aliases: aliases, Tags: tags,
		IsAIContent: isAI,
	}
}

func entry(id string, at time.Time, text string, links []domain.EntityLink, rolls []domain.RollResult) domain.TranscriptEntry {
	return domain.TranscriptEntry{ID: id, Text: text, CreatedAt: at, Links: links, Rolls: rolls}
}
