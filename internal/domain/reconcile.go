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

// ReconciliationKind classifies what the DM is reviewing.
type ReconciliationKind string

const (
	ReconLinkReview     ReconciliationKind = "link_review"
	ReconPromoteDraft   ReconciliationKind = "promote_draft"
	ReconTranscriptNote ReconciliationKind = "transcript_note"
)

// ReconciliationItem is a reviewable change derived from a session.
// It references transcript entries by ID and never rewrites them.
type ReconciliationItem struct {
	ID        string
	EntryID   string
	RecordID  string
	Kind      ReconciliationKind
	Summary   string
	Status    ReconciliationStatus
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
			out.Items = append(out.Items, ReconciliationItem{
				ID:        fmt.Sprintf("%s-link-%s-%s", out.ID, entry.ID, link.RecordID),
				EntryID:   entry.ID,
				RecordID:  link.RecordID,
				Kind:      ReconLinkReview,
				Summary:   fmt.Sprintf("Review @%s from transcript", link.Text),
				Status:    ReconPending,
				CreatedAt: created,
			})
			if record, ok := findRecord(records, link.RecordID); ok && record.Authority == Draft {
				seenDraft[record.ID] = true
				out.Items = append(out.Items, ReconciliationItem{
					ID:        fmt.Sprintf("%s-draft-%s", out.ID, record.ID),
					EntryID:   entry.ID,
					RecordID:  record.ID,
					Kind:      ReconPromoteDraft,
					Summary:   "Promote draft: " + record.Title,
					Status:    ReconPending,
					CreatedAt: created,
				})
			}
		}
		if len(entry.Links) == 0 && strings.TrimSpace(entry.Text) != "" {
			out.Items = append(out.Items, ReconciliationItem{
				ID:        fmt.Sprintf("%s-note-%s", out.ID, entry.ID),
				EntryID:   entry.ID,
				Kind:      ReconTranscriptNote,
				Summary:   truncate(entry.Text, 72),
				Status:    ReconPending,
				CreatedAt: created,
			})
		}
	}
	for _, record := range records {
		if record.Authority != Draft || record.Source != session.Title || seenDraft[record.ID] {
			continue
		}
		out.Items = append(out.Items, ReconciliationItem{
			ID:        fmt.Sprintf("%s-draft-%s", out.ID, record.ID),
			RecordID:  record.ID,
			Kind:      ReconPromoteDraft,
			Summary:   "Promote draft: " + record.Title,
			Status:    ReconPending,
			CreatedAt: created,
		})
	}
	return out
}

// ApproveItem marks an item approved and, for draft promotion, returns the
// updated record list. Transcript entries are never modified.
func ApproveItem(item ReconciliationItem, records []Record) (ReconciliationItem, []Record, error) {
	item.Status = ReconApproved
	if item.Kind != ReconPromoteDraft || item.RecordID == "" {
		return item, records, nil
	}
	out := append([]Record(nil), records...)
	for index := range out {
		if out[index].ID != item.RecordID {
			continue
		}
		if out[index].Authority != Draft {
			return item, records, fmt.Errorf("record %q is not a draft", item.RecordID)
		}
		out[index].Authority = Canon
		return item, out, nil
	}
	return item, records, fmt.Errorf("record %q not found", item.RecordID)
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
