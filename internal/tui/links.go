package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/ingest/fivetools"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

type hopKind string

const (
	hopWiki      hopKind = "wiki"
	hopReference hopKind = "reference"
	hopBroken    hopKind = "broken"
	hopPrep      hopKind = "prep"
	hopSession   hopKind = "session"
	hopHistory   hopKind = "history"
	hopRule      hopKind = "rule"
)

type detailHop struct {
	Kind       hopKind
	Section    string
	Prefix     string
	Label      string
	Relation   string
	RecordID   string
	SessionID  string
	PlanID     string
	EntryID    string
	RuleKind   string
	RuleSource string
}

func (m Model) detailHops() []detailHop {
	switch m.currentNav().Kind {
	case NavSessions:
		return m.sessionDetailHops()
	case NavPrep:
		return m.prepDetailHops()
	default:
		return m.wikiDetailHops()
	}
}

func (m Model) wikiDetailHops() []detailHop {
	record := m.selectedRecord()
	if record == nil {
		return nil
	}
	hops := make([]detailHop, 0)
	resolved, broken := domain.EntityOutgoingRefs(*record, m.workspace.Records)
	broken = m.bindReferenceMentions(broken)
	stillBroken := make([]domain.Mention, 0, len(broken))
	for _, mention := range broken {
		if mention.RecordID == "" {
			stillBroken = append(stillBroken, mention)
			continue
		}
		resolved = append(resolved, mention)
	}
	broken = stillBroken
	for _, mention := range resolved {
		title := mention.Text
		kind := hopWiki
		prefix := "wiki · "
		relation := "outgoing @ mention"
		if rec, ok := m.lookupAny(mention.RecordID); ok {
			title = rec.Title
			if fivetools.IsReferenceID(rec.ID) {
				kind = hopReference
				prefix = "reference · "
				relation = "outgoing source reference"
			}
		}
		hops = append(hops, detailHop{Kind: kind, Section: "ref", Prefix: prefix, Label: title, Relation: relation, RecordID: mention.RecordID})
	}
	for _, mention := range broken {
		hops = append(hops, detailHop{Kind: hopBroken, Section: "ref", Prefix: "missing · @", Label: mention.Text, Relation: "unresolved @ mention"})
	}
	seenSession := map[string]int{}
	for _, link := range domain.EntityBacklinks(m.workspace, record.ID) {
		switch link.Kind {
		case domain.BacklinkWiki:
			hops = append(hops, detailHop{Kind: hopWiki, Section: "linked", Prefix: "wiki · ", Label: link.Title, Relation: "backlink · mentions this", RecordID: link.ID})
		case domain.BacklinkPrep:
			hops = append(hops, detailHop{Kind: hopPrep, Section: "linked", Prefix: "prep · ", Label: link.Title, Relation: "backlink · prep cast", PlanID: link.ID})
		case domain.BacklinkSessionAssoc, domain.BacklinkTranscript:
			relation := "backlink · session cast"
			if link.Kind == domain.BacklinkTranscript {
				relation = "backlink · session transcript"
				if link.Count > 0 {
					relation = fmt.Sprintf("%s · %d hits", relation, link.Count)
				}
			}
			if index, ok := seenSession[link.ID]; ok {
				if link.Kind == domain.BacklinkTranscript && !strings.Contains(hops[index].Relation, "transcript") {
					hops[index].Relation += " + transcript"
				}
				continue
			}
			seenSession[link.ID] = len(hops)
			hops = append(hops, detailHop{Kind: hopSession, Section: "linked", Prefix: "session · ", Label: link.Title, Relation: relation, SessionID: link.ID})
		}
	}
	for _, row := range domain.EntitySessionHistory(m.workspace, record.ID) {
		hops = append(hops, detailHop{Kind: hopHistory, Section: "history", Label: row.Title, Relation: "session history", SessionID: row.SessionID, EntryID: historyEntryID(row)})
	}
	return hops
}

