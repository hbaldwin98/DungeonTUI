package domain

import "testing"

func TestCollectionToggleAndScope(t *testing.T) {
	scope := Scope{WorldID: "w", CampaignID: "c"}
	col := Collection{ID: "col-1", Title: "Greywatch", Scope: scope}
	if err := col.Validate(); err != nil {
		t.Fatal(err)
	}
	col = col.Add("npc-vale")
	col = col.Add("npc-vale")
	if !col.Has("npc-vale") || len(col.RecordIDs) != 1 {
		t.Fatalf("dedupe failed: %#v", col.RecordIDs)
	}
	col = col.Toggle("npc-merrow")
	if !col.Has("npc-merrow") {
		t.Fatal("expected merrow added")
	}
	col = col.Toggle("npc-vale")
	if col.Has("npc-vale") {
		t.Fatal("expected vale removed")
	}
	other := Collection{ID: "col-x", Title: "Elsewhere", Scope: Scope{WorldID: "w", CampaignID: "x"}, RecordIDs: []string{"npc-vale"}}
	cols := []Collection{col, other}
	scoped := ScopedCollections(cols, scope)
	if len(scoped) != 1 || scoped[0].ID != "col-1" {
		t.Fatalf("scoped=%#v", scoped)
	}
	found := CollectionsContaining(cols, scope, "npc-merrow")
	if len(found) != 1 || found[0].ID != "col-1" {
		t.Fatalf("containing=%#v", found)
	}
}
