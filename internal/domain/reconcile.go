package domain

import (
	"fmt"
	"strings"
	"time"
)

// ReconciliationStatus is the review state of one extracted change.
type ReconciliationStatus string

const (
	ReconPending  ReconciliationStatus = "pending"
	ReconApproved ReconciliationStatus = "approved"
	ReconRejected ReconciliationStatus = "rejected"
	ReconDeferred ReconciliationStatus = "deferred"
)

// ReconciliationUnresolved reports whether an item still requires an owner decision.
func ReconciliationUnresolved(status ReconciliationStatus) bool {
	return status == ReconPending || status == ReconDeferred
}

// ReconciliationKind classifies what the DM is reviewing.
type ReconciliationKind string

const (
	ReconLinkReview     ReconciliationKind = "link_review"
	ReconPromoteDraft   ReconciliationKind = "promote_draft"
	ReconTranscriptNote ReconciliationKind = "transcript_note"
)

// MutationOp is the factual wiki write an approved recon item will apply.
type MutationOp string

const (
	MutationCite    MutationOp = "cite"
	MutationPromote MutationOp = "promote"
	MutationNote    MutationOp = "note"
)

// Mutation is an editable, source-linked wiki change. The transcript is never
// rewritten; approving applies this mutation to Records instead.
type Mutation struct {
	Op       MutationOp
	RecordID string
	Text     string
}

// ReconciliationItem is a reviewable change derived from a session.
// It references transcript entries by ID and never rewrites them.
type ReconciliationItem struct {
	ID        string
	EntryID   string
	RecordID  string
	Kind      ReconciliationKind
	Summary   string
	Status    ReconciliationStatus
	Mutation  Mutation
	CreatedAt time.Time
}

// ReconciliationRecord groups post-session review items for one ended session.
type ReconciliationRecord struct {
	ID        string
	SessionID string
	Title     string
	CreatedAt time.Time
	Items     []ReconciliationItem
}

// BuildSessionReconciliation extracts review items from an ended session
// without mutating transcript entries.
func BuildSessionReconciliation(session SessionRecord, records []Record) ReconciliationRecord {
	created := time.Now().UTC()
	if session.EndedAt != nil {
		created = session.EndedAt.UTC()
	}
	out := ReconciliationRecord{
		ID:        fmt.Sprintf("recon-%s", session.ID),
		SessionID: session.ID,
		Title:     "Reconcile " + session.Title,
		CreatedAt: created,
	}
	seenDraft := map[string]bool{}
	for _, entry := range session.Entries {
		if entry.Undone {
			continue
		}
		for _, link := range entry.Links {
			out.Items = append(out.Items, linkReviewItem(out.ID, session, entry, link, created))
			if record, ok := findRecord(records, link.RecordID); ok && record.Authority == Draft {
				seenDraft[record.ID] = true
				out.Items = append(out.Items, promoteDraftItem(out.ID, session, entry.ID, record, created))
			}
		}
		if len(entry.Links) == 0 && strings.TrimSpace(entry.Text) != "" {
			out.Items = append(out.Items, transcriptNoteItem(out.ID, session, entry, created))
		}
	}
	for _, record := range records {
		if record.Authority != Draft || record.Source != session.Title || seenDraft[record.ID] {
			continue
		}
		out.Items = append(out.Items, promoteDraftItem(out.ID, session, "", record, created))
	}
	return out
}

func linkReviewItem(reconID string, session SessionRecord, entry TranscriptEntry, link EntityLink, created time.Time) ReconciliationItem {
	text := strings.TrimSpace(entry.Text)
	if text == "" {
		text = "@" + link.Text + " appeared in play."
	}
	return ReconciliationItem{
		ID:        fmt.Sprintf("%s-link-%s-%s", reconID, entry.ID, link.RecordID),
		EntryID:   entry.ID,
		RecordID:  link.RecordID,
		Kind:      ReconLinkReview,
		Summary:   fmt.Sprintf("Cite @%s from transcript", link.Text),
		Status:    ReconPending,
		Mutation:  Mutation{Op: MutationCite, RecordID: link.RecordID, Text: text},
		CreatedAt: created,
	}
}

func promoteDraftItem(reconID string, session SessionRecord, entryID string, record Record, created time.Time) ReconciliationItem {
	return ReconciliationItem{
		ID:       fmt.Sprintf("%s-draft-%s", reconID, record.ID),
		EntryID:  entryID,
		RecordID: record.ID,
		Kind:     ReconPromoteDraft,
		Summary:  "Promote draft: " + record.Title,
		Status:   ReconPending,
		Mutation: Mutation{
			Op:       MutationPromote,
			RecordID: record.ID,
			Text:     "Promoted to campaign canon from " + session.Title,
		},
		CreatedAt: created,
	}
}

func transcriptNoteItem(reconID string, session SessionRecord, entry TranscriptEntry, created time.Time) ReconciliationItem {
	text := strings.TrimSpace(entry.Text)
	return ReconciliationItem{
		ID:        fmt.Sprintf("%s-note-%s", reconID, entry.ID),
		EntryID:   entry.ID,
		Kind:      ReconTranscriptNote,
		Summary:   truncate(text, 72),
		Status:    ReconPending,
		Mutation:  Mutation{Op: MutationNote, Text: text},
		CreatedAt: created,
	}
}

