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

func TestResolveReferenceAtCursorTypingAndBoundaries(t *testing.T) {
	model := New()
	tests := []struct {
		name string
		line string
		col  int
		want string
	}{
		{name: "incomplete reference", line: "Meet @Capt", col: len("Meet @Capt"), want: "Captain Vale"},
		{name: "punctuated reference", line: "Meet @Captain Vale, now", col: len("Meet @Captain"), want: "Captain Vale"},
		{name: "non-boundary suffix", line: "Meet @Captain Valex", col: len("Meet @Captain"), want: ""},
		{name: "cursor after reference", line: "Meet @Captain Vale later", col: len("Meet @Captain Vale later"), want: ""},
		{name: "empty reference", line: "Meet @", col: len("Meet @"), want: ""},
		{name: "whitespace reference", line: "Meet @ ", col: len("Meet @ "), want: ""},
		{name: "negative cursor", line: "Meet @Captain Vale", col: -1, want: ""},
		{name: "cursor past line", line: "Meet @Captain Vale", col: 100, want: "Captain Vale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := model.resolveReferenceAtCursor(tt.line, tt.col)
			if tt.want == "" {
				if record != nil {
					t.Fatalf("expected no reference, got %#v", record)
				}
				return
			}
			if record == nil || record.Title != tt.want {
				t.Fatalf("got %#v, want %q", record, tt.want)
			}
		})
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
