package domain

import "testing"

func TestSessionPlaybackDerivesBeatsWithoutMutatingTranscript(t *testing.T) {
	ended := mustParseTime(t, "2026-01-02T18:00:00Z")
	original := "@Captain Vale opens the door"
	ws := Workspace{
		Records: []Record{{ID: "npc-vale", Title: "Captain Vale", Summary: "Guard captain"}},
		Sessions: []SessionRecord{{
			ID:        "s1",
			Title:     "Night one",
			StartedAt: mustParseTime(t, "2026-01-02T17:00:00Z"),
			EndedAt:   &ended,
			Links:     []EntityLink{{Text: "Vale", RecordID: "npc-vale"}},
			Entries: []TranscriptEntry{{
				ID:    "e1",
				Text:  original,
				Links: []EntityLink{{Text: "Captain Vale", RecordID: "npc-vale"}},
			}},
		}},
		Reconciliations: []ReconciliationRecord{{
			SessionID: "s1",
			Items: []ReconciliationItem{{
				EntryID:  "e1",
				RecordID: "npc-vale",
				Summary:  "Review @Captain Vale from transcript",
				Status:   ReconPending,
			}},
		}},
	}

	session, frames, ok := SessionPlayback(ws, "s1")
	if !ok || session.ID != "s1" {
		t.Fatal("expected session")
	}
	if len(frames) < 3 {
		t.Fatalf("expected cast+beat+recon, got %#v", frames)
	}
	if frames[0].Kind != PlaybackCast || frames[0].RecordID != "npc-vale" {
		t.Fatalf("first frame should be cast, got %#v", frames[0])
	}
	if frames[1].Kind != PlaybackTranscript || frames[1].EntryText != original {
		t.Fatalf("transcript frame=%#v", frames[1])
	}
	if frames[2].Kind != PlaybackRecon || frames[2].Status != string(ReconPending) {
		t.Fatalf("recon frame=%#v", frames[2])
	}
	if ws.Sessions[0].Entries[0].Text != original {
		t.Fatal("playback must not rewrite transcript")
	}
	if _, _, ok := SessionPlayback(ws, "missing"); ok {
		t.Fatal("missing session should not play")
	}
}
