package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// EntityBriefSection names one block of entity detail. The order of these
// blocks varies by entity type, but the set does not, so a DM learns one
// vocabulary and only has to relearn where a block sits.
type EntityBriefSection string

const (
	// BriefWhyNow answers "why does this matter now".
	BriefWhyNow EntityBriefSection = "why-now"
	// BriefLastSeen answers "where did this appear".
	BriefLastSeen EntityBriefSection = "last-seen"
	// BriefConnected answers "what connects to this".
	BriefConnected EntityBriefSection = "connected"
	// BriefChanged answers "what changed".
	BriefChanged EntityBriefSection = "changed"
	// BriefOpen answers "what is still unresolved".
	BriefOpen EntityBriefSection = "open"
	// BriefMeta carries scope, tags, collections, source, and provenance. It
	// is never first: metadata is what you check after you have decided the
	// entity is the one you wanted.
	BriefMeta EntityBriefSection = "meta"
)

// EntityBriefOrder is the scan order for one entity type. Identity always
// leads and metadata always trails; the middle order follows the question a
// DM asks first about that kind of thing.
func EntityBriefOrder(entityType EntityType) []EntityBriefSection {
	switch entityType {
	case Thread:
		// A thread is defined by its momentum, so its development and its
		// open questions outrank the cast attached to it.
		return []EntityBriefSection{BriefWhyNow, BriefChanged, BriefOpen, BriefConnected, BriefLastSeen, BriefMeta}
	case Item, Faction:
		// Who holds it or belongs to it is the first question; when it last
		// came up is the second.
		return []EntityBriefSection{BriefWhyNow, BriefConnected, BriefLastSeen, BriefChanged, BriefOpen, BriefMeta}
	case NPC, Character, Creature, Location:
		return []EntityBriefSection{BriefWhyNow, BriefLastSeen, BriefConnected, BriefChanged, BriefOpen, BriefMeta}
	default:
		// Notes, rules, scenes, and imported reference are read for their
		// text, so what changed and what they touch come before sightings.
		return []EntityBriefSection{BriefWhyNow, BriefChanged, BriefConnected, BriefLastSeen, BriefOpen, BriefMeta}
	}
}

// EntitySighting is one session in which an entity appeared.
type EntitySighting struct {
	SessionID string
	Title     string
	At        time.Time
	Live      bool
	// Notes describe how it appeared: session cast, transcript mentions,
	// review items. Empty means the session touched it in no recorded way.
	Notes []string
}

// Label renders a sighting as one scannable line.
func (s EntitySighting) Label() string {
	parts := make([]string, 0, 3)
	if !s.At.IsZero() {
		parts = append(parts, s.At.Local().Format("2006-01-02"))
	}
	if title := strings.TrimSpace(s.Title); title != "" {
		parts = append(parts, title)
	}
	if len(s.Notes) > 0 {
		parts = append(parts, strings.Join(s.Notes, ", "))
	}
	if s.Live {
		parts = append(parts, "in play now")
	}
	return strings.Join(parts, " · ")
}

// EntityChange is one recorded change to an entity, newest first.
type EntityChange struct {
	SessionID string
	Session   string
	At        time.Time
	Summary   string
	// Status is the reconciliation status when the change came from review,
	// and empty when it came from the transcript.
	Status ReconciliationStatus
}

// EntityOpenItemKind classifies why something is still unresolved.
type EntityOpenItemKind string

const (
	OpenReview    EntityOpenItemKind = "review"
	OpenReference EntityOpenItemKind = "reference"
	OpenFiling    EntityOpenItemKind = "filing"
	OpenAuthority EntityOpenItemKind = "authority"
)

// EntityOpenItem is one unresolved thing attached to an entity.
type EntityOpenItem struct {
	Kind    EntityOpenItemKind
	Summary string
	Detail  string
}

// EntityBrief answers the five questions a DM asks at the table: what is it,
// why does it matter now, where has it appeared, what connects to it, and what
// changed. It derives everything from existing records and stores nothing.
type EntityBrief struct {
	Record        Record
	WhyNow        []string
	LastSeen      *EntitySighting
	Sightings     []EntitySighting
	Changes       []EntityChange
	Open          []EntityOpenItem
	ReferenceOnly bool
}

// DeriveEntityBrief builds the brief for one record within a scope.
func DeriveEntityBrief(workspace Workspace, record Record, scope Scope) EntityBrief {
	brief := EntityBrief{Record: record, ReferenceOnly: record.Authority == Reference}
	if record.ID == "" {
		return brief
	}
	brief.Sightings = entitySightings(workspace, record.ID)
	if len(brief.Sightings) > 0 {
		latest := brief.Sightings[0]
		brief.LastSeen = &latest
	}
	brief.Changes = entityChanges(workspace, record.ID, 6)
	brief.Open = entityOpenItems(workspace, record)
	brief.WhyNow = entityWhyNow(workspace, record, scope, brief)
	return brief
}