func historyEntryID(row domain.SessionHistoryRow) string {
	for _, event := range row.Events {
		if event.Kind == domain.HistoryTranscript && event.EntryID != "" {
			return event.EntryID
		}
	}
	for _, event := range row.Events {
		if event.EntryID != "" {
			return event.EntryID
		}
	}
	return ""
}

func (m Model) sessionDetailHops() []detailHop {
	session := m.selectedSession()
	if session == nil {
		return nil
	}
	hops := make([]detailHop, 0)
	if session.LocationID != "" {
		title := session.LocationName
		if title == "" {
			title = session.LocationID
		}
		hops = append(hops, detailHop{Kind: hopWiki, Section: "cast", Prefix: "location · ", Label: title, Relation: "session context · location", RecordID: session.LocationID})
	}
	if session.PlannedNotesID != "" {
		label := "Prep notes"
		if plan := m.plannedByID(session.PlannedNotesID); plan != nil {
			label = plan.Title
		}
		hops = append(hops, detailHop{Kind: hopPrep, Section: "cast", Prefix: "prep · ", Label: label, Relation: "session context · planned prep", PlanID: session.PlannedNotesID})
	}
	for _, link := range session.Links {
		title := link.Text
		kind := hopBroken
		if target, ok := recordByID(m.workspace.Records, link.RecordID); ok {
			title = target.Title
			kind = hopWiki
		} else if strings.TrimSpace(title) == "" {
			title = link.RecordID
		}
		prefix := "@ "
		relation := "session cast"
		if kind == hopBroken {
			prefix = "missing · @"
			relation = "unresolved session cast"
		}
		hops = append(hops, detailHop{Kind: kind, Section: "cast", Prefix: prefix, Label: title, Relation: relation, RecordID: link.RecordID})
	}
	return hops
}

func (m Model) prepDetailHops() []detailHop {
	if m.selectedPlanID == "" {
		return nil
	}
	plan := m.plannedByID(m.selectedPlanID)
	if plan == nil {
		return nil
	}
	hops := make([]detailHop, 0)
	if plan.LocationID != "" {
		title := plan.LocationName
		if title == "" {
			title = plan.LocationID
		}
		hops = append(hops, detailHop{Kind: hopWiki, Section: "cast", Prefix: "location · ", Label: title, Relation: "prep context · location", RecordID: plan.LocationID})
	}
	for _, link := range plan.Links {
		title := link.Text
		kind := hopBroken
		if target, ok := recordByID(m.workspace.Records, link.RecordID); ok {
			title = target.Title
			kind = hopWiki
		}
		prefix := "@ "
		relation := "prep cast"
		if kind == hopBroken {
			prefix = "missing · @"
			relation = "unresolved prep cast"
		}
		hops = append(hops, detailHop{Kind: kind, Section: "cast", Prefix: prefix, Label: title, Relation: relation, RecordID: link.RecordID})
	}
	for _, sit := range domain.ResolvePriorSits(m.workspace.Sessions, m.workspace.Records, plan.PriorSessionIDs) {
		hops = append(hops, detailHop{Kind: hopSession, Section: "cast", Prefix: "prior · ", Label: sit.Title, Relation: "linked session · prior", SessionID: sit.ID})
	}
	for _, sit := range domain.SessionsSeededFrom(m.workspace.Sessions, plan.ID) {
		hops = append(hops, detailHop{Kind: hopSession, Section: "cast", Prefix: "live · ", Label: sit.Title, Relation: "linked session · seeded from prep", SessionID: sit.ID})
	}
	return hops
}

func (m *Model) moveDetailCursor(delta int) {
	hops := m.detailHops()
	if len(hops) == 0 {
		return
	}
	m.historyCursor = clamp(m.historyCursor+delta, 0, len(hops)-1)
}

func (m *Model) focusPrep(id string) {
	m.focusNavKind(NavPrep)
	m.selectedPlanID = id
	m.selectedID = ""
	m.selectedSessionID = ""
	m.selectedFolderPath = ""
	m.historyCursor = 0
	m.layout.Focus = prefs.PaneDetail
	plans := m.scopedPlannedNotes()
	for index, plan := range plans {
		if plan.ID == id {
			m.cursor = index
			return
		}
	}
}

