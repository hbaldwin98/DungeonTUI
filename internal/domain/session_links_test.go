package domain

import "testing"

func TestSessionAssociateDedupesByRecordID(t *testing.T) {
	session := SessionRecord{ID: "s1"}
	session = session.Associate(EntityLink{Text: "Vale", RecordID: "npc-vale"})
	session = session.Associate(EntityLink{Text: "Captain Vale", RecordID: "npc-vale"})
	if len(session.Links) != 1 {
		t.Fatalf("expected 1 link, got %#v", session.Links)
	}
}

func TestSessionDisassociateRemovesLink(t *testing.T) {
	session := SessionRecord{ID: "s1", Links: []EntityLink{
		{Text: "Vale", RecordID: "npc-vale"},
		{Text: "Merrow", RecordID: "npc-merrow"},
	}}
	session = session.Disassociate("npc-vale")
	if len(session.Links) != 1 || session.Links[0].RecordID != "npc-merrow" {
		t.Fatalf("unexpected links %#v", session.Links)
	}
}

func TestSessionSeedLinksFromPlan(t *testing.T) {
	plan := PlannedNotes{Links: []EntityLink{
		{Text: "Vale", RecordID: "npc-vale"},
		{Text: "Vale", RecordID: "npc-vale"},
	}}
	session := SessionRecord{ID: "s1"}.SeedLinksFrom(plan)
	if len(session.Links) != 1 || session.Links[0].RecordID != "npc-vale" {
		t.Fatalf("seeded links=%#v", session.Links)
	}
}
