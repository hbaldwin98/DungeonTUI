package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

func complexCreatureMarkdown() string {
	return strings.TrimSpace(`
*Medium fiend, chaotic evil*

**Armor Class** 19 (natural armor)
**Hit Points** 187 (15d10 + 105)

| STR | DEX | CON | INT | WIS | CHA |
| --- | --- | --- | --- | --- | --- |
| 20 (+5) | 15 (+2) | 21 (+5) | 19 (+4) | 17 (+3) | 22 (+6) |

**Saving Throws** Dex +9, Con +12, Wis +10, Cha +13
**Skills** Deception +13, Insight +10, Perception +10
**Damage Resistances** cold, fire, lightning; bludgeoning, piercing, and slashing from nonmagical attacks

### Legendary Actions

- **Teleport.** The creature magically teleports, along with any equipment it is wearing or carrying, up to 120 feet to an unoccupied space it can see.

> A block quote that mentions @Captain Vale and @Sister Maren of the Ashen Crown who were last seen near the ruined chapel.
`)
}

func TestDetailRendersMarkdownMarkup(t *testing.T) {
	model := New()
	model.width = 120
	model.height = 36
	record := domain.Record{
		ID:        "creature-goblin",
		Type:      domain.Creature,
		Title:     "Goblin Warrior",
		Summary:   "*Small humanoid, neutral evil*",
		Body:      "# Goblin Warrior\n\n**AC** 15\n**HP** 10 (3d6)\n\n## Actions\n\nScimitar. Melee Weapon Attack.\n\nWary of @Captain Vale.\n",
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Monster Manual (2025)",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)

	detail := model.renderDetailWidth(56)
	stripped := testANSI.ReplaceAllString(detail, "")
	if strings.Contains(stripped, "**AC**") || strings.Contains(stripped, "**HP**") {
		t.Fatalf("detail should render emphasis, not source markup: %q", stripped)
	}
	if !strings.Contains(stripped, "AC") || !strings.Contains(stripped, "15") {
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
	body := "| STR | DEX |\n| --- | --- |\n| 8 (-1) | 15 (+2) |\n**Saves** Dex +4\n**Skills** Stealth +6\n"
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
	for _, token := range []string{"STR", "DEX", "CON", "INT", "WIS", "CHA", "20 (+5)", "22 (+6)", "@Captain Vale"} {
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
		Title:     "Complex Fiend",
		Summary:   "*Medium fiend, chaotic evil*",
		Body:      complexCreatureMarkdown(),
		Authority: domain.Canon,
		Scope:     model.workspace.Scope,
		Source:    "Monster Manual (2025)",
	}
	model.workspace.Records = append(model.workspace.Records, record)
	model.selectRecord(record)
	h := &Harness{Model: model}
	frame := h.Frame()
	if !frame.Contains("Complex Fiend") {
		t.Fatalf("expected selected creature in view:\n%s", frame.Plain)
	}
	if !frame.Contains("CHA") || !frame.Contains("22 (+6)") {
		t.Fatalf("expected all ability scores in view:\n%s", frame.Plain)
	}
	if strings.Contains(frame.Plain, "┼") {
		t.Fatalf("wrapped table grid staggered the UI:\n%s", frame.Plain)
	}
	if errs := frame.FillErrors(); len(errs) > 0 {
		t.Fatalf("layout fill failed: %s\n%s", strings.Join(errs, "; "), frame.Plain)
	}
}
