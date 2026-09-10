package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CaptureNoteTag marks a quick-capture note the owner has not filed yet.
// Removing the tag files the note; the note itself stays an ordinary Note
// record so the existing editor, search, and delete paths keep working.
const CaptureNoteTag = "inbox"

// CaptureSource identifies quick-capture provenance on a Note record.
const CaptureSource = "quick capture"

// CaptureContext is the optional provenance recorded at capture time. Every
// field beyond the timestamp may be absent; capture never blocks on context.
type CaptureContext struct {
	Origin       string // UI surface the capture came from, such as "live play"
	SessionID    string
	SessionTitle string
	LocationID   string
	LocationName string
	EntityID     string
	EntityTitle  string
	At           time.Time
}

// NewCaptureNote builds an unfiled draft note from free text. It never
// produces canon: the note is a draft tagged for the capture inbox until the
// owner files, classifies, or discards it.
func NewCaptureNote(text string, context CaptureContext, scope Scope) (Record, error) {
	body := strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), " \t\n")
	if strings.TrimSpace(body) == "" {
		return Record{}, fmt.Errorf("capture text is required")
	}
	if context.At.IsZero() {
		context.At = time.Now().UTC()
	} else {
		context.At = context.At.UTC()
	}
	return Record{
		ID:        fmt.Sprintf("capture-%d", context.At.UnixNano()),
		Type:      Note,
		Title:     captureTitle(body),
		Summary:   CaptureContextLabel(context),
		Body:      body,
		Authority: Draft,
		Scope:     scope,
		Source:    CaptureSource,
		Tags:      []string{CaptureNoteTag},
		Capture:   &context,
	}, nil
}

func captureTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(strings.TrimLeft(line, "#> -*"))
		if trimmed != "" {
			return truncate(trimmed, 48)
		}
	}
	return "Captured note"
}

// IsCaptureNote reports whether a record came from quick capture.
func IsCaptureNote(record Record) bool {
	return record.Type == Note && (record.Capture != nil || record.Source == CaptureSource)
}

// IsUnfiledCapture reports whether a capture still waits for the owner to
// file, classify, or discard it.
func IsUnfiledCapture(record Record) bool {
	return IsCaptureNote(record) && recordHasTag(record, CaptureNoteTag)
}

// FileCaptureNote clears the inbox tag so the note leaves the capture queue.
// Authority is untouched: filing is not promotion to canon.
func FileCaptureNote(record Record) (Record, error) {
	if !IsUnfiledCapture(record) {
		return record, fmt.Errorf("record %q is not an unfiled capture", record.ID)
	}
	tags := make([]string, 0, len(record.Tags))
	for _, tag := range record.Tags {
		if !strings.EqualFold(tag, CaptureNoteTag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		tags = nil
	}
	record.Tags = tags
	return record, nil
}

// UnfiledCaptures returns the scope's unfiled captures, newest first.
func UnfiledCaptures(workspace Workspace, scope Scope) []Record {
	enabled := workspace.EnabledSourceIDs(scope)
	var out []Record
	for _, record := range workspace.Records {
		if IsUnfiledCapture(record) && RecordVisibleIn(record, scope, enabled) {
			out = append(out, record)
		}
	}
	sortCapturesNewestFirst(out)
	return out
}

// SessionCaptures returns unfiled captures taken during one session, newest first.
func SessionCaptures(workspace Workspace, sessionID string) []Record {
	if sessionID == "" {
		return nil
	}
	var out []Record
	for _, record := range workspace.Records {
		if IsUnfiledCapture(record) && record.Capture != nil && record.Capture.SessionID == sessionID {
			out = append(out, record)
		}
	}
	sortCapturesNewestFirst(out)
	return out
}

func sortCapturesNewestFirst(records []Record) {
	sort.SliceStable(records, func(i, j int) bool {
		return captureTime(records[i]).After(captureTime(records[j]))
	})
}

func captureTime(record Record) time.Time {
	if record.Capture != nil {
		return record.Capture.At
	}
	return time.Time{}
}

// CaptureContextLabel renders the provenance a capture actually has as one
// compact line. Absent context contributes nothing rather than a placeholder.
func CaptureContextLabel(context CaptureContext) string {
	parts := make([]string, 0, 4)
	if context.SessionTitle != "" {
		parts = append(parts, context.SessionTitle)
	}
	if context.LocationName != "" {
		parts = append(parts, context.LocationName)
	}
	if context.EntityTitle != "" {
		parts = append(parts, "@"+context.EntityTitle)
	}
	if context.Origin != "" {
		parts = append(parts, context.Origin)
	}
	if len(parts) == 0 {
		return "Captured without context"
	}
	return strings.Join(parts, " · ")
}

// CaptureProvenanceLines describes one capture for a review surface.
func CaptureProvenanceLines(record Record) []string {
	if !IsCaptureNote(record) {
		return nil
	}
	lines := []string{"Unclassified capture · draft · not canon"}
	if record.Capture == nil {
		return append(lines, "Captured without context")
	}
	lines = append(lines, CaptureContextLabel(*record.Capture))
	if !record.Capture.At.IsZero() {
		lines = append(lines, record.Capture.At.Format("2006-01-02 15:04"))
	}
	return lines
}
