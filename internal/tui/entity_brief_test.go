package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

// briefModel opens the demo campaign at a given size with the detail pane
// focused, which is how a DM reads an entity at the table.
func briefModel(t *testing.T, width, height int) Model {
	t.Helper()
	model := New()
	model.width = width
	model.height = height
	model.layout.Focus = prefs.PaneDetail
	return model
}

func selectByID(t *testing.T, model *Model, id string) domain.Record {
	t.Helper()
	for _, record := range model.workspace.Records {
		if record.ID == id {
			model.selectRecord(record)
			return record
		}
	}
	t.Fatalf("no fixture record %q", id)
	return domain.Record{}
}

func detailPlain(model Model) string {
	return stripANSIForTest(model.renderDetail())
}

// sectionOrder reports the order the named headings appear in the pane.
func sectionOrder(detail string, headings ...string) []string {
	type placed struct {
		at   int
		name string
	}
	found := make([]placed, 0, len(headings))
	for _, heading := range headings {
		if at := strings.Index(detail, heading); at >= 0 {
			found = append(found, placed{at: at, name: heading})
		}
	}
	for i := 1; i < len(found); i++ {
		for j := i; j > 0 && found[j].at < found[j-1].at; j-- {
			found[j], found[j-1] = found[j-1], found[j]
		}
	}
	order := make([]string, 0, len(found))
	for _, entry := range found {
		order = append(order, entry.name)
	}
	return order
}

func TestEntityDetailLeadsWithIdentityThenRelevance(t *testing.T) {
	model := briefModel(t, 120, 40)
	selectByID(t, &model, "npc-captain-vale")
	detail := detailPlain(model)

	title := strings.Index(detail, "Captain Vale")
	why := strings.Index(detail, "WHY NOW")
	details := strings.Index(detail, "DETAILS")
	if title < 0 || why < 0 || details < 0 {
		t.Fatalf("detail is missing a section: %q", detail)
	}
	if !(title < why && why < details) {
		t.Fatalf("identity, relevance, then metadata: %q", detail)
	}
	// Metadata must remain visible, just not first.
	for _, want := range []string{"SCOPE", "AUTHORITY", "SOURCE", "TAGS", "COLLECTIONS"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("metadata %q must stay visible: %q", want, detail)
		}
	}
	if strings.Index(detail, "SCOPE") < why {
		t.Fatalf("scope must not precede relevance: %q", detail)
	}
}

func TestEntityDetailScanOrderFollowsTheEntityType(t *testing.T) {
	headings := []string{"WHY NOW", "LAST SEEN", "CONNECTED", "WHAT CHANGED", "DETAILS"}
	cases := []struct {
		id   string
		want []string
	}{
		{"npc-captain-vale", []string{"WHY NOW", "LAST SEEN", "CONNECTED", "WHAT CHANGED", "DETAILS"}},
		{"location-monastery", []string{"WHY NOW", "LAST SEEN", "CONNECTED", "WHAT CHANGED", "DETAILS"}},
		{"item-silver-key", []string{"WHY NOW", "CONNECTED", "LAST SEEN", "WHAT CHANGED", "DETAILS"}},
		{"thread-reliquary", []string{"WHY NOW", "WHAT CHANGED", "CONNECTED", "LAST SEEN", "DETAILS"}},
	}
	for _, test := range cases {
		model := briefModel(t, 120, 40)
		selectByID(t, &model, test.id)
		got := sectionOrder(detailPlain(model), headings...)
		if strings.Join(got, ",") != strings.Join(test.want, ",") {
			t.Fatalf("%s order = %v, want %v", test.id, got, test.want)
		}
	}
}

// A faction is not in the demo fixtures, so this asserts the type mapping
// against a record built for the test.
func TestFactionDetailLeadsWithItsMembers(t *testing.T) {
	model := briefModel(t, 120, 40)
	faction := domain.Record{
		ID: "faction-greywatch", Type: domain.Faction, Title: "The Greywatch",
		Summary: "City watch of Greywatch", Authority: domain.Canon,
		Scope: model.workspace.Scope, Source: "Session 12",
		Body: "Commanded by @Captain Vale.",
	}
	model.workspace.Records = append(model.workspace.Records, faction)
	model.selectRecord(faction)
	detail := detailPlain(model)
	got := sectionOrder(detail, "WHY NOW", "CONNECTED", "LAST SEEN", "WHAT CHANGED", "DETAILS")
	want := []string{"WHY NOW", "CONNECTED", "LAST SEEN", "WHAT CHANGED", "DETAILS"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("faction order = %v, want %v", got, want)
	}
	if !strings.Contains(detail, "Captain Vale") {
		t.Fatalf("a faction should show its named members: %q", detail)
	}
}

