package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
)

const fakeBestiary = `# Test Bestiary (2025)

## Stat Block Overview

How blocks work.

## Cave Rascals

<div data-rd-name="Cave Rascal" data-statblock-hash="cave%20rascal_bst">
<i>Loading "Cave Rascal"...</i>
</div>

Mischievous raiders who travel in packs.

## Night Hunters

Hairy hunters who hunt at night.
`

const fakeAdventure = `# The Hollow Crown

### Important NPCs

| Name | Role |
|------|------|
| Mira Holt | Innkeeper. |
| **Captain Reed** | Town guard. |

# The Ambush

The party is ambushed by **cave rascals** on the trail.

## The Hideout

A cave hideout.

### 1. Cave Mouth

A stream flows from the cave. Two **cave rascals** watch from the thicket.

### 2. Watch Post

More **cave rascals** here.

# Appendix B: Monsters

## Cave Rascal

Adventure creature stats live here.
`

func testWorkspace(t *testing.T) domain.Workspace {
	t.Helper()
	scope := domain.Scope{WorldID: "test-world", WorldName: "Test World", CampaignID: "test-campaign", Campaign: "Test Campaign"}
	ws, err := domain.NewWorkspace(scope, nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.Library = []domain.WorldRef{{
		ID: "test-world", Name: "Test World",
		Campaigns: []domain.CampaignRef{{ID: "test-campaign", Name: "Test Campaign"}},
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
	var rascals ParsedEntry
	for _, e := range book.Entries {
		if e.Title == "Cave Rascals" {
			rascals = e
		}
	}
	if rascals.Title == "" {
		t.Fatal("missing Cave Rascals entry")
	}
	if !containsAlias(book.Aliases["Cave Rascals"], "Cave Rascal") {
		t.Fatalf("aliases=%#v", book.Aliases["Cave Rascals"])
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
	var rascals domain.Record
	for _, r := range ws.Records {
		if r.Title == "Cave Rascals" {
			rascals = r
		}
	}
	if rascals.ID == "" || rascals.Type != domain.Creature {
		t.Fatalf("rascals=%#v", rascals)
	}
	if rascals.Scope.CampaignID != "" || rascals.SourceID != report.SourceID {
		t.Fatalf("library scope=%#v source=%s", rascals.Scope, rascals.SourceID)
	}
	if rascals.Folder != "Test Bestiary (2025)/Creatures" {
		t.Fatalf("folder=%q", rascals.Folder)
	}
	enabled := ws.EnabledSourceIDs(ws.Scope)
	if len(enabled) != 1 || enabled[0] != report.SourceID {
		t.Fatalf("enabled=%#v", enabled)
	}
	if !domain.RecordVisibleIn(rascals, ws.Scope, enabled) {
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
	for _, want := range []string{"Mira Holt", "Captain Reed", "The Hideout", "1. Cave Mouth", "Cave Rascal"} {
		if _, ok := titles[want]; !ok {
			t.Fatalf("missing %q in %#v", want, titles)
		}
	}
	if titles["Mira Holt"] != domain.NPC {
		t.Fatalf("Mira type=%s", titles["Mira Holt"])
	}
	if _, dup := titles["The Hideout — 1. Cave Mouth"]; dup {
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
	if !strings.Contains(cave.Folder, "The Hideout") {
		t.Fatalf("cave should nest under The Hideout, folder=%q", cave.Folder)
	}
	if !strings.Contains(cave.Folder, "The Ambush") {
		t.Fatalf("cave should nest under The Ambush, folder=%q", cave.Folder)
	}
	if !strings.Contains(cave.Body, "@Cave Rascals") && !strings.Contains(cave.Body, "@Cave Rascal") {
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

func TestApplyAdventureMarkdownDoesNotFileUnderIntroduction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "The Hollow Crown.md")
	markdown := `# Introduction

Welcome.

### Running the Adventure

How to run the game.

### Background

Miners found a hollow.

# The Ambush

The wagon is ambushed.

## The Hideout

A cave hideout.

### 1. Cave Mouth

Two cave rascals watch the cave.

# Millhaven

A frontier town.

### The Mill Inn

A stout inn.

# Appendix B: Monsters

## Cave Rascal

Adventure creature stats live here.
`
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := testWorkspace(t)
	ws, report, err := ApplyFile(ws, path, Options{Scope: ws.Scope, Kind: domain.SourceAdventure})
	if err != nil {
		t.Fatal(err)
	}
	if report.Title != "The Hollow Crown" {
		t.Fatalf("title=%q", report.Title)
	}
	if strings.EqualFold(report.Title, "Introduction") {
		t.Fatal("source should not be named Introduction")
	}
	titles := map[string]domain.Record{}
	for _, record := range ws.Records {
		if record.SourceID == report.SourceID {
			titles[record.Title] = record
		}
	}
	if rec, ok := titles["Introduction"]; !ok || rec.Type != domain.Note {
		t.Fatalf("introduction should be a note: %#v", titles["Introduction"])
	}
	if rec, ok := titles["Running the Adventure"]; ok && rec.Type == domain.Location {
		t.Fatal("intro sections should not become locations")
	}
	if rec, ok := titles["Background"]; ok && rec.Type == domain.Location {
		t.Fatal("intro background should not become a location")
	}
	inn, ok := titles["The Mill Inn"]
	if !ok || inn.Type != domain.Location {
		t.Fatalf("expected The Mill Inn, got %#v", titles)
	}
	if !strings.Contains(inn.Folder, "Millhaven") {
		t.Fatalf("inn should nest under Millhaven, folder=%q", inn.Folder)
	}
	if strings.Contains(inn.Folder, "Introduction") {
		t.Fatalf("inn should not file under Introduction: %q", inn.Folder)
	}
	cave := titles["1. Cave Mouth"]
	if cave.ID == "" || !strings.Contains(cave.Folder, "The Hideout") || strings.Contains(cave.Folder, "Introduction") {
		t.Fatalf("cave folder=%q", cave.Folder)
	}
}

func TestApplyFiveECachesAdventureWithoutWikiRows(t *testing.T) {
	ws := testWorkspace(t)
	fetcher := fivetools.MapFetcher{
		"data/adventures.json":              []byte(`{"adventure":[{"id":"ADV","name":"The Hollow Crown","published":"2020-01-01"}]}`),
		"data/adventure/adventure-adv.json": []byte(`{"data":[{"type":"section","name":"Millhaven","entries":["A frontier town. {@creature Mira Holt|ADV} keeps the inn."]}]}`),
	}
	ws, report, err := ApplyFiveE(ws, "5e:ADV", Options{Scope: ws.Scope, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	if report.SourceID != "src-5e-adv" || !report.Reference || report.Records != 0 {
		t.Fatalf("report=%#v", report)
	}
	if len(ws.Records) != 0 {
		t.Fatalf("adventure leaked into wiki: %#v", ws.Records)
	}
	if !ws.SourceEnabled(ws.Scope, report.SourceID) {
		t.Fatal("imported adventure should be enabled for the campaign")
	}
	_, _, err = ApplyFiveE(ws, "5e:BST", Options{Scope: ws.Scope, Fetcher: fetcher})
	if err == nil {
		t.Fatal("rules books must not import")
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

func TestApplyReportsProgress(t *testing.T) {
	ws := testWorkspace(t)
	var stages []Stage
	ws, _, err := Apply(ws, fakeAdventure, Options{
		Scope: ws.Scope,
		Progress: func(p Progress) {
			if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
				stages = append(stages, p.Stage)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := fmt.Sprintf("%v", stages)
	if !strings.Contains(joined, string(StageConvert)) || !strings.Contains(joined, string(StageClassify)) || !strings.Contains(joined, string(StageWrite)) {
		t.Fatalf("expected convert/classify/write stages, got %v", stages)
	}
}
