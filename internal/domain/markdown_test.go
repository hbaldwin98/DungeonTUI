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
	}
	doc := FormatEntityMarkdown(original)
	if !strings.Contains(doc, "type: NPC") || !strings.Contains(doc, "# Captain Vale") {
		t.Fatalf("unexpected format: %q", doc)
	}
	parsed, err := ParseEntityMarkdown(doc, Note)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Type != NPC || parsed.Title != original.Title || parsed.Summary != original.Summary {
		t.Fatalf("parsed=%#v", parsed)
	}
	if strings.TrimSpace(parsed.Body) != strings.TrimSpace(original.Body) {
		t.Fatalf("body=%q want %q", parsed.Body, original.Body)
	}
	if parsed.Authority != Canon {
		t.Fatalf("authority=%q", parsed.Authority)
	}
}

func TestParseEntityMarkdownRequiresTitle(t *testing.T) {
	_, err := ParseEntityMarkdown("type: NPC\n\nno heading here\n", NPC)
	if err == nil {
		t.Fatal("expected missing title error")
	}
}
