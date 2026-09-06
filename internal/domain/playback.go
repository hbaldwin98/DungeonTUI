package domain

import "strings"

// PlaybackKind classifies one derived beat in a session change view.
type PlaybackKind string

const (
	PlaybackCast       PlaybackKind = "cast"
	PlaybackTranscript PlaybackKind = "transcript"
	PlaybackRecon      PlaybackKind = "recon"
)

// PlaybackFrame is one rewind/fast-forward step. It is derived from the
// immutable transcript, session Links, and reconciliation items.
type PlaybackFrame struct {
	Kind      PlaybackKind
	Label     string
	Summary   string
	RecordID  string
	EntryID   string
	EntryText string
	Status    string
}

// SessionPlayback builds a derived scrub timeline for an ended sit.
func SessionPlayback(ws Workspace, sessionID string) (SessionRecord, []PlaybackFrame, bool) {
	session, ok := findSession(ws.Sessions, sessionID)
	if !ok {
		return SessionRecord{}, nil, false
	}
	frames := make([]PlaybackFrame, 0)
	for _, link := range session.Links {
		title := link.Text
		if record, found := findRecord(ws.Records, link.RecordID); found {
			title = record.Title
		}
		frames = append(frames, PlaybackFrame{
			Kind:     PlaybackCast,
			Label:    "cast",
			Summary:  "Associated " + title,
			RecordID: link.RecordID,
		})
	}
	reconItems := reconItemsForSession(ws, session.ID)
	seenEntry := map[string]bool{}
	for _, entry := range session.Entries {
		if entry.Undone {
			continue
		}
		summary := truncate(entry.Text, 72)
		if len(entry.Rolls) > 0 {
			summary += " · " + formatPlaybackRolls(entry.Rolls)
		}
		frames = append(frames, PlaybackFrame{
			Kind:      PlaybackTranscript,
			Label:     "beat",
			Summary:   summary,
			EntryID:   entry.ID,
			EntryText: entry.Text,
			RecordID:  firstLinkID(entry.Links),
		})
		seenEntry[entry.ID] = true
		for _, item := range reconItems {
			if item.EntryID != entry.ID {
				continue
			}
			frames = append(frames, reconFrame(item))
		}
	}
	for _, item := range reconItems {
		if item.EntryID != "" && seenEntry[item.EntryID] {
			continue
		}
		frames = append(frames, reconFrame(item))
	}
	return session, frames, true
}

func reconFrame(item ReconciliationItem) PlaybackFrame {
	return PlaybackFrame{
		Kind:     PlaybackRecon,
		Label:    "recon",
		Summary:  item.Summary,
		RecordID: item.RecordID,
		EntryID:  item.EntryID,
		Status:   string(item.Status),
	}
}

func reconItemsForSession(ws Workspace, sessionID string) []ReconciliationItem {
	for _, recon := range ws.Reconciliations {
		if recon.SessionID == sessionID {
			return recon.Items
		}
	}
	return nil
}

func firstLinkID(links []EntityLink) string {
	for _, link := range links {
		if link.RecordID != "" {
			return link.RecordID
		}
	}
	return ""
}

func formatPlaybackRolls(rolls []RollResult) string {
	parts := make([]string, 0, len(rolls))
	for _, roll := range rolls {
		label := roll.Expression
		if roll.Label != "" {
			label = roll.Label + " " + roll.Expression
		}
		parts = append(parts, label+" → "+roll.Detail)
	}
	return strings.Join(parts, " · ")
}

func findSession(sessions []SessionRecord, id string) (SessionRecord, bool) {
	for _, session := range sessions {
		if session.ID == id {
			return session, true
		}
	}
	return SessionRecord{}, false
}
