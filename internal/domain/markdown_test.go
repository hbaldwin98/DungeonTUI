package domain

import (
	"strings"
	"testing"
)

func TestEntityMarkdownRoundTrip(t *testing.T) {
	original := Record{
		Type:      NPC,
		Title:     "Captain Vale",
		Summary:   "Captain of the Greywatch Guard",
		Body:      "Keeps the crypt sealed.\nTrusted by the council.\n",
		Authority: Canon,
		Scope:     Scope{WorldID: "greywatch", WorldName: "Greywatch"},
		Tags:      []string{"greywatch", "guard"},
	}
	doc := FormatEntityMarkdown(original)
	if !strings.Contains(doc, "type: NPC") || !strings.Contains(doc, "# Captain Vale") {
		t.Fatalf("unexpected format: %q", doc)
	}
	if !strings.Contains(doc, "tags: greywatch, guard") {
		t.Fatalf("expected tags frontmatter, got %q", doc)
	}
	parsed, err := ParseEntityMarkdown(doc, Note)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Record.Type != NPC || parsed.Record.Title != original.Title || parsed.Record.Summary != original.Summary {
		t.Fatalf("parsed=%#v", parsed)
	}
	if strings.TrimSpace(parsed.Record.Body) != strings.TrimSpace(original.Body) {
		t.Fatalf("body=%q want %q", parsed.Record.Body, original.Body)
	}
	if parsed.Record.Authority != Canon {
		t.Fatalf("authority=%q", parsed.Record.Authority)
	}
	if parsed.ScopeLevel != WorldScope {
		t.Fatalf("scope=%q", parsed.ScopeLevel)
	}
	if len(parsed.Record.Tags) != 2 || parsed.Record.Tags[0] != "greywatch" || parsed.Record.Tags[1] != "guard" {
		t.Fatalf("tags=%v", parsed.Record.Tags)
	}
}

func TestParseEntityMarkdownRequiresTitle(t *testing.T) {
	_, err := ParseEntityMarkdown("type: NPC\n\nno heading here\n", NPC)
	if err == nil {
		t.Fatal("expected missing title error")
	}
}

func TestParseEntityMarkdownRejectsUnknownMetadata(t *testing.T) {
	tests := []string{
		"type: PERSON\n\n# Vale\n",
		"authority: maybe\n\n# Vale\n",
		"scope: library\n\n# Vale\n",
	}
	for _, doc := range tests {
		if _, err := ParseEntityMarkdown(doc, NPC); err == nil {
			t.Fatalf("expected invalid metadata error for %q", doc)
		}
	}
}