// The detail cursor must walk the pane top to bottom, so hop order has to
// follow the type's scan order rather than a fixed reference-then-history one.
// A thread is the case that proves it: its history renders above its links.
func TestDetailCursorWalksThePaneInVisualOrder(t *testing.T) {
	model := briefModel(t, 120, 40)
	thread := domain.Record{
		ID: "thread-cursor", Type: domain.Thread, Title: "The Cracked Seal",
		Authority: domain.Canon, Scope: model.workspace.Scope, Source: "notes",
		Body: "Watched by @Captain Vale.",
	}
	model.workspace.Records = append(model.workspace.Records, thread)
	at := time.Date(2026, 2, 1, 20, 0, 0, 0, time.UTC)
	ended := at.Add(time.Hour)
	model.workspace.Sessions = append(model.workspace.Sessions, domain.SessionRecord{
		ID: "sit-cursor", Title: "Seal Sit", Scope: model.workspace.Scope,
		StartedAt: at, EndedAt: &ended, Links: []domain.EntityLink{{RecordID: thread.ID}},
	})
	model.selectRecord(thread)

	hops := model.detailHops()
	sections := map[string]bool{}
	for _, hop := range hops {
		sections[hop.Section] = true
	}
	if !sections["history"] || !sections["ref"] {
		t.Fatalf("this test needs both a reference and a history hop: %#v", hops)
	}
	if hops[0].Section != "history" {
		t.Fatalf("a thread renders history first, so j/k must start there: %#v", hops)
	}

	// Labels repeat between blocks, so this walks forward through the pane and
	// requires each hop to appear at or after the previous one.
	detail := detailPlain(model)
	cursor := 0
	for _, hop := range hops {
		at := strings.Index(detail[cursor:], hop.Label)
		if at < 0 {
			t.Fatalf("hop %q does not appear after the previous hop: %q", hop.Label, detail)
		}
		cursor += at + len(hop.Label)
	}
}

func TestEntityDetailConsolidatesReferencesAndBacklinks(t *testing.T) {
	model := briefModel(t, 120, 40)
	selectByID(t, &model, "npc-captain-vale")
	detail := detailPlain(model)
	if strings.Contains(detail, "REFERENCES") || strings.Contains(detail, "LINKED") {
		t.Fatalf("references and backlinks should share one CONNECTED block: %q", detail)
	}
	block := detail[strings.Index(detail, "CONNECTED"):]
	if end := strings.Index(block, "WHAT CHANGED"); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "outgoing @") || !strings.Contains(block, "backlink") {
		t.Fatalf("CONNECTED should hold both directions: %q", block)
	}
}

func TestEntityDetailStatesEmptySectionsExplicitly(t *testing.T) {
	model := briefModel(t, 120, 40)
	record := domain.Record{
		ID: "npc-unused", Type: domain.NPC, Title: "Unremarked",
		Authority: domain.Canon, Scope: model.workspace.Scope, Source: "notes",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	detail := detailPlain(model)
	for _, want := range []string{"no prep, sit, or review link", "not yet seen in play", "nothing references this", "no recorded changes", "No description yet"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("missing empty state %q: %q", want, detail)
		}
	}
	if strings.Contains(detail, "OPEN") {
		t.Fatalf("an entity with nothing unresolved should omit OPEN: %q", detail)
	}
}

