package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func largeDetailMarkdown() string {
	var builder strings.Builder
	for index := 0; index < 240; index++ {
		fmt.Fprintf(&builder, "### Chapter %d\n\n", index+1)
		builder.WriteString("The lantern burns low while the party crosses the ruined causeway. ")
		builder.WriteString("A careful scout checks every arch, every loose stone, and every shadow before moving on.\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func complexCreatureMarkdown() string {
	return strings.TrimSpace(`
*Medium beast, unaligned*

**Armor Class** 14 (hide)
**Hit Points** 45 (6d8 + 18)

| STR | DEX | CON | INT | WIS | CHA |
| --- | --- | --- | --- | --- | --- |
| 14 (+2) | 12 (+1) | 16 (+3) | 8 (-1) | 11 (+0) | 9 (-1) |

**Saving Throws** Dex +3, Con +5, Wis +2
**Skills** Perception +2, Stealth +3
**Damage Resistances** mud, hail

### Legendary Actions

- **Slip.** The creature steps through a nearby shadow to an unoccupied space it can see.

> A block quote that mentions @Captain Vale and @Sister Maren of the Ashen Crown who were last seen near the ruined chapel.
`)
}

func TestDetailRendersMarkdownMarkup(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 36
	record := domain.Record{
		ID:        "creature-rascal",
		Type:      domain.Creature,
		Title:     "Cave Rascal",
		Summary:   "*Small humanoid, chaotic neutral*",
		Body:      "# Cave Rascal\n\n**AC** 12\n**HP** 9 (2d6)\n\n## Actions\n\nShortblade. Melee Weapon Attack.\n\nWary of @Captain Vale.\n",
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Test Bestiary (2025)",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)

	detail := model.renderDetailWidth(56)
	stripped := testANSI.ReplaceAllString(detail, "")
	if strings.Contains(stripped, "**AC**") || strings.Contains(stripped, "**HP**") {
		t.Fatalf("detail should render emphasis, not source markup: %q", stripped)
	}
	if !strings.Contains(stripped, "AC") || !strings.Contains(stripped, "12") {
		t.Fatalf("expected rendered AC line: %q", stripped)
	}
	if strings.Contains(stripped, "## Actions") {
		t.Fatalf("heading marks should not remain: %q", stripped)
	}
	if !strings.Contains(stripped, "Actions") {
		t.Fatalf("expected Actions heading: %q", stripped)
	}
	if strings.Contains(stripped, "*Small humanoid") {
		t.Fatalf("summary italics should render: %q", stripped)
	}
	if !strings.Contains(stripped, "@Captain Vale") {
		t.Fatalf("wiki mentions should survive rendering: %q", stripped)
	}
	if strings.Contains(stripped, "⟦m") {
		t.Fatalf("mention placeholders leaked: %q", stripped)
	}
}

func TestMarkdownTablesDoNotSwallowFollowingLines(t *testing.T) {
	model := New()
	body := "| STR | DEX |\n| --- | --- |\n| 9 (-1) | 13 (+1) |\n**Saves** Dex +3\n**Skills** Stealth +3\n"
	got := testANSI.ReplaceAllString(model.renderMarkdown(body, 48), "")
	if strings.Contains(got, "**Saves**") {
		t.Fatalf("saves should render, not stay markup: %q", got)
	}
	savesAt := strings.Index(got, "Saves")
	tableDex := strings.Index(got, "DEX")
	if savesAt < 0 || tableDex < 0 || savesAt < tableDex {
		t.Fatalf("Saves should appear after the table: %q", got)
	}
	between := got[tableDex:savesAt]
	if !strings.Contains(between, "\n") {
		t.Fatalf("expected a blank gap after the table: %q", got)
	}
}

func TestMarkdownFitsPaneWidth(t *testing.T) {
	model := New()
	const width = 36
	got := model.renderMarkdown(complexCreatureMarkdown(), width)
	stripped := testANSI.ReplaceAllString(got, "")
	if strings.Contains(stripped, "┼") || strings.Contains(stripped, "│ DEX") {
		t.Fatalf("wide ability table should flatten instead of wrapping a grid: %q", stripped)
	}
	for _, token := range []string{"STR", "DEX", "CON", "INT", "WIS", "CHA", "14 (+2)", "12 (+1)", "@Captain Vale"} {
		if !strings.Contains(stripped, token) {
			t.Fatalf("expected %q in flattened markdown: %q", token, stripped)
		}
	}
	for index, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("line %d width %d > %d: %q", index, w, width, testANSI.ReplaceAllString(line, ""))
		}
	}
}

func TestComplexEntityViewKeepsAlignedPanes(t *testing.T) {
	model := New()
	model.width = 80
	model.height = 24
	record := domain.Record{
		ID:        "creature-complex",
		Type:      domain.Creature,
		Title:     "Marsh Beast",
		Summary:   "*Medium beast, unaligned*",
		Body:      complexCreatureMarkdown(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Test Bestiary (2025)",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	h := &Harness{Model: model}
	frame := h.Frame()
	if !frame.Contains("Marsh Beast") {
		t.Fatalf("expected selected creature in view:\n%s", frame.Plain)
	}
	if !frame.Contains("CHA") || !frame.Contains("14 (+2)") {
		t.Fatalf("expected all ability scores in view:\n%s", frame.Plain)
	}
	if strings.Contains(frame.Plain, "┼") {
		t.Fatalf("wrapped table grid staggered the UI:\n%s", frame.Plain)
	}
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("layout fill failed: %s\n%s", strings.Join(errs, "; "), frame.Plain)
	}
}

func TestRenderMarkdownRetainsFinalOutputsAndCurrentMentionStyle(t *testing.T) {
	resetMarkdownCacheForTest()
	defer resetMarkdownCacheForTest()

	model := New()
	firstSource := "A first cached fragment."
	secondSource := "A second cached fragment."
	first := model.renderMarkdown(firstSource, 48)
	_ = model.renderMarkdown(secondSource, 48)

	markdownMu.Lock()
	markdownFailed = true
	markdownMu.Unlock()
	if got := model.renderMarkdown(firstSource, 48); got != first {
		t.Fatalf("expected the first fragment to survive a second cache entry")
	}

	resetMarkdownCacheForTest()
	brokenModel := New()
	mentionSource := "See @Cache Target."
	broken := brokenModel.renderMarkdown(mentionSource, 48)
	resolvedModel := New()
	resolvedModel.workspace.Records = append(resolvedModel.workspace.Records, domain.Record{
		ID:        "cache-target",
		Title:     "Cache Target",
		Authority: domain.Canon,
	})
	resolved := resolvedModel.renderMarkdown(mentionSource, 48)
	if broken == resolved {
		t.Fatalf("resolved and broken mentions should retain distinct styles")
	}
}

func TestPreviewBodyViewCachesLinesAndInvalidatesOnWorkspaceChange(t *testing.T) {
	model := New()
	record := domain.Record{
		ID:        "preview-cache-record",
		Type:      domain.Note,
		Title:     "Preview Cache",
		Body:      "CACHE_HEAD",
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.rebuildSearch()
	model.openPreview(detailHop{Kind: hopWiki, Label: record.Title, RecordID: record.ID})

	width := model.previewWidth()
	first := model.previewBodyLines(width)
	second := model.previewBodyLines(width)
	if len(first) == 0 || &first[0] != &second[0] {
		t.Fatal("preview body lines should be reused at the same width")
	}
	if !strings.Contains(testANSI.ReplaceAllString(strings.Join(first, "\n"), ""), "CACHE_HEAD") {
		t.Fatalf("initial preview body missing marker: %q", first)
	}
	model.previewFrame()
	afterFrame := model.previewBodyLines(width)
	if !strings.Contains(testANSI.ReplaceAllString(strings.Join(afterFrame, "\n"), ""), "CACHE_HEAD") {
		t.Fatalf("preview frame should not mutate cached body lines: %q", afterFrame)
	}

	model.workspace.Records[len(model.workspace.Records)-1].Body = "CACHE_UPDATED"
	model.rebuildSearch()
	updated := model.previewBodyLines(width)
	plain := testANSI.ReplaceAllString(strings.Join(updated, "\n"), "")
	if strings.Contains(plain, "CACHE_HEAD") || !strings.Contains(plain, "CACHE_UPDATED") {
		t.Fatalf("preview body cache was not invalidated: %q", plain)
	}
}

func TestPreviewFrameCachesRenderedPanelAndInvalidatesOnInputs(t *testing.T) {
	model := New()
	model.width = 100
	model.height = 36
	record := domain.Record{
		ID:        "preview-frame-cache-record",
		Type:      domain.Note,
		Title:     "Preview Frame Cache",
		Body:      strings.Repeat("FRAME_CACHE_HEAD\n", 40),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.rebuildSearch()
	model.openPreview(detailHop{Kind: hopWiki, Label: record.Title, RecordID: record.ID})

	first := model.previewFrame()
	if !model.preview.frameValid {
		t.Fatal("preview frame should be cached after rendering")
	}
	second := model.previewFrame()
	if first != second {
		t.Fatal("cached preview frame changed without input changes")
	}

	model.preview.Scroll++
	scrolled := model.previewFrame()
	if scrolled == first {
		t.Fatal("preview frame cache was not invalidated by scrolling")
	}

	model.workspace.Records[len(model.workspace.Records)-1].Body = strings.Repeat("FRAME_CACHE_UPDATED\n", 40)
	model.rebuildSearch()
	updated := model.previewFrame()
	plain := testANSI.ReplaceAllString(updated, "")
	if strings.Contains(plain, "FRAME_CACHE_HEAD") || !strings.Contains(plain, "FRAME_CACHE_UPDATED") {
		t.Fatalf("preview frame cache was not invalidated: %q", plain)
	}
}

func resetMarkdownCacheForTest() {
	markdownMu.Lock()
	defer markdownMu.Unlock()
	markdownWidth = 0
	markdownTerm = nil
	markdownFailed = false
	markdownCache = [markdownCacheCapacity]markdownCacheEntry{}
	markdownNext = 0
}

func BenchmarkRenderDetailLargeBody(b *testing.B) {
	model := New()
	model.width = 120
	model.height = 36
	record := domain.Record{
		ID:        "large-detail-benchmark",
		Type:      domain.Note,
		Title:     "Long Field Notes",
		Summary:   "*A long-running expedition log*",
		Body:      largeDetailMarkdown(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Benchmark Notes",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)

	b.ReportAllocs()
	b.SetBytes(int64(len(record.Body)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = model.renderDetailWidth(56)
	}
}

func BenchmarkRenderMarkdownLargeBody(b *testing.B) {
	model := New()
	body := largeDetailMarkdown()

	_ = model.renderMarkdown(body, 56)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = model.renderMarkdown(body, 56)
	}
}

func BenchmarkRenderMarkdownLargeBodyCold(b *testing.B) {
	model := New()
	body := largeDetailMarkdown()

	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		b.StopTimer()
		resetMarkdownCacheForTest()
		b.StartTimer()
		_ = model.renderMarkdown(body, 56)
	}
}

func BenchmarkViewLargeDetail(b *testing.B) {
	model := New()
	model.width = 120
	model.height = 36
	body := largeDetailMarkdown()
	record := domain.Record{
		ID:        "large-view-benchmark",
		Type:      domain.Note,
		Title:     "Long Field Notes",
		Summary:   "*A long-running expedition log*",
		Body:      body,
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Benchmark Notes",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)

	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = model.View()
	}
}

func BenchmarkViewAlternatingLargeDetails(b *testing.B) {
	model := New()
	model.width = 120
	model.height = 36
	firstBody := largeDetailMarkdown()
	secondBody := strings.ReplaceAll(firstBody, "lantern", "moonlight")
	first := domain.Record{
		ID:        "alternating-detail-a",
		Type:      domain.Note,
		Title:     "First Field Notes",
		Body:      firstBody,
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Benchmark Notes",
	}
	second := first
	second.ID = "alternating-detail-b"
	second.Title = "Second Field Notes"
	second.Body = secondBody
	model.workspace.Records = append(model.workspace.Records, first, second)
	model.selectRecord(first)

	b.ReportAllocs()
	b.SetBytes(int64(len(firstBody)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if index%2 == 0 {
			model.selectRecord(first)
		} else {
			model.selectRecord(second)
		}
		_ = model.View()
	}
}

func BenchmarkPreviewFrameLargeBody(b *testing.B) {
	model := New()
	model.width = 120
	model.height = 36
	record := domain.Record{
		ID:        "preview-large-body",
		Type:      domain.Note,
		Title:     "Preview Field Notes",
		Body:      largeDetailMarkdown(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Benchmark Notes",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.openPreview(detailHop{Kind: hopWiki, Label: record.Title, RecordID: record.ID})

	b.ReportAllocs()
	b.SetBytes(int64(len(record.Body)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = model.previewFrame()
	}
}

func BenchmarkPreviewFrameAndMaxScrollLargeBody(b *testing.B) {
	model := New()
	model.width = 120
	model.height = 36
	record := domain.Record{
		ID:        "preview-cache-benchmark",
		Type:      domain.Note,
		Title:     "Preview Cache Notes",
		Body:      largeDetailMarkdown(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.openPreview(detailHop{Kind: hopWiki, Label: record.Title, RecordID: record.ID})

	b.ReportAllocs()
	b.SetBytes(int64(len(record.Body)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = model.previewFrame()
		_ = model.previewMaxScroll()
	}
}