func (m *Model) focusSession(id string) {
	m.focusNavKind(NavSessions)
	m.selectedSessionID = id
	m.selectedID = ""
	m.selectedPlanID = ""
	m.historyCursor = 0
	m.layout.Focus = prefs.PaneDetail
	m.syncSessionTreeCursor()
}

func (m *Model) focusNavKind(kind NavKind) {
	for index, entry := range m.navEntries() {
		if entry.Kind != kind {
			continue
		}
		m.navCursor = index
		m.navKind = entry.Kind
		m.navType = entry.Type
		if kind == NavType {
			m.typeFilter = entry.Type
		} else {
			m.typeFilter = ""
		}
		return
	}
}

func (m Model) hopMarker(index int) (cursor string, style lipgloss.Style) {
	if m.layout.Focus == prefs.PaneDetail && index == m.historyCursor {
		return "▸ ", selectedItemStyle
	}
	return "  ", normalItemStyle
}

func (m Model) renderProseWithMentions(text string) string {
	mentions := m.resolveMentions(text)
	if len(mentions) == 0 {
		return text
	}
	var builder strings.Builder
	last := 0
	for _, mention := range mentions {
		if mention.Start < last || mention.Start > len(text) || mention.End > len(text) {
			continue
		}
		builder.WriteString(text[last:mention.Start])
		token := text[mention.Start:mention.End]
		if mention.RecordID == "" {
			builder.WriteString(brokenRefStyle.Render(token))
		} else {
			builder.WriteString(refStyle.Render(token))
		}
		last = mention.End
	}
	builder.WriteString(text[last:])
	return builder.String()
}

func (m Model) renderHopLine(index int, prefix, label string, broken bool) string {
	return m.renderHopLineWithRelation(index, prefix, label, "", broken)
}

func (m Model) renderHopLineWithRelation(index int, prefix, label, relation string, broken bool) string {
	cursor, style := m.hopMarker(index)
	if broken {
		style = brokenRefStyle
		if m.layout.Focus == prefs.PaneDetail && index == m.historyCursor {
			style = selectedItemStyle
		}
	}
	line := style.Render(fmt.Sprintf("%s%s%s", cursor, prefix, label))
	if relation != "" {
		line += "  " + mutedStyle.Render("("+m.detailRelation(relation)+")")
	}
	return line
}

func (m Model) detailRelation(relation string) string {
	if m.width >= 120 {
		return relation
	}
	switch {
	case relation == "outgoing @ mention":
		return "outgoing @"
	case relation == "outgoing source reference":
		return "source reference"
	case relation == "unresolved @ mention":
		return "unresolved @"
	case relation == "backlink · mentions this":
		return "backlink"
	case relation == "backlink · prep cast":
		return "prep backlink"
	case strings.HasPrefix(relation, "backlink · session transcript"):
		return "session transcript"
	case relation == "backlink · session cast":
		return "session cast"
	case relation == "session history":
		return "history"
	case relation == "session context · location", relation == "prep context · location":
		return "location context"
	case relation == "session context · planned prep":
		return "planned prep"
	case relation == "unresolved session cast", relation == "unresolved prep cast":
		return "unresolved cast"
	case relation == "linked session · prior":
		return "prior session"
	case relation == "linked session · seeded from prep":
		return "prep-linked session"
	case relation == "inline @ reference":
		return "inline @"
	case relation == "inline source reference":
		return "inline source"
	default:
		return truncateImportLine(relation, 20)
	}
}

func (m Model) renderCastHops() string {
	hops := m.detailHops()
	var builder strings.Builder
	builder.WriteString(labelStyle.Render("LINKS"))
	builder.WriteString("\n")
	wrote := false
	for index, hop := range hops {
		if hop.Section != "cast" {
			continue
		}
		wrote = true
		builder.WriteString(m.renderHopLineWithRelation(index, hop.Prefix, hop.Label, hop.Relation, hop.Kind == hopBroken))
		builder.WriteRune('\n')
	}
	if !wrote {
		builder.WriteString(mutedStyle.Render("  —"))
		builder.WriteRune('\n')
	}
	if m.layout.Focus == prefs.PaneDetail {
		builder.WriteString(mutedStyle.Render("j/k select · Enter preview · Enter again follow"))
		builder.WriteRune('\n')
	}
	return builder.String()
}