// ApproveItem marks an item approved and applies its mutation to wiki records.
// Transcript entries are never modified. Prefer ApplyReconItem when the session
// is available so citations can include the sit title.
func ApproveItem(item ReconciliationItem, records []Record) (ReconciliationItem, []Record, error) {
	return ApplyReconItem(item, records, SessionRecord{})
}

// ApplyReconItem applies an editable sourced mutation to wiki records and marks
// the item approved. It refuses non-pending items and never rewrites transcripts.
func ApplyReconItem(item ReconciliationItem, records []Record, session SessionRecord) (ReconciliationItem, []Record, error) {
	if item.Status == ReconApproved {
		return item, records, nil
	}
	if !ReconciliationUnresolved(item.Status) {
		return item, records, fmt.Errorf("item %q is %s", item.ID, item.Status)
	}
	if item.Mutation.Op == "" && item.Kind == "" {
		item.Status = ReconApproved
		return item, records, nil
	}
	op := item.Mutation.Op
	if op == "" {
		op = mutationOpFor(item.Kind)
		item.Mutation.Op = op
		if item.Mutation.RecordID == "" {
			item.Mutation.RecordID = item.RecordID
		}
	}
	out := append([]Record(nil), records...)
	var err error
	switch op {
	case MutationCite:
		out, err = applyCite(out, item, session)
	case MutationPromote:
		out, err = applyPromote(out, item, session)
	case MutationNote:
		out, err = applyNote(out, item, session)
	default:
		err = fmt.Errorf("unknown mutation %q", op)
	}
	if err != nil {
		return item, records, err
	}
	item.Status = ReconApproved
	return item, out, nil
}

func mutationOpFor(kind ReconciliationKind) MutationOp {
	switch kind {
	case ReconPromoteDraft:
		return MutationPromote
	case ReconTranscriptNote:
		return MutationNote
	default:
		return MutationCite
	}
}

func applyCite(records []Record, item ReconciliationItem, session SessionRecord) ([]Record, error) {
	id := item.Mutation.RecordID
	if id == "" {
		id = item.RecordID
	}
	index, err := recordIndex(records, id)
	if err != nil {
		return records, err
	}
	marker := sourceMarker(session.ID, item.EntryID)
	if strings.Contains(records[index].Body, marker) {
		return records, nil
	}
	citation := sourcedBlock(session, item.Mutation.Text, marker)
	records[index].Body = appendBlock(records[index].Body, citation)
	return records, nil
}

func applyPromote(records []Record, item ReconciliationItem, session SessionRecord) ([]Record, error) {
	id := item.Mutation.RecordID
	if id == "" {
		id = item.RecordID
	}
	index, err := recordIndex(records, id)
	if err != nil {
		return records, err
	}
	if records[index].Authority != Draft {
		return records, fmt.Errorf("record %q is not a draft", id)
	}
	records[index].Authority = Canon
	if strings.TrimSpace(item.Mutation.Text) != "" {
		marker := sourceMarker(session.ID, item.EntryID)
		if !strings.Contains(records[index].Body, marker) {
			records[index].Body = appendBlock(records[index].Body, sourcedBlock(session, item.Mutation.Text, marker))
		}
	}
	return records, nil
}

func applyNote(records []Record, item ReconciliationItem, session SessionRecord) ([]Record, error) {
	text := strings.TrimSpace(item.Mutation.Text)
	if text == "" {
		return records, fmt.Errorf("mutation text is required")
	}
	id := uniqueNoteID(records, item.EntryID)
	title := truncate(text, 48)
	if title == "" {
		title = "Session note"
	}
	note := Record{
		ID:        id,
		Type:      Note,
		Title:     title,
		Body:      text,
		Authority: Canon,
		Scope:     session.Scope,
		Source:    session.Title,
	}
	if note.Source == "" {
		note.Source = "session"
	}
	return append(records, note), nil
}

func recordIndex(records []Record, id string) (int, error) {
	if id == "" {
		return -1, fmt.Errorf("mutation target is required")
	}
	for index := range records {
		if records[index].ID == id {
			return index, nil
		}
	}
	return -1, fmt.Errorf("record %q not found", id)
}

func uniqueNoteID(records []Record, entryID string) string {
	base := "note-recon"
	if entryID != "" {
		base = "note-" + entryID
	}
	if _, exists := findRecord(records, base); !exists {
		return base
	}
	for n := 2; ; n++ {
		id := fmt.Sprintf("%s-%d", base, n)
		if _, exists := findRecord(records, id); !exists {
			return id
		}
	}
}

func sourceMarker(sessionID, entryID string) string {
	if sessionID == "" && entryID == "" {
		return "dungeon:session"
	}
	if entryID == "" {
		return fmt.Sprintf("dungeon:session=%s", sessionID)
	}
	return fmt.Sprintf("dungeon:session=%s entry=%s", sessionID, entryID)
}

func sourcedBlock(session SessionRecord, text, marker string) string {
	title := strings.TrimSpace(session.Title)
	if title == "" {
		title = "Session"
	}
	body := strings.TrimSpace(text)
	if body == "" {
		body = "Cited from play."
	}
	return fmt.Sprintf("From %s:\n%s\n<!-- %s -->", title, body, marker)
}

func appendBlock(body, block string) string {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return block
	}
	return body + "\n\n" + block
}

func findRecord(records []Record, id string) (Record, bool) {
	for _, record := range records {
		if record.ID == id {
			return record, true
		}
	}
	return Record{}, false
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}
