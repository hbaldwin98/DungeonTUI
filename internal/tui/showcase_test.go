package tui

import "testing"

func TestShowcaseWorkspaceValidates(t *testing.T) {
	ws := ShowcaseWorkspace()
	if len(ws.Records) < 20 {
		t.Fatalf("expected a full wiki, got %d records", len(ws.Records))
	}
	if len(ws.Sessions) < 5 || len(ws.PlannedNotes) < 3 || len(ws.Collections) < 3 {
		t.Fatalf("sessions=%d prep=%d collections=%d", len(ws.Sessions), len(ws.PlannedNotes), len(ws.Collections))
	}
	filed := 0
	unfiled := 0
	for _, session := range ws.Sessions {
		if session.EndedAt == nil {
			t.Fatalf("showcase sits should be ended for playback, %q is live", session.Title)
		}
		if session.Folder == "" {
			unfiled++
		} else {
			filed++
		}
	}
	if filed == 0 || unfiled == 0 {
		t.Fatalf("need named folders and month buckets, filed=%d unfiled=%d", filed, unfiled)
	}
}