func recordByID(records []domain.Record, id string) (domain.Record, bool) {
	for _, record := range records {
		if record.ID == id {
			return record, true
		}
	}
	return domain.Record{}, false
}

func (m Model) renderEntityGraph(record domain.Record) string {
	hops := m.wikiDetailHops()
	history := domain.EntitySessionHistory(m.workspace, record.ID)
	historyByID := map[string]domain.SessionHistoryRow{}
	for _, row := range history {
		historyByID[row.SessionID] = row
	}
	var builder strings.Builder
	builder.WriteString(labelStyle.Render("REFERENCES"))
	builder.WriteString("\n")
	wroteRef := false
	for index, hop := range hops {
		if hop.Section != "ref" {
			continue
		}
		wroteRef = true
		if hop.Kind == hopBroken {
			builder.WriteString(m.renderHopLineWithRelation(index, hop.Prefix, hop.Label, hop.Relation, true))
		} else {
			builder.WriteString(m.renderHopLineWithRelation(index, hop.Prefix, hop.Label, hop.Relation, false))
		}
		builder.WriteRune('\n')
	}
	if !wroteRef {
		builder.WriteString(mutedStyle.Render("  — no @ mentions in this record"))
		builder.WriteRune('\n')
	}

	builder.WriteString("\n")
	builder.WriteString(labelStyle.Render("LINKED"))
	builder.WriteString("\n")
	wroteLinked := false
	for index, hop := range hops {
		if hop.Section != "linked" {
			continue
		}
		wroteLinked = true
		builder.WriteString(m.renderHopLineWithRelation(index, hop.Prefix, hop.Label, hop.Relation, hop.Kind == hopBroken))
		builder.WriteRune('\n')
	}
	if !wroteLinked {
		builder.WriteString(mutedStyle.Render("  — no backlinks yet"))
		builder.WriteRune('\n')
	}

	builder.WriteString("\n")
	builder.WriteString(labelStyle.Render("HISTORY"))
	builder.WriteString("\n")
	wroteHistory := false
	for index, hop := range hops {
		if hop.Section != "history" {
			continue
		}
		wroteHistory = true
		row, ok := historyByID[hop.SessionID]
		stamp := ""
		parts := make([]string, 0, 3)
		if ok {
			stamp = row.StartedAt.Local().Format("2006-01-02") + " · "
			if row.Associated {
				parts = append(parts, "cast")
			}
			if row.TranscriptHits > 0 {
				parts = append(parts, fmt.Sprintf("%d @", row.TranscriptHits))
			}
			if row.ReconItems > 0 {
				parts = append(parts, fmt.Sprintf("%d recon", row.ReconItems))
			}
		}
		expand := ""
		showEvents := ok && len(row.Events) > 0 && m.layout.Focus == prefs.PaneDetail && index == m.historyCursor
		if showEvents {
			expand = " ▼"
		} else if ok && len(row.Events) > 0 {
			expand = " ▸"
		}
		meta := hop.Label
		if len(parts) > 0 {
			meta += " · " + strings.Join(parts, ", ")
		}
		builder.WriteString(m.renderHopLineWithRelation(index, stamp, meta+expand, hop.Relation, false))
		builder.WriteRune('\n')
		if showEvents {
			for _, event := range row.Events {
				builder.WriteString(mutedStyle.Render("      · " + event.Summary))
				builder.WriteString("\n")
			}
		}
	}
	if !wroteHistory {
		builder.WriteString(mutedStyle.Render("  — no session history yet"))
		builder.WriteRune('\n')
	}
	if m.layout.Focus == prefs.PaneDetail {
		builder.WriteString(mutedStyle.Render("j/k select · Enter preview · Enter again follow"))
		builder.WriteRune('\n')
	}
	return builder.String()
}
