package tui

import (
	"strconv"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/prefs"
)

type hitAction int

const (
	hitNone hitAction = iota
	hitSelectRecord
	hitAcceptSuggestion
	hitPinReview
	hitClearReview
	hitFocusPane
	hitCampaignType
)

type hitTarget struct {
	MinX, MaxX int
	MinY, MaxY int
	Action     hitAction
	Record     *domain.Record
	Suggestion int
	EntityType domain.EntityType
	Pane       prefs.Pane
}

type contentLine struct {
	Text       string
	Action     hitAction
	Record     *domain.Record
	Suggestion int
	EntityType domain.EntityType
	PinX       int // column offset within content; -1 if unused
	ClearX     int
}

func (m Model) sessionHitTargets() []hitTarget {
	if m.session == nil || m.width == 0 || m.height == 0 {
		return nil
	}
	upperHeight := m.sessionUpperHeight()
	transcriptHeight := m.sessionTranscriptHeight()
	inputHeight := m.sessionInputHeight()
	divider := m.splitWidth(m.width)
	upperTop := 1
	transcriptTop := upperTop + upperHeight
	inputTop := transcriptTop + transcriptHeight

	var hits []hitTarget
	campaignOn := m.layout.SessionLeafVisible(prefs.PaneCampaign)
	contextOn := m.layout.SessionLeafVisible(prefs.PaneContext)
	switch {
	case campaignOn && contextOn:
		hits = append(hits, hitTarget{MinX: 0, MaxX: divider - 1, MinY: upperTop, MaxY: upperTop + upperHeight - 1, Action: hitFocusPane, Pane: prefs.PaneCampaign})
		hits = append(hits, hitTarget{MinX: divider, MaxX: m.width - 1, MinY: upperTop, MaxY: upperTop + upperHeight - 1, Action: hitFocusPane, Pane: prefs.PaneContext})
		hits = append(hits, m.panelHits(0, divider, upperTop, upperHeight, m.campaignContentLines())...)
		hits = append(hits, m.panelHits(divider, m.width-divider, upperTop, upperHeight, m.contextContentLines())...)
	case contextOn:
		hits = append(hits, hitTarget{MinX: 0, MaxX: m.width - 1, MinY: upperTop, MaxY: upperTop + upperHeight - 1, Action: hitFocusPane, Pane: prefs.PaneContext})
		hits = append(hits, m.panelHits(0, m.width, upperTop, upperHeight, m.contextContentLines())...)
	default:
		hits = append(hits, hitTarget{MinX: 0, MaxX: m.width - 1, MinY: upperTop, MaxY: upperTop + upperHeight - 1, Action: hitFocusPane, Pane: prefs.PaneCampaign})
		hits = append(hits, m.panelHits(0, m.width, upperTop, upperHeight, m.campaignContentLines())...)
	}
	hits = append(hits, hitTarget{MinX: 0, MaxX: m.width - 1, MinY: transcriptTop, MaxY: transcriptTop + transcriptHeight - 1, Action: hitFocusPane, Pane: prefs.PaneTranscript})
	hits = append(hits, m.panelHits(0, m.width, inputTop, inputHeight, m.inputContentLines())...)
	hits = append(hits, hitTarget{
		MinX: 0, MaxX: m.width - 1,
		MinY: inputTop, MaxY: inputTop + inputHeight - 1,
		Action: hitFocusPane, Pane: prefs.PaneInput,
	})
	return hits
}

