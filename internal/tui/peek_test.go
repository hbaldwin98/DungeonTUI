package tui

import "testing"

func TestReferenceAtCursor(t *testing.T) {
	text, ok := referenceAtCursor("Meet @Captain Vale later", 8)
	if !ok {
		t.Fatal("expected token under cursor")
	}
	if text != "Captain" {
		t.Fatalf("got %q want Captain", text)
	}
	_, ok = referenceAtCursor("no refs here", 3)
	if ok {
		t.Fatal("expected no token")
	}
}

func TestResolveReferenceAtCursorMultiWord(t *testing.T) {
	model := New()
	line := "Meet @Captain Vale later"
	// Cursor on 'V' of Vale
	col := stringsIndex(line, "Vale")
	record := model.resolveReferenceAtCursor(line, col)
	if record == nil || record.Title != "Captain Vale" {
		t.Fatalf("expected Captain Vale peek, got %#v", record)
	}
	if model.resolveReferenceAtCursor("plain text", 2) != nil {
		t.Fatal("expected no peek")
	}
}

func stringsIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
