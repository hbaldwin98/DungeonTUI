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