func (m Model) panelHits(panelX, panelWidth, panelY, panelHeight int, lines []contentLine) []hitTarget {
	inner := panelInnerHeight(panelHeight)
	contentTop := panelY + 2
	contentLeft := panelX + 3
	contentRight := panelX + panelWidth - 2
	if contentRight < contentLeft {
		contentRight = contentLeft
	}
	visible := lines
	if len(visible) > inner {
		visible = visible[:inner]
	}
	hits := make([]hitTarget, 0, len(visible)+2)
	for index, line := range visible {
		y := contentTop + index
		if line.Action != hitNone {
			hits = append(hits, hitTarget{
				MinX: contentLeft, MaxX: contentRight,
				MinY: y, MaxY: y,
				Action:     line.Action,
				Record:     line.Record,
				Suggestion: line.Suggestion,
				EntityType: line.EntityType,
			})
		}
		if line.PinX >= 0 {
			hits = append(hits, hitTarget{
				MinX: contentLeft + line.PinX, MaxX: contentLeft + line.PinX + 4,
				MinY: y, MaxY: y,
				Action: hitPinReview,
			})
		}
		if line.ClearX >= 0 {
			hits = append(hits, hitTarget{
				MinX: contentLeft + line.ClearX, MaxX: contentLeft + line.ClearX + 4,
				MinY: y, MaxY: y,
				Action: hitClearReview,
			})
		}
	}
	return hits
}

func (m Model) hitTest(x, y int) (hitTarget, bool) {
	hits := m.sessionHitTargets()
	for i := len(hits) - 1; i >= 0; i-- {
		hit := hits[i]
		if hit.Action == hitFocusPane {
			continue
		}
		if x >= hit.MinX && x <= hit.MaxX && y >= hit.MinY && y <= hit.MaxY {
			return hit, true
		}
	}
	for _, hit := range hits {
		if hit.Action != hitFocusPane {
			continue
		}
		if x >= hit.MinX && x <= hit.MaxX && y >= hit.MinY && y <= hit.MaxY {
			return hit, true
		}
	}
	return hitTarget{}, false
}

