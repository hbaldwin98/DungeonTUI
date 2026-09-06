package domain

import (
	"sort"
	"time"
)

// BacklinkKind classifies where an entity appears.
type BacklinkKind string

const (
	BacklinkSessionAssoc BacklinkKind = "session_assoc"
	BacklinkTranscript   BacklinkKind = "transcript"
	BacklinkPrep         BacklinkKind = "prep"
	BacklinkWiki         BacklinkKind = "wiki"
)

// Backlink is one place a RecordID is referenced.
type Backlink struct {
	Kind      BacklinkKind
	Title     string
	ID        string // session, prep, or entry id
	SessionID string // set for transcript hits
	Count     int    // transcript hit count within a session, when aggregated
}

// EntityBacklinks collects prep, session associations, and transcript hits for recordID.
func EntityBacklinks(ws Workspace, recordID string) []Backlink {
	if recordID == "" {
		return nil
	}
	var out []Backlink
	for _, record := range ws.Records {
		if record.ID == recordID {
			continue
		}
		resolved, _ := EntityOutgoingRefs(record, ws.Records)
		for _, mention := range resolved {
			if mention.RecordID != recordID {
				continue
			}
			out = append(out, Backlink{
				Kind:  BacklinkWiki,
				Title: record.Title,
				ID:    record.ID,
			})
			break
		}
	}
	for _, plan := range ws.PlannedNotes {
		for _, link := range plan.Links {
			if link.RecordID == recordID {
				out = append(out, Backlink{
					Kind:  BacklinkPrep,
					Title: plan.Title,
					ID:    plan.ID,
				})
				break
			}
		}
	}
	for _, session := range ws.Sessions {
		associated := session.HasLink(recordID)
		hitCount := 0
		for _, entry := range session.Entries {
			if entry.Undone {
				continue
			}
			for _, link := range entry.Links {
				if link.RecordID == recordID {
					hitCount++
					break
				}
			}
		}
		if associated {
			out = append(out, Backlink{
				Kind:  BacklinkSessionAssoc,
				Title: session.Title,
				ID:    session.ID,
			})
		}
		if hitCount > 0 {
			out = append(out, Backlink{
				Kind:      BacklinkTranscript,
				Title:     session.Title,
				ID:        session.ID,
				SessionID: session.ID,
				Count:     hitCount,
			})
		}
	}
	return out
}

// HistoryEventKind classifies a fine-grained hub event under a session row.
type HistoryEventKind string

const (
	HistoryAssoc      HistoryEventKind = "associated"
	HistoryTranscript HistoryEventKind = "transcript"
	HistoryRecon      HistoryEventKind = "reconciliation"
)

// HistoryEvent is one expandable detail under a session history row.
type HistoryEvent struct {
	Kind    HistoryEventKind
	Summary string
	EntryID string
}

// SessionHistoryRow is one session-grouped changelog entry for an entity hub.
type SessionHistoryRow struct {
	SessionID      string
	Title          string
	StartedAt      time.Time
	EndedAt        *time.Time
	Associated     bool
	TranscriptHits int
	ReconItems     int
	Events         []HistoryEvent
}

// EntitySessionHistory builds session-grouped career rows for recordID (changelog A+C).
func EntitySessionHistory(ws Workspace, recordID string) []SessionHistoryRow {
	if recordID == "" {
		return nil
	}
	reconBySession := map[string][]ReconciliationItem{}
	for _, recon := range ws.Reconciliations {
		for _, item := range recon.Items {
			if item.RecordID != recordID {
				continue
			}
			reconBySession[recon.SessionID] = append(reconBySession[recon.SessionID], item)
		}
	}

	rows := make([]SessionHistoryRow, 0)
	for _, session := range ws.Sessions {
		associated := session.HasLink(recordID)
		events := make([]HistoryEvent, 0)
		if associated {
			events = append(events, HistoryEvent{
				Kind:    HistoryAssoc,
				Summary: "Associated with session cast",
			})
		}
		hitCount := 0
		for _, entry := range session.Entries {
			if entry.Undone {
				continue
			}
			for _, link := range entry.Links {
				if link.RecordID != recordID {
					continue
				}
				hitCount++
				snippet := truncate(entry.Text, 64)
				events = append(events, HistoryEvent{
					Kind:    HistoryTranscript,
					Summary: snippet,
					EntryID: entry.ID,
				})
				break
			}
		}
		reconItems := reconBySession[session.ID]
		for _, item := range reconItems {
			events = append(events, HistoryEvent{
				Kind:    HistoryRecon,
				Summary: item.Summary + " [" + string(item.Status) + "]",
				EntryID: item.EntryID,
			})
		}
		if !associated && hitCount == 0 && len(reconItems) == 0 {
			continue
		}
		rows = append(rows, SessionHistoryRow{
			SessionID:      session.ID,
			Title:          session.Title,
			StartedAt:      session.StartedAt,
			EndedAt:        session.EndedAt,
			Associated:     associated,
			TranscriptHits: hitCount,
			ReconItems:     len(reconItems),
			Events:         events,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].StartedAt.After(rows[j].StartedAt)
	})
	return rows
}

// CountSessionsTouching returns how many sessions associate or link recordID.
func CountSessionsTouching(ws Workspace, recordID string) int {
	return len(EntitySessionHistory(ws, recordID))
}
