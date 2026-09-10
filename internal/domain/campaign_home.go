package domain

import (
	"sort"
	"strings"
	"time"
)

// CampaignHomeSummary is a derived working set for one campaign.
type CampaignHomeSummary struct {
	NextPlan          *PlannedNotes
	LatestSession     *SessionRecord
	CurrentLocation   string
	Cast              []Record
	OpenThreads       []Record
	UnresolvedReviews int
	NextReviewID      string
	RecentChanges     []CampaignHomeChange
}

// CampaignHomeChange identifies a recent approved wiki mutation.
type CampaignHomeChange struct {
	RecordID string
	Title    string
	Summary  string
	At       time.Time
}

// DeriveCampaignHome builds a campaign summary without storing duplicate state.
func DeriveCampaignHome(workspace Workspace, scope Scope) CampaignHomeSummary {
	summary := CampaignHomeSummary{}
	sessions := campaignSessions(workspace.Sessions, scope)
	summary.NextPlan = nextUnusedPlan(workspace.PlannedNotes, sessions, scope)
	summary.LatestSession = latestEndedSession(sessions)
	summary.CurrentLocation = campaignHomeLocation(sessions, summary.LatestSession)
	summary.OpenThreads = openCampaignThreads(workspace, scope)
	summary.UnresolvedReviews, summary.NextReviewID = unresolvedCampaignReviews(workspace.Reconciliations, sessions)
	summary.RecentChanges = recentCampaignChanges(workspace, sessions, 4)
	summary.Cast = campaignHomeCast(workspace, scope, summary.NextPlan, sessions, summary.LatestSession)
	return summary
}

func campaignSessions(sessions []SessionRecord, scope Scope) []SessionRecord {
	out := make([]SessionRecord, 0)
	for _, session := range sessions {
		if session.Scope.WorldID == scope.WorldID && session.Scope.CampaignID == scope.CampaignID {
			out = append(out, session)
		}
	}
	return out
}

func nextUnusedPlan(plans []PlannedNotes, sessions []SessionRecord, scope Scope) *PlannedNotes {
	used := map[string]bool{}
	for _, session := range sessions {
		used[session.PlannedNotesID] = session.PlannedNotesID != ""
	}
	var candidates []PlannedNotes
	for _, plan := range plans {
		if plan.Scope.WorldID == scope.WorldID && plan.Scope.CampaignID == scope.CampaignID && !used[plan.ID] {
			candidates = append(candidates, plan)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return planActivity(candidates[i]).After(planActivity(candidates[j]))
	})
	if len(candidates) == 0 {
		return nil
	}
	return &candidates[0]
}

func planActivity(plan PlannedNotes) time.Time {
	if !plan.UpdatedAt.IsZero() {
		return plan.UpdatedAt
	}
	return plan.CreatedAt
}

func latestEndedSession(sessions []SessionRecord) *SessionRecord {
	var latest *SessionRecord
	for index := range sessions {
		session := sessions[index]
		if session.EndedAt == nil || latest != nil && !session.StartedAt.After(latest.StartedAt) {
			continue
		}
		copy := session
		latest = &copy
	}
	return latest
}

func campaignHomeLocation(sessions []SessionRecord, latest *SessionRecord) string {
	for index := len(sessions) - 1; index >= 0; index-- {
		if sessions[index].EndedAt == nil {
			return sessions[index].LocationName
		}
	}
	if latest != nil {
		return latest.LocationName
	}
	return ""
}

func openCampaignThreads(workspace Workspace, scope Scope) []Record {
	enabled := workspace.EnabledSourceIDs(scope)
	var threads []Record
	for _, record := range workspace.Records {
		if record.Type != Thread || record.Authority == Proposal || record.Authority == Superseded || !RecordVisibleIn(record, scope, enabled) || !recordHasTag(record, "open") {
			continue
		}
		threads = append(threads, record)
	}
	sort.SliceStable(threads, func(i, j int) bool { return threads[i].Title < threads[j].Title })
	return threads
}

func recordHasTag(record Record, want string) bool {
	for _, tag := range record.Tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}

func unresolvedCampaignReviews(reconciliations []ReconciliationRecord, sessions []SessionRecord) (int, string) {
	sessionIDs := map[string]bool{}
	for _, session := range sessions {
		sessionIDs[session.ID] = true
	}
	count := 0
	nextID := ""
	var nextAt time.Time
	for _, reconciliation := range reconciliations {
		if !sessionIDs[reconciliation.SessionID] {
			continue
		}
		unresolved := 0
		for _, item := range reconciliation.Items {
			if ReconciliationUnresolved(item.Status) {
				unresolved++
			}
		}
		count += unresolved
		if unresolved > 0 && (nextID == "" || reconciliation.CreatedAt.After(nextAt)) {
			nextID = reconciliation.ID
			nextAt = reconciliation.CreatedAt
		}
	}
	return count, nextID
}

func recentCampaignChanges(workspace Workspace, sessions []SessionRecord, limit int) []CampaignHomeChange {
	sessionIDs := map[string]bool{}
	for _, session := range sessions {
		sessionIDs[session.ID] = true
	}
	byID := map[string]Record{}
	for _, record := range workspace.Records {
		byID[record.ID] = record
	}
	var changes []CampaignHomeChange
	for _, reconciliation := range workspace.Reconciliations {
		if !sessionIDs[reconciliation.SessionID] {
			continue
		}
		for _, item := range reconciliation.Items {
			if item.Status != ReconApproved {
				continue
			}
			recordID := item.Mutation.RecordID
			if recordID == "" {
				recordID = item.RecordID
			}
			title := "Session note"
			if record, ok := byID[recordID]; ok {
				title = record.Title
			}
			changes = append(changes, CampaignHomeChange{RecordID: recordID, Title: title, Summary: item.Summary, At: item.CreatedAt})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].At.After(changes[j].At) })
	if limit > 0 && len(changes) > limit {
		changes = changes[:limit]
	}
	return changes
}

func campaignHomeCast(workspace Workspace, scope Scope, plan *PlannedNotes, sessions []SessionRecord, latest *SessionRecord) []Record {
	var links []EntityLink
	if plan != nil {
		links = plan.Links
	} else {
		for index := len(sessions) - 1; index >= 0; index-- {
			if sessions[index].EndedAt == nil {
				links = sessions[index].Links
				break
			}
		}
		if len(links) == 0 && latest != nil {
			links = latest.Links
		}
	}
	enabled := workspace.EnabledSourceIDs(scope)
	byID := map[string]Record{}
	for _, record := range workspace.Records {
		if RecordVisibleIn(record, scope, enabled) {
			byID[record.ID] = record
		}
	}
	seen := map[string]bool{}
	var cast []Record
	for _, link := range links {
		record, ok := byID[link.RecordID]
		if !ok || seen[record.ID] || record.Type != NPC && record.Type != Character {
			continue
		}
		seen[record.ID] = true
		cast = append(cast, record)
	}
	return cast
}