func TestEntityDetailSurfacesBrokenReferencesAndOpenReviews(t *testing.T) {
	model := briefModel(t, 120, 40)
	record := domain.Record{
		ID: "npc-broken", Type: domain.NPC, Title: "Torn Page",
		Authority: domain.Canon, Scope: model.workspace.Scope, Source: "notes",
		Body: "Answers to @Nobody At All.",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	detail := detailPlain(model)
	if !strings.Contains(detail, "OPEN") {
		t.Fatalf("a broken reference belongs in OPEN: %q", detail)
	}
	if !strings.Contains(detail, "no entity of that name") {
		t.Fatalf("a broken reference should explain itself: %q", detail)
	}
	if !strings.Contains(detail, "missing · @") {
		t.Fatalf("the broken hop should stay navigable in CONNECTED: %q", detail)
	}
}

func TestEntityDetailStatesDraftProposalAndReferenceAuthority(t *testing.T) {
	cases := []struct {
		record domain.Record
		want   string
	}{
		{domain.Record{ID: "d", Type: domain.NPC, Title: "Draft", Authority: domain.Draft}, "DRAFT ENTITY"},
		{domain.Record{ID: "p", Type: domain.Thread, Title: "Proposed", Authority: domain.Proposal}, "AI PROPOSAL"},
		{domain.Record{ID: "r", Type: domain.Note, Title: "Imported", Authority: domain.Reference}, "IMPORTED REFERENCE"},
		{domain.Record{ID: "s", Type: domain.NPC, Title: "Old", Authority: domain.Superseded}, "SUPERSEDED"},
		{domain.Record{ID: "a", Type: domain.NPC, Title: "Guessed", Authority: domain.Draft, IsAIContent: true}, "AI-GENERATED"},
	}
	for _, test := range cases {
		model := briefModel(t, 120, 40)
		record := test.record
		record.Scope = model.workspace.Scope
		model.workspace.Records = append(model.workspace.Records, record)
		model.selectRecord(record)
		detail := detailPlain(model)
		if !strings.Contains(detail, test.want) {
			t.Fatalf("%s should announce %q: %q", record.Authority, test.want, detail)
		}
		// The notice qualifies the body, so it has to precede it.
		if strings.Index(detail, test.want) > strings.Index(detail, "WHY NOW") {
			t.Fatalf("%s notice should sit above the brief: %q", record.Authority, detail)
		}
	}
}

func TestReferenceOnlyEntityExplainsItIsUnusedAndUnowned(t *testing.T) {
	model := briefModel(t, 120, 40)
	record := domain.Record{
		ID: "reference-book-chapter", Type: domain.Note, Title: "Chapter 2",
		Authority: domain.Reference, Scope: model.workspace.Scope,
		SourceID: "src-book", Source: "Imported Book", Body: "The crypt lies east.",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	detail := detailPlain(model)
	if !strings.Contains(detail, "imported reference · unused") {
		t.Fatalf("an unused import should say so under LAST SEEN: %q", detail)
	}
	if !strings.Contains(detail, "Dungeon does not own this text") {
		t.Fatalf("ownership belongs in DETAILS: %q", detail)
	}
}

func TestEntityDetailShowsWhyAnEntityMattersNow(t *testing.T) {
	model := briefModel(t, 120, 40)
	record := selectByID(t, &model, "npc-captain-vale")
	at := time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC)
	model.workspace.PlannedNotes = append(model.workspace.PlannedNotes, domain.PlannedNotes{
		ID: "plan-crypt", Title: "Crypt descent", Scope: model.workspace.Scope,
		UpdatedAt: at, Links: []domain.EntityLink{{RecordID: record.ID}},
	})
	detail := detailPlain(model)
	if !strings.Contains(detail, "In the next prep cast · Crypt descent") {
		t.Fatalf("prep membership is the clearest 'why now': %q", detail)
	}
}

func TestEntityDetailBoundsLongContentAtBothWidths(t *testing.T) {
	for _, width := range []int{80, 120} {
		model := briefModel(t, width, 30)
		record := domain.Record{
			ID: "npc-verbose", Type: domain.NPC, Title: "The Exceedingly Long-Winded Chronicler of Greywatch",
			Authority: domain.Canon, Scope: model.workspace.Scope, Source: "A very long source citation indeed",
			Summary: strings.Repeat("A dense summary sentence that keeps going. ", 6),
			Body:    strings.Repeat("Body prose that also keeps going for quite a while. ", 20),
			Tags:    []string{"greywatch", "chronicler", "longwinded", "archive"},
		}
		model.workspace.Records = append(model.workspace.Records, record)
		model.selectRecord(record)
		view := model.View().Content
		for index, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(stripANSIForTest(line)); got > width {
				t.Fatalf("width %d line %d is %d wide: %q", width, index, got, stripANSIForTest(line))
			}
		}
		if !strings.Contains(stripANSIForTest(view), "The Exceedingly Long") {
			t.Fatalf("width %d lost the title: %q", width, stripANSIForTest(view))
		}
	}
}

func TestEntityDetailBoundsLongSightingLists(t *testing.T) {
	model := briefModel(t, 120, 40)
	record := selectByID(t, &model, "npc-captain-vale")
	at := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	for index := 0; index < 8; index++ {
		ended := at.Add(time.Duration(index)*24*time.Hour + time.Hour)
		model.workspace.Sessions = append(model.workspace.Sessions, domain.SessionRecord{
			ID:        "sit-" + string(rune('a'+index)),
			Title:     "Sit " + string(rune('A'+index)),
			Scope:     model.workspace.Scope,
			StartedAt: at.Add(time.Duration(index) * 24 * time.Hour),
			EndedAt:   &ended,
			Links:     []domain.EntityLink{{RecordID: record.ID}},
		})
	}
	detail := detailPlain(model)
	block := detail[strings.Index(detail, "LAST SEEN"):]
	if end := strings.Index(block, "CONNECTED"); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "earlier sits") {
		t.Fatalf("a long career should collapse into a count: %q", block)
	}
	if lines := strings.Count(strings.TrimSpace(block), "\n"); lines > 4 {
		t.Fatalf("LAST SEEN should stay bounded, got %d lines: %q", lines, block)
	}
}

func TestEntityDetailAnswersEveryQuestionWithoutAnotherPane(t *testing.T) {
	model := briefModel(t, 120, 40)
	selectByID(t, &model, "npc-captain-vale")
	detail := detailPlain(model)
	// What it is, why now, where it appeared, what connects, what changed.
	for _, want := range []string{"NPC", "WHY NOW", "LAST SEEN", "CONNECTED", "WHAT CHANGED"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail cannot answer without %q: %q", want, detail)
		}
	}
	if model.preview != nil {
		t.Fatal("reading detail must not open a preview")
	}
}
