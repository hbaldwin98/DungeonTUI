package domain

import "testing"

func TestMentionsInLongestMatchAndBroken(t *testing.T) {
	records := []Record{
		{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Authority: Canon, Aliases: []string{"Alaric Vale"}},
		{ID: "npc-merrow", Type: NPC, Title: "Father Merrow", Authority: Secret},
	}
	text := "Ask @Captain Vale about @Father Merrow and @Ghost Who."
	mentions := MentionsIn(text, records)
	if len(mentions) != 3 {
		t.Fatalf("mentions=%#v", mentions)
	}
	if mentions[0].RecordID != "npc-vale" || mentions[0].Text != "Captain Vale" {
		t.Fatalf("vale=%#v", mentions[0])
	}
	if mentions[1].RecordID != "npc-merrow" {
		t.Fatalf("merrow=%#v", mentions[1])
	}
	if mentions[2].RecordID != "" || mentions[2].Text != "Ghost Who" {
		t.Fatalf("broken should keep the unmatched name, got %#v", mentions[2])
	}

	alias := MentionsIn("Meet @Alaric Vale at dusk.", records)
	if len(alias) != 1 || alias[0].RecordID != "npc-vale" {
		t.Fatalf("alias mention=%#v", alias)
	}

	resolved, broken := EntityOutgoingRefs(Record{
		ID:      "note-1",
		Title:   "Hook",
		Summary: "Pressure @Captain Vale.",
		Body:    "If @Nobody answers, try @Father Merrow.",
	}, records)
	if len(resolved) != 2 || len(broken) != 1 || broken[0].Text != "Nobody" {
		t.Fatalf("outgoing resolved=%#v broken=%#v", resolved, broken)
	}
}

func TestWikiBacklinksAndMissingTarget(t *testing.T) {
	records := []Record{
		{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Authority: Canon, Body: "Distrusts @Father Merrow."},
		{ID: "npc-merrow", Type: NPC, Title: "Father Merrow", Authority: Secret, Body: "Watched by @Captain Vale."},
	}
	ws := Workspace{Records: records}
	links := EntityBacklinks(ws, "npc-merrow")
	if len(links) != 1 || links[0].Kind != BacklinkWiki || links[0].ID != "npc-vale" {
		t.Fatalf("expected wiki backlink from Vale, got %#v", links)
	}

	ws.Records = ws.Records[:1] // Merrow removed
	_, broken := EntityOutgoingRefs(ws.Records[0], ws.Records)
	if len(broken) != 1 || broken[0].Text != "Father Merrow" {
		t.Fatalf("deleted target should become a broken mention, got %#v", broken)
	}
}