func (m Model) campaignContentLines() []contentLine {
	lines := []contentLine{
		{Text: "CAMPAIGN", PinX: -1, ClearX: -1},
		{Text: "", PinX: -1, ClearX: -1},
	}
	sections := []struct {
		label      string
		entityType domain.EntityType
		count      int
	}{
		{label: "Session"},
		{label: "World"},
		{label: "NPCs", entityType: domain.NPC, count: m.countByType(domain.NPC)},
		{label: "Locations", entityType: domain.Location, count: m.countByType(domain.Location)},
		{label: "Factions", entityType: domain.Faction, count: m.countByType(domain.Faction)},
		{label: "Threads", entityType: domain.Thread, count: m.countByType(domain.Thread)},
		{label: "Items", entityType: domain.Item, count: m.countByType(domain.Item)},
		{label: "Notes", entityType: domain.Note, count: m.countByType(domain.Note)},
	}
	navIndex := 0
	for _, section := range sections {
		marker := "  "
		if section.entityType != "" {
			if navIndex == m.campaignCursor && m.sessionFocus() == prefs.PaneCampaign {
				marker = "▸ "
			}
			navIndex++
		} else if section.label == "Session" && m.sessionFocus() != prefs.PaneCampaign {
			marker = "▸ "
		}
		text := marker + section.label
		if section.entityType != "" {
			count := strconv.Itoa(section.count)
			text += strings.Repeat(" ", max(1, 14-len(section.label)-len(count))) + count
		}
		line := contentLine{Text: text, PinX: -1, ClearX: -1}
		if section.entityType != "" {
			line.Action = hitCampaignType
			line.EntityType = section.entityType
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) contextContentLines() []contentLine {
	lines := []contentLine{
		{Text: "CURRENT SCENE", PinX: -1, ClearX: -1},
		{Text: "", PinX: -1, ClearX: -1},
	}

	locationName := ""
	locationSummary := ""
	var location *domain.Record
	if m.session != nil {
		locationName = m.session.LocationName
	}
	if record := m.sessionLocationRecord(); record != nil {
		location = record
		locationName = record.Title
		locationSummary = record.Summary
		if locationSummary == "" {
			locationSummary = record.Body
		}
	}
	if locationName == "" {
		lines = append(lines,
			contentLine{Text: "No location set", PinX: -1, ClearX: -1},
			contentLine{Text: "Use #location Name", PinX: -1, ClearX: -1},
			contentLine{Text: "", PinX: -1, ClearX: -1},
		)
	} else {
		locLine := contentLine{Text: locationName, PinX: -1, ClearX: -1}
		if location != nil {
			locLine.Action = hitSelectRecord
			locLine.Record = location
		}
		lines = append(lines, locLine)
		if locationSummary != "" {
			for _, wrapped := range strings.Split(m.renderMarkdown(locationSummary, 36), "\n") {
				lines = append(lines, contentLine{Text: wrapped, PinX: -1, ClearX: -1})
			}
		}
		lines = append(lines, contentLine{Text: "", PinX: -1, ClearX: -1})
	}

	if m.review != nil {
		pinLabel := "pin"
		if m.reviewPinned {
			pinLabel = "unpin"
		}
		prefix := "SELECTED  ["
		pinX := len(prefix)
		clearX := pinX + len(pinLabel) + len("] [")
		lines = append(lines,
			contentLine{Text: prefix + pinLabel + "] [clear]", PinX: pinX, ClearX: clearX},
			contentLine{Text: m.review.Title, PinX: -1, ClearX: -1},
			contentLine{Text: "", PinX: -1, ClearX: -1},
		)
	}

	lines = append(lines, contentLine{Text: "PRESENT", PinX: -1, ClearX: -1})
	present := m.sessionPresent()
	if len(present) == 0 {
		lines = append(lines, contentLine{Text: "  —", PinX: -1, ClearX: -1})
	} else {
		for index := range present {
			record := present[index]
			marker := "◆ "
			itemIndex := index
			if location := m.sessionLocationRecord(); location != nil {
				itemIndex++
			}
			if m.sessionFocus() == prefs.PaneContext && m.contextCursor == itemIndex {
				marker = "▸ "
			} else if m.review != nil && m.review.ID == record.ID {
				marker = "▸ "
			}
			candidate := record
			lines = append(lines, contentLine{
				Text:   marker + record.Title,
				Action: hitSelectRecord,
				Record: &candidate,
				PinX:   -1,
				ClearX: -1,
			})
		}
	}
	lines = append(lines, contentLine{Text: "", PinX: -1, ClearX: -1})
	lines = append(lines, contentLine{Text: "ACTIVE THREADS", PinX: -1, ClearX: -1})
	threads := m.sessionThreads()
	if len(threads) == 0 {
		lines = append(lines, contentLine{Text: "  —", PinX: -1, ClearX: -1})
	} else {
		offset := len(present)
		if m.sessionLocationRecord() != nil {
			offset++
		}
		for index := range threads {
			record := threads[index]
			marker := "  "
			if m.sessionFocus() == prefs.PaneContext && m.contextCursor == offset+index {
				marker = "▸ "
			} else if m.review != nil && m.review.ID == record.ID {
				marker = "▸ "
			}
			candidate := record
			lines = append(lines, contentLine{
				Text:   marker + record.Title,
				Action: hitSelectRecord,
				Record: &candidate,
				PinX:   -1,
				ClearX: -1,
			})
		}
	}
	return lines
}

func (m Model) inputContentLines() []contentLine {
	lines := []contentLine{{Text: "INPUT", PinX: -1, ClearX: -1}}
	if len(m.suggestions) == 0 {
		return lines
	}
	lines = append(lines, contentLine{Text: "suggestions  click / Tab insert", PinX: -1, ClearX: -1})
	for index, item := range m.suggestions {
		if index >= 5 {
			break
		}
		prefix := "  "
		if index == m.suggestion {
			prefix = "▸ "
		}
		line := contentLine{
			Text:       prefix + item.Label,
			Action:     hitAcceptSuggestion,
			Suggestion: index,
			PinX:       -1,
			ClearX:     -1,
		}
		if item.Record != nil {
			candidate := *item.Record
			line.Record = &candidate
		}
		lines = append(lines, line)
	}
	return lines
}

func renderContentLines(lines []contentLine) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, line.Text)
	}
	return strings.Join(parts, "\n")
}
