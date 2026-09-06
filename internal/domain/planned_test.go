package domain

import (
	"strings"
	"testing"
)

func TestParsePlannedBodyExtractsLinksAndLocation(t *testing.T) {
	records := []Record{
		{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Authority: Canon},
		{ID: "loc-crypt", Type: Location, Title: "Greywatch Crypt", Authority: Canon},
		{ID: "item-key", Type: Item, Title: "Silver Key", Authority: Draft},
	}
	body := `# Next sit

Meet @Captain Vale at the crypt with the @Silver Key.

#location Greywatch Crypt

- Pressure the relic rumor
`
	links, locationID, locationName := ParsePlannedBody(body, records)
	if locationID != "loc-crypt" || locationName != "Greywatch Crypt" {
		t.Fatalf("location=%q %q", locationID, locationName)
	}
	if len(links) != 2 {
		t.Fatalf("links=%#v", links)
	}
	ids := map[string]bool{}
	for _, link := range links {
		ids[link.RecordID] = true
	}
	if !ids["npc-vale"] || !ids["item-key"] {
		t.Fatalf("expected vale and key links, got %#v", links)
	}
	if !strings.Contains(body, "@Captain Vale") || !strings.Contains(body, "#location Greywatch Crypt") {
		t.Fatal("parse must not mutate caller body semantics")
	}
}

func TestRefreshPlannedLinksLeavesBodyIntact(t *testing.T) {
	body := "Talk to @Captain Vale\n#location Greywatch Crypt\n"
	notes := PlannedNotes{
		ID:    "plan-1",
		Title: "Session 4 prep",
		Body:  body,
	}
	records := []Record{
		{ID: "npc-vale", Type: NPC, Title: "Captain Vale", Authority: Canon},
		{ID: "loc-crypt", Type: Location, Title: "Greywatch Crypt", Authority: Canon},
	}
	updated := notes.RefreshPlannedLinks(records)
	if updated.Body != body {
		t.Fatalf("body changed: %q", updated.Body)
	}
	if len(updated.Links) != 1 || updated.LocationID != "loc-crypt" {
		t.Fatalf("refresh failed: %#v", updated)
	}
	if err := updated.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultPriorSessionIDsSkipsLiveAndOtherCampaigns(t *testing.T) {
	ended := mustParseTime(t, "2026-01-02T18:00:00Z")
	older := mustParseTime(t, "2026-01-01T18:00:00Z")
	scope := Scope{WorldID: "w", CampaignID: "c"}
	sessions := []SessionRecord{
		{ID: "live", Title: "Now", Scope: scope, StartedAt: mustParseTime(t, "2026-01-03T17:00:00Z")},
		{ID: "other", Title: "Elsewhere", Scope: Scope{WorldID: "w", CampaignID: "x"}, StartedAt: older, EndedAt: &older},
		{ID: "s1", Title: "Night one", Scope: scope, StartedAt: older, EndedAt: &older, LocationName: "Crypt",
			Links: []EntityLink{{Text: "Vale", RecordID: "npc-vale"}}},
		{ID: "s2", Title: "Night two", Scope: scope, StartedAt: mustParseTime(t, "2026-01-02T17:00:00Z"), EndedAt: &ended,
			Entries: []TranscriptEntry{{Text: "@Captain Vale opens the door"}}},
	}
	ids := DefaultPriorSessionIDs(sessions, scope, 3)
	if len(ids) != 2 || ids[0] != "s2" || ids[1] != "s1" {
		t.Fatalf("expected newest ended first, got %#v", ids)
	}
	sits := ResolvePriorSits(sessions, []Record{{ID: "npc-vale", Title: "Captain Vale"}}, ids)
	if len(sits) != 2 {
		t.Fatalf("sits=%#v", sits)
	}
	if sits[0].ID != "s2" || sits[0].LastLine != "@Captain Vale opens the door" {
		t.Fatalf("unexpected newest sit %#v", sits[0])
	}
	if len(sits[1].Cast) != 1 || sits[1].Cast[0] != "Captain Vale" {
		t.Fatalf("expected resolved cast, got %#v", sits[1])
	}
	if !strings.Contains(sits[1].Summary(), "Night one") || !strings.Contains(sits[1].Summary(), "Captain Vale") {
		t.Fatalf("summary=%q", sits[1].Summary())
	}
}
