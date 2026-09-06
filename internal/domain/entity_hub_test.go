package domain

import (
	"testing"
	"time"
)

func TestEntityBacklinksAndHistory(t *testing.T) {
	ended := mustParseTime(t, "2026-01-02T18:00:00Z")
	ws := Workspace{
		PlannedNotes: []PlannedNotes{{
			ID:    "plan-1",
			Title: "Crypt approach",
			Links: []EntityLink{{Text: "Vale", RecordID: "npc-vale"}},
		}},
		Sessions: []SessionRecord{{
			ID:        "s1",
			Title:     "Night one",
			StartedAt: mustParseTime(t, "2026-01-02T17:00:00Z"),
			EndedAt:   &ended,
			Links:     []EntityLink{{Text: "Vale", RecordID: "npc-vale"}},
			Entries: []TranscriptEntry{{
				ID:   "e1",
				Text: "@Captain Vale opens the door",
				Links: []EntityLink{{Text: "Captain Vale", RecordID: "npc-vale"}},
			}},
		}},
		Reconciliations: []ReconciliationRecord{{
			SessionID: "s1",
			Items: []ReconciliationItem{{
				RecordID: "npc-vale",
				Summary:  "Review @Captain Vale from transcript",
				Status:   ReconPending,
			}},
		}},
	}

	links := EntityBacklinks(ws, "npc-vale")
	if len(links) < 3 {
		t.Fatalf("expected prep+assoc+transcript backlinks, got %#v", links)
	}

	history := EntitySessionHistory(ws, "npc-vale")
	if len(history) != 1 {
		t.Fatalf("expected 1 history row, got %#v", history)
	}
	row := history[0]
	if !row.Associated || row.TranscriptHits != 1 || row.ReconItems != 1 {
		t.Fatalf("unexpected row %#v", row)
	}
	if len(row.Events) < 3 {
		t.Fatalf("expected expandable events, got %#v", row.Events)
	}
	if CountSessionsTouching(ws, "npc-vale") != 1 {
		t.Fatal("expected one touching session")
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