func entitySightings(workspace Workspace, recordID string) []EntitySighting {
	rows := EntitySessionHistory(workspace, recordID)
	sessionsByID := map[string]SessionRecord{}
	for _, session := range workspace.Sessions {
		sessionsByID[session.ID] = session
	}
	sightings := make([]EntitySighting, 0, len(rows))
	for _, row := range rows {
		sighting := EntitySighting{SessionID: row.SessionID, Title: row.Title, At: row.StartedAt}
		if session, ok := sessionsByID[row.SessionID]; ok {
			sighting.Live = session.EndedAt == nil
		}
		if row.Associated {
			sighting.Notes = append(sighting.Notes, "session cast")
		}
		if row.TranscriptHits == 1 {
			sighting.Notes = append(sighting.Notes, "1 mention")
		} else if row.TranscriptHits > 1 {
			sighting.Notes = append(sighting.Notes, fmt.Sprintf("%d mentions", row.TranscriptHits))
		}
		if row.ReconItems > 0 {
			sighting.Notes = append(sighting.Notes, fmt.Sprintf("%d review", row.ReconItems))
		}
		sightings = append(sightings, sighting)
	}
	return sightings
}

func entityChanges(workspace Workspace, recordID string, limit int) []EntityChange {
	titles := map[string]string{}
	starts := map[string]time.Time{}
	for _, session := range workspace.Sessions {
		titles[session.ID] = session.Title
		starts[session.ID] = session.StartedAt
	}
	changes := make([]EntityChange, 0)
	for _, reconciliation := range workspace.Reconciliations {
		for _, item := range reconciliation.Items {
			if item.RecordID != recordID && item.Mutation.RecordID != recordID {
				continue
			}
			at := item.CreatedAt
			if at.IsZero() {
				at = starts[reconciliation.SessionID]
			}
			changes = append(changes, EntityChange{
				SessionID: reconciliation.SessionID,
				Session:   titles[reconciliation.SessionID],
				At:        at,
				Summary:   item.Summary,
				Status:    item.Status,
			})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].At.After(changes[j].At) })
	if limit > 0 && len(changes) > limit {
		changes = changes[:limit]
	}
	return changes
}

func entityOpenItems(workspace Workspace, record Record) []EntityOpenItem {
	open := make([]EntityOpenItem, 0)
	for _, reconciliation := range workspace.Reconciliations {
		for _, item := range reconciliation.Items {
			if item.RecordID != record.ID && item.Mutation.RecordID != record.ID {
				continue
			}
			if !ReconciliationUnresolved(item.Status) {
				continue
			}
			open = append(open, EntityOpenItem{
				Kind:    OpenReview,
				Summary: item.Summary,
				Detail:  "review · " + string(item.Status),
			})
		}
	}
	_, broken := EntityOutgoingRefs(record, workspace.Records)
	for _, mention := range broken {
		if mention.RecordID != "" {
			continue
		}
		open = append(open, EntityOpenItem{
			Kind:    OpenReference,
			Summary: "@" + mention.Text,
			Detail:  "no entity of that name",
		})
	}
	if IsUnfiledCapture(record) {
		open = append(open, EntityOpenItem{
			Kind:    OpenFiling,
			Summary: "Still in the capture inbox",
			Detail:  "file it to clear the inbox tag",
		})
	}
	if record.Authority == Proposal {
		open = append(open, EntityOpenItem{
			Kind:    OpenAuthority,
			Summary: "Awaiting owner approval",
			Detail:  "AI proposal · never canon until accepted",
		})
	}
	return open
}

func entityWhyNow(workspace Workspace, record Record, scope Scope, brief EntityBrief) []string {
	reasons := make([]string, 0, 4)
	sessions := campaignSessions(workspace.Sessions, scope)
	if plan := nextUnusedPlan(workspace.PlannedNotes, sessions, scope); plan != nil {
		if plan.LocationID == record.ID {
			reasons = append(reasons, "Location of the next prep · "+plan.Title)
		} else {
			for _, link := range plan.Links {
				if link.RecordID == record.ID {
					reasons = append(reasons, "In the next prep cast · "+plan.Title)
					break
				}
			}
		}
	}
	for index := len(sessions) - 1; index >= 0; index-- {
		session := sessions[index]
		if session.EndedAt != nil {
			continue
		}
		if session.LocationID == record.ID {
			reasons = append(reasons, "Where the party is right now")
		} else if session.HasLink(record.ID) {
			reasons = append(reasons, "In the cast of the sit in progress")
		}
		break
	}
	if record.Type == Thread && recordHasTag(record, "open") {
		reasons = append(reasons, "Open thread")
	}
	if reviews := countOpen(brief.Open, OpenReview); reviews > 0 {
		reasons = append(reasons, plural(reviews, "item", "items")+" waiting in review")
	}
	if latest := latestEndedSession(sessions); latest != nil && brief.LastSeen != nil && brief.LastSeen.SessionID == latest.ID {
		reasons = append(reasons, "Appeared in the most recent sit")
	}
	if IsUnfiledCapture(record) {
		reasons = append(reasons, "Unfiled capture · "+CaptureContextLabel(captureContextOf(record)))
	}
	return reasons
}

func captureContextOf(record Record) CaptureContext {
	if record.Capture == nil {
		return CaptureContext{}
	}
	return *record.Capture
}

func countOpen(items []EntityOpenItem, kind EntityOpenItemKind) int {
	count := 0
	for _, item := range items {
		if item.Kind == kind {
			count++
		}
	}
	return count
}

func plural(count int, one, many string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, one)
	}
	return fmt.Sprintf("%d %s", count, many)
}
