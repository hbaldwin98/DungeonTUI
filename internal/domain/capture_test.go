package domain

import (
	"strings"
	"testing"
	"time"
)

func captureScope() Scope {
	return Scope{WorldID: "w1", WorldName: "Aerth", CampaignID: "c1", Campaign: "Ashfall"}
}

func TestNewCaptureNoteStaysAnUnfiledDraft(t *testing.T) {
	at := time.Date(2026, 3, 4, 19, 30, 0, 0, time.UTC)
	note, err := NewCaptureNote("  The innkeeper lied about the ledger\nsecond line  ",
		CaptureContext{Origin: "live play", SessionID: "s1", SessionTitle: "Sit 4", LocationName: "Hollowmere", EntityTitle: "Marta", At: at},
		captureScope())
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if note.Type != Note {
		t.Fatalf("type = %q", note.Type)
	}
	if note.Authority != Draft {
		t.Fatalf("capture must never be canon, got %q", note.Authority)
	}
	if !IsCaptureNote(note) || !IsUnfiledCapture(note) {
		t.Fatalf("capture should be an unfiled capture: %#v", note)
	}
	if note.Title != "The innkeeper lied about the ledger" {
		t.Fatalf("title = %q", note.Title)
	}
	if !strings.Contains(note.Body, "second line") {
		t.Fatalf("body lost content: %q", note.Body)
	}
	if note.Scope != captureScope() {
		t.Fatalf("scope = %#v", note.Scope)
	}
	if note.Capture == nil || note.Capture.SessionID != "s1" || note.Capture.LocationName != "Hollowmere" || note.Capture.EntityTitle != "Marta" {
		t.Fatalf("provenance lost: %#v", note.Capture)
	}
	if err := note.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestNewCaptureNoteWorksWithoutOptionalContext(t *testing.T) {
	note, err := NewCaptureNote("a thought", CaptureContext{}, Scope{WorldID: "w1", WorldName: "Aerth"})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if note.Capture == nil || note.Capture.At.IsZero() {
		t.Fatal("capture should stamp a time even with no other context")
	}
	if got := CaptureContextLabel(*note.Capture); got != "Captured without context" {
		t.Fatalf("label = %q", got)
	}
	if err := note.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestNewCaptureNoteRejectsEmptyText(t *testing.T) {
	if _, err := NewCaptureNote("   \n\t ", CaptureContext{}, captureScope()); err == nil {
		t.Fatal("expected an error for blank capture text")
	}
}

func TestFileCaptureNoteClearsInboxWithoutPromoting(t *testing.T) {
	note, _ := NewCaptureNote("ledger thread", CaptureContext{}, captureScope())
	note.Tags = append(note.Tags, "open")

	filed, err := FileCaptureNote(note)
	if err != nil {
		t.Fatalf("file: %v", err)
	}
	if IsUnfiledCapture(filed) {
		t.Fatal("filed capture should leave the inbox")
	}
	if !IsCaptureNote(filed) {
		t.Fatal("filing should keep capture provenance")
	}
	if filed.Authority != Draft {
		t.Fatalf("filing must not promote, authority = %q", filed.Authority)
	}
	if len(filed.Tags) != 1 || filed.Tags[0] != "open" {
		t.Fatalf("tags = %#v", filed.Tags)
	}
	if _, err := FileCaptureNote(filed); err == nil {
		t.Fatal("filing twice should report the note is not unfiled")
	}
}

func TestUnfiledCapturesScopedAndNewestFirst(t *testing.T) {
	older, _ := NewCaptureNote("older", CaptureContext{At: time.Unix(100, 0)}, captureScope())
	newer, _ := NewCaptureNote("newer", CaptureContext{At: time.Unix(200, 0)}, captureScope())
	filed, _ := NewCaptureNote("filed", CaptureContext{At: time.Unix(300, 0)}, captureScope())
	filed, _ = FileCaptureNote(filed)
	elsewhere, _ := NewCaptureNote("other campaign", CaptureContext{At: time.Unix(400, 0)},
		Scope{WorldID: "w1", WorldName: "Aerth", CampaignID: "c2", Campaign: "Other"})
	plain := Record{ID: "n1", Type: Note, Title: "Plain note", Authority: Canon, Scope: captureScope()}

	ws := Workspace{Records: []Record{older, newer, filed, elsewhere, plain}}
	got := UnfiledCaptures(ws, captureScope())
	if len(got) != 2 {
		t.Fatalf("want 2 unfiled captures, got %d", len(got))
	}
	if got[0].Title != "newer" || got[1].Title != "older" {
		t.Fatalf("order = %q, %q", got[0].Title, got[1].Title)
	}
}

func TestSessionCapturesOnlyMatchThatSession(t *testing.T) {
	mine, _ := NewCaptureNote("mine", CaptureContext{SessionID: "s1", At: time.Unix(100, 0)}, captureScope())
	later, _ := NewCaptureNote("later", CaptureContext{SessionID: "s1", At: time.Unix(200, 0)}, captureScope())
	other, _ := NewCaptureNote("other", CaptureContext{SessionID: "s2"}, captureScope())
	loose, _ := NewCaptureNote("loose", CaptureContext{}, captureScope())

	ws := Workspace{Records: []Record{mine, later, other, loose}}
	got := SessionCaptures(ws, "s1")
	if len(got) != 2 || got[0].Title != "later" {
		t.Fatalf("session captures = %#v", got)
	}
	if len(SessionCaptures(ws, "")) != 0 {
		t.Fatal("an empty session id should match nothing")
	}
}

func TestCampaignHomeSurfacesUnfiledCaptures(t *testing.T) {
	note, _ := NewCaptureNote("innkeeper lied", CaptureContext{}, captureScope())
	ws := Workspace{Records: []Record{note}}

	home := DeriveCampaignHome(ws, captureScope())
	if len(home.UnfiledCaptures) != 1 || home.UnfiledCaptures[0].Title != "innkeeper lied" {
		t.Fatalf("home captures = %#v", home.UnfiledCaptures)
	}
}

func TestCaptureProvenanceLinesStateTheAuthority(t *testing.T) {
	note, _ := NewCaptureNote("thing", CaptureContext{SessionTitle: "Sit 4", At: time.Unix(0, 0)}, captureScope())
	lines := CaptureProvenanceLines(note)
	if len(lines) != 3 || !strings.Contains(lines[0], "not canon") {
		t.Fatalf("lines = %#v", lines)
	}
	if CaptureProvenanceLines(Record{Type: NPC}) != nil {
		t.Fatal("non-capture records have no capture provenance")
	}
}

func TestCaptureMarkdownRoundTripKeepsInboxTag(t *testing.T) {
	note, _ := NewCaptureNote("keep the tag", CaptureContext{}, captureScope())
	parsed, err := ParseEntityMarkdown(FormatEntityMarkdown(note), Note)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !recordHasTag(parsed.Record, CaptureNoteTag) {
		t.Fatalf("markdown round trip dropped the inbox tag: %#v", parsed.Record.Tags)
	}
}
