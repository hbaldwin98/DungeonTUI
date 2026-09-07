package ingest

import (
	"os"
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
)

const fakeBestiary = `# Monster Manual (2025)

## Stat Block Overview

How blocks work.

## Goblins

<div data-rd-name="Goblin Warrior" data-statblock-hash="goblin%20warrior_xmm">
<i>Loading "Goblin Warrior"...</i>
</div>

Mischievous raiders who travel in packs.

## Bugbears

Hairy goblinoids who hunt at night.
`

const fakeAdventure = `# Lost Test of Phandelver

### Important NPCs

| Name | Role |
|------|------|
| Toblen Stonehill | Innkeeper. |
| **Sildar Hallwinter** | Knights of Neverwinter. |

# Goblin Arrows

The party is ambushed by **goblins** on the trail.

## Cragmaw Hideout

A cave hideout.

### 1. Cave Mouth

A stream flows from the cave. Two **goblins** watch from the thicket.

### 2. Goblin Blind

More **goblins** here.

# Appendix B: Monsters

## Goblin

Adventure goblin stats live here.
`

func testWorkspace(t *testing.T) domain.Workspace {
	t.Helper()
	scope := domain.Scope{WorldID: "fr", WorldName: "Forgotten Realms", CampaignID: "lmop", Campaign: "Lost Mine"}
	ws, err := domain.NewWorkspace(scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.Library = []domain.WorldRef{{
		ID: "fr", Name: "Forgotten Realms",
		Campaigns: []domain.CampaignRef{{ID: "lmop", Name: "Lost Mine"}},
	}}
	return ws
}

func TestParseDetectsBestiaryAndStripsWidgets(t *testing.T) {
	book := Parse(fakeBestiary, "")
	if book.Kind != domain.SourceBestiary {
		t.Fatalf("kind=%s", book.Kind)
	}
	if book.Edition != "2025" {
		t.Fatalf("edition=%q", book.Edition)
	}
	if strings.Contains(strings.Join(bodies(book), "\n"), "Loading") {
		t.Fatal("expected HTML loader text to be stripped")
	}
	var goblins ParsedEntry
	for _, e := range book.Entries {
		if e.Title == "Goblins" {
			goblins = e
		}
	}
	if goblins.Title == "" {
		t.Fatal("missing Goblins entry")
	}
	if !containsAlias(book.Aliases["Goblins"], "Goblin Warrior") {
		t.Fatalf("aliases=%#v", book.Aliases["Goblins"])
	}
}

func TestApplyBestiaryCreatesLibraryCreatures(t *testing.T) {
	ws := testWorkspace(t)
	ws, report, err := Apply(ws, fakeBestiary, Options{Scope: ws.Scope})
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != domain.SourceBestiary || report.Records < 2 {
		t.Fatalf("report=%#v", report)
	}
	var goblins domain.Record
	for _, r := range ws.Records {
		if r.Title == "Goblins" {
			goblins = r
		}
	}
	if goblins.ID == "" || goblins.Type != domain.Creature {
		t.Fatalf("goblins=%#v", goblins)
	}
	if goblins.Scope.CampaignID != "" || goblins.SourceID != report.SourceID {
		t.Fatalf("library scope=%#v source=%s", goblins.Scope, goblins.SourceID)
	}
	if goblins.Folder != "Monster Manual (2025)/Creatures" {
		t.Fatalf("folder=%q", goblins.Folder)
	}
	enabled := ws.EnabledSourceIDs(ws.Scope)
	if len(enabled) != 1 || enabled[0] != report.SourceID {
		t.Fatalf("enabled=%#v", enabled)
	}
	if !domain.RecordVisibleIn(goblins, ws.Scope, enabled) {
		t.Fatal("enabled bestiary creature should be campaign-visible")
	}
}

func TestApplyAdventureExtractsRoomsNPCsAndPrep(t *testing.T) {
	ws := testWorkspace(t)
	ws, _, err := Apply(ws, fakeBestiary, Options{Scope: ws.Scope})
	if err != nil {
		t.Fatal(err)
	}
	ws, report, err := Apply(ws, fakeAdventure, Options{Scope: ws.Scope})
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != domain.SourceAdventure {
		t.Fatalf("kind=%s", report.Kind)
	}
	titles := map[string]domain.EntityType{}
	for _, r := range ws.Records {
		if r.SourceID == report.SourceID {
			titles[r.Title] = r.Type
		}
	}
	for _, want := range []string{"Toblen Stonehill", "Sildar Hallwinter", "Cragmaw Hideout", "1. Cave Mouth", "Goblin"} {
		if _, ok := titles[want]; !ok {
			t.Fatalf("missing %q in %#v", want, titles)
		}
	}
	if titles["Toblen Stonehill"] != domain.NPC {
		t.Fatalf("Toblen type=%s", titles["Toblen Stonehill"])
	}
	if _, dup := titles["Cragmaw Hideout — 1. Cave Mouth"]; dup {
		t.Fatal("rooms should not be duplicated as prefixed sibling titles")
	}
	if len(ws.PlannedNotes) == 0 {
		t.Fatal("expected prep notes for adventure parts")
	}
	var cave domain.Record
	for _, r := range ws.Records {
		if r.Title == "1. Cave Mouth" {
			cave = r
		}
	}
	if !strings.Contains(cave.Folder, "Cragmaw Hideout") {
		t.Fatalf("cave should nest under Cragmaw Hideout, folder=%q", cave.Folder)
	}
	if !strings.Contains(cave.Body, "@Goblins") && !strings.Contains(cave.Body, "@Goblin") {
		t.Fatalf("expected creature association in body: %q", cave.Body)
	}
}

func TestReimportReplacesSourceRecords(t *testing.T) {
	ws := testWorkspace(t)
	ws, _, err := Apply(ws, fakeBestiary, Options{Scope: ws.Scope})
	if err != nil {
		t.Fatal(err)
	}
	first := len(ws.Records)
	ws, _, err = Apply(ws, fakeBestiary, Options{Scope: ws.Scope})
	if err != nil {
		t.Fatal(err)
	}
	if len(ws.Records) != first {
		t.Fatalf("reimport duplicated records: %d then %d", first, len(ws.Records))
	}
}

func TestApplyDownloadsIfPresent(t *testing.T) {
	mm := "/mnt/c/Users/hunte/Downloads/Monster Manual (2025).md"
	lmop := "/mnt/c/Users/hunte/Downloads/Lost Mine of Phandelver.md"
	if _, err := os.Stat(mm); err != nil {
		t.Skip("Monster Manual markdown not on disk")
	}
	if _, err := os.Stat(lmop); err != nil {
		t.Skip("LMoP markdown not on disk")
	}
	ws := testWorkspace(t)
	ws, mmReport, err := ApplyFile(ws, mm, Options{Scope: ws.Scope, Kind: domain.SourceBestiary})
	if err != nil {
		t.Fatal(err)
	}
	if mmReport.Records < 50 {
		t.Fatalf("expected dozens of MM creatures, got %d", mmReport.Records)
	}
	ws, advReport, err := ApplyFile(ws, lmop, Options{Scope: ws.Scope, Kind: domain.SourceAdventure})
	if err != nil {
		t.Fatal(err)
	}
	if advReport.Records < 20 || advReport.Planned < 3 {
		t.Fatalf("adventure report=%#v", advReport)
	}
	enabled := ws.EnabledSourceIDs(ws.Scope)
	var goblins domain.Record
	for _, record := range ws.Records {
		if record.Title == "Goblins" && record.SourceID == mmReport.SourceID {
			goblins = record
		}
	}
	if goblins.ID == "" || !domain.RecordVisibleIn(goblins, ws.Scope, enabled) {
		t.Fatalf("MM Goblins not campaign-visible; enabled=%v", enabled)
	}
	t.Logf("MM %d creatures; LMoP %d records %d prep %d links", mmReport.Records, advReport.Records, advReport.Planned, advReport.Linked)
}

func TestApplyFiveEIngestsChosenBook(t *testing.T) {
	ws := testWorkspace(t)
	fetcher := fivetools.MapFetcher{
		"data/adventures.json":            []byte(`{"adventure":[]}`),
		"data/books.json":                 []byte(`{"book":[{"id":"XMM","name":"Monster Manual (2025)","group":"core","published":"2025-02-18"}]}`),
		"data/bestiary/index.json":        []byte(`{"XMM":"bestiary-xmm.json"}`),
		"data/spells/index.json":          []byte(`{}`),
		"data/bestiary/bestiary-xmm.json": []byte(`{"monster":[{"name":"Goblin Warrior","source":"XMM","ac":[15],"hp":{"average":10,"formula":"3d6"},"str":8,"dex":15,"con":10,"int":10,"wis":8,"cha":8}]}`),
	}
	ws, report, err := ApplyFiveE(ws, "5e:XMM", Options{Scope: ws.Scope, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceID != "src-5e-xmm" || report.Records != 1 {
		t.Fatalf("report=%#v", report)
	}
	var goblin domain.Record
	for _, record := range ws.Records {
		if record.Title == "Goblin Warrior" {
			goblin = record
		}
	}
	if goblin.Type != domain.Creature || goblin.Scope.CampaignID != "" {
		t.Fatalf("library creature=%#v", goblin)
	}
	if !strings.Contains(goblin.Body, "**AC** 15") {
		t.Fatalf("body=%q", goblin.Body)
	}
	if !ws.SourceEnabled(ws.Scope, report.SourceID) {
		t.Fatal("imported book should be enabled for the campaign")
	}
}

func bodies(book ParsedBook) []string {
	out := make([]string, 0, len(book.Entries))
	for _, e := range book.Entries {
		out = append(out, e.Body)
	}
	return out
}

func containsAlias(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
