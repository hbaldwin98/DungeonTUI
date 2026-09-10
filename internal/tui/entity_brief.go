package tui

import (
	"fmt"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

// briefSightingRows bounds the sightings shown inline so a long-running NPC
// does not push metadata off a short terminal.
const briefSightingRows = 3

// briefChangeRows bounds the change list for the same reason.
const briefChangeRows = 4

// entityBrief derives the current record's table brief.
func (m Model) entityBrief(record domain.Record) domain.EntityBrief {
	return domain.DeriveEntityBrief(m.workspace, record, m.workspace.Scope)
}

// renderEntityBriefSections renders the middle of entity detail in the scan
// order for this entity type. Identity is rendered by the caller above it.
func (m Model) renderEntityBriefSections(record domain.Record) string {
	brief := m.entityBrief(record)
	hops := m.wikiDetailHops()
	var builder strings.Builder
	for _, section := range domain.EntityBriefOrder(record.Type) {
		block := m.renderEntityBriefSection(section, record, brief, hops)
		if block == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(block)
	}
	if m.layout.Focus == prefs.PaneDetail {
		builder.WriteString("\n")
		builder.WriteString(mutedStyle.Render("j/k select · Enter preview · Enter again follow"))
		builder.WriteString("\n")
	}
	return builder.String()
}

func (m Model) renderEntityBriefSection(section domain.EntityBriefSection, record domain.Record, brief domain.EntityBrief, hops []detailHop) string {
	switch section {
	case domain.BriefWhyNow:
		return renderBriefWhyNow(brief)
	case domain.BriefLastSeen:
		return renderBriefLastSeen(brief)
	case domain.BriefConnected:
		return m.renderBriefConnected(hops)
	case domain.BriefChanged:
		return m.renderBriefChanged(brief, hops)
	case domain.BriefOpen:
		return renderBriefOpen(brief)
	case domain.BriefMeta:
		return m.renderBriefMeta(record, brief)
	default:
		return ""
	}
}

func briefHeading(label string) string {
	return labelStyle.Render(label) + "\n"
}

func renderBriefWhyNow(brief domain.EntityBrief) string {
	var builder strings.Builder
	builder.WriteString(briefHeading("WHY NOW"))
	if len(brief.WhyNow) == 0 {
		builder.WriteString(mutedStyle.Render("  — no prep, sit, or review link"))
		builder.WriteString("\n")
		return builder.String()
	}
	for _, reason := range brief.WhyNow {
		builder.WriteString("  " + reason + "\n")
	}
	return builder.String()
}

func renderBriefLastSeen(brief domain.EntityBrief) string {
	var builder strings.Builder
	builder.WriteString(briefHeading("LAST SEEN"))
	if brief.LastSeen == nil {
		if brief.ReferenceOnly {
			builder.WriteString(mutedStyle.Render("  — imported reference · unused"))
		} else {
			builder.WriteString(mutedStyle.Render("  — not yet seen in play"))
		}
		builder.WriteString("\n")
		return builder.String()
	}
	builder.WriteString("  " + brief.LastSeen.Label() + "\n")
	for index, sighting := range brief.Sightings {
		if index == 0 {
			continue
		}
		if index >= briefSightingRows {
			builder.WriteString(mutedStyle.Render(fmt.Sprintf("  + %d earlier %s", len(brief.Sightings)-index, pluralWord(len(brief.Sightings)-index, "sit", "sits"))))
			builder.WriteString("\n")
			break
		}
		builder.WriteString(mutedStyle.Render("  " + sighting.Label()))
		builder.WriteString("\n")
	}
	return builder.String()
}

// renderBriefConnected consolidates outgoing @ references and every backlink
// into one list, so "what connects to this" is a single scan rather than two.
func (m Model) renderBriefConnected(hops []detailHop) string {
	var builder strings.Builder
	builder.WriteString(briefHeading("CONNECTED"))
	wrote := false
	for _, group := range []string{"ref", "linked"} {
		for index, hop := range hops {
			if hop.Section != group {
				continue
			}
			wrote = true
			builder.WriteString(m.renderHopLineWithRelation(index, hop.Prefix, hop.Label, hop.Relation, hop.Kind == hopBroken))
			builder.WriteString("\n")
		}
	}
	if !wrote {
		builder.WriteString(mutedStyle.Render("  — nothing references this"))
		builder.WriteString("\n")
	}
	return builder.String()
}

// renderBriefChanged shows reviewed changes first, then the navigable session
// history rows those changes came from.
func (m Model) renderBriefChanged(brief domain.EntityBrief, hops []detailHop) string {
	var builder strings.Builder
	builder.WriteString(briefHeading("WHAT CHANGED"))
	wrote := false
	for index, change := range brief.Changes {
		if index >= briefChangeRows {
			builder.WriteString(mutedStyle.Render(fmt.Sprintf("  + %d more", len(brief.Changes)-index)))
			builder.WriteString("\n")
			break
		}
		wrote = true
		line := "  " + change.Summary
		if change.Status != "" {
			line += "  " + mutedStyle.Render("("+string(change.Status)+")")
		}
		builder.WriteString(line + "\n")
	}
	history := m.renderBriefHistoryRows(hops)
	if history != "" {
		wrote = true
		builder.WriteString(history)
	}
	if !wrote {
		builder.WriteString(mutedStyle.Render("  — no recorded changes"))
		builder.WriteString("\n")
	}
	return builder.String()
}

func (m Model) renderBriefHistoryRows(hops []detailHop) string {
	record := m.selectedRecord()
	if record == nil {
		return ""
	}
	historyByID := map[string]domain.SessionHistoryRow{}
	for _, row := range domain.EntitySessionHistory(m.workspace, record.ID) {
		historyByID[row.SessionID] = row
	}
	var builder strings.Builder
	for index, hop := range hops {
		if hop.Section != "history" {
			continue
		}
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
		builder.WriteString("\n")
		if showEvents {
			for _, event := range row.Events {
				builder.WriteString(mutedStyle.Render("      · " + event.Summary))
				builder.WriteString("\n")
			}
		}
	}
	return builder.String()
}

func renderBriefOpen(brief domain.EntityBrief) string {
	if len(brief.Open) == 0 {
		// An empty OPEN block is the common case and says nothing useful, so
		// it is omitted rather than shown as a row of dashes.
		return ""
	}
	var builder strings.Builder
	builder.WriteString(briefHeading("OPEN"))
	for _, item := range brief.Open {
		line := "  " + item.Summary
		if item.Detail != "" {
			line += "  " + mutedStyle.Render("("+item.Detail+")")
		}
		builder.WriteString(line + "\n")
	}
	return builder.String()
}

// renderBriefMeta trails the brief: scope, tags, collections, and provenance
// stay fully visible, but they are what you check after the entity has already
// answered the questions above it.
func (m Model) renderBriefMeta(record domain.Record, brief domain.EntityBrief) string {
	var builder strings.Builder
	builder.WriteString(briefHeading("DETAILS"))
	builder.WriteString("  " + labelStyle.Render("SCOPE") + "  " + entityScopeLabel(record.Scope) + "\n")
	builder.WriteString("  " + labelStyle.Render("AUTHORITY") + "  " +
		authorityStyle(record.Authority).Render(record.Authority.Marker()+" "+record.Authority.Label()) + "\n")
	if len(record.Tags) > 0 {
		builder.WriteString("  " + labelStyle.Render("TAGS") + "  #" + strings.Join(record.Tags, "  #") + "\n")
	}
	if cols := domain.CollectionsContaining(m.workspace.Collections, m.workspace.Scope, record.ID); len(cols) > 0 {
		names := make([]string, 0, len(cols))
		for _, col := range cols {
			names = append(names, col.Title)
		}
		builder.WriteString("  " + labelStyle.Render("COLLECTIONS") + "  " + strings.Join(names, "  ·  ") + "\n")
	}
	source := strings.TrimSpace(record.Source)
	if source == "" {
		source = "—"
	}
	builder.WriteString("  " + labelStyle.Render("SOURCE") + "  " + source + "\n")
	if brief.ReferenceOnly {
		builder.WriteString("  " + labelStyle.Render("OWNED") + "  " + mutedStyle.Render("external material · Dungeon does not own this text") + "\n")
	}
	for _, line := range domain.CaptureProvenanceLines(record) {
		builder.WriteString("  " + mutedStyle.Render(line) + "\n")
	}
	return builder.String()
}

func pluralWord(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}
