package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/prefs"
)

type homeActionID string

const (
	homePrep    homeActionID = "prep"
	homeStart   homeActionID = "start"
	homeReview  homeActionID = "review"
	homeCapture homeActionID = "capture"
)

type homeAction struct {
	ID      homeActionID
	Label   string
	Detail  string
	Enabled bool
}

func (m Model) campaignHome() domain.CampaignHomeSummary {
	return domain.DeriveCampaignHome(m.workspace, m.workspace.Scope)
}

func (m Model) homeActions() []homeAction {
	home := m.campaignHome()
	prep := homeAction{ID: homePrep, Label: "Draft prep", Detail: "Create notes for the next sit", Enabled: true}
	if home.NextPlan != nil {
		prep.Label = "Resume prep"
		prep.Detail = home.NextPlan.Title
	}
	review := homeAction{ID: homeReview, Label: "Review changes", Detail: "Nothing waiting", Enabled: false}
	if home.UnresolvedReviews > 0 {
		review.Detail = fmt.Sprintf("%d items waiting", home.UnresolvedReviews)
		review.Enabled = true
	}
	return []homeAction{
		prep,
		{ID: homeStart, Label: "Start session", Detail: "Use the next prep and cast", Enabled: true},
		review,
		{ID: homeCapture, Label: "Capture note", Detail: "Keep an idea as a draft", Enabled: true},
	}
}

func (m Model) activateHomeAction(index int) (tea.Model, tea.Cmd) {
	actions := m.homeActions()
	if len(actions) == 0 {
		return m, nil
	}
	action := actions[clamp(index, 0, len(actions)-1)]
	if !action.Enabled {
		m.status = action.Detail
		return m, nil
	}
	home := m.campaignHome()
	switch action.ID {
	case homePrep:
		if home.NextPlan == nil {
			return m.openPlannedNotes(true)
		}
		m.selectedPlanID = home.NextPlan.ID
		m.planID = home.NextPlan.ID
		return m.openPlannedNotes(false)
	case homeStart:
		if home.NextPlan != nil {
			m.selectedPlanID = home.NextPlan.ID
		}
		return m.startSession()
	case homeReview:
		for index := range m.workspace.Reconciliations {
			if m.workspace.Reconciliations[index].ID == home.NextReviewID {
				m.reconIndex = index
				break
			}
		}
		return m.openReconciliation()
	case homeCapture:
		m.typeFilter = domain.Note
		return m.openEditor(true)
	default:
		return m, nil
	}
}

func (m Model) renderHomeActions() string {
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("NEXT ACTION"))
	if m.layout.Focus == prefs.PaneList {
		builder.WriteString(mutedStyle.Render(" · focused"))
	}
	builder.WriteString("\n\n")
	for index, action := range m.homeActions() {
		cursor := "  "
		style := normalItemStyle
		if index == m.cursor {
			cursor = "▸ "
			style = selectedItemStyle
		}
		if !action.Enabled {
			style = mutedStyle
		}
		builder.WriteString(style.Render(cursor + action.Label + " · " + action.Detail))
		builder.WriteString("\n")
	}
	return builder.String()
}

func (m Model) renderCampaignHome() string {
	home := m.campaignHome()
	var builder strings.Builder
	builder.WriteString(sectionStyle.Render("CAMPAIGN HOME"))
	builder.WriteString("\n\n")
	builder.WriteString(titleStyle.Render(m.workspace.Scope.Label()))
	builder.WriteString("\n")
	if home.NextPlan == nil {
		builder.WriteString(mutedStyle.Render("No upcoming prep yet"))
	} else {
		builder.WriteString("Next: " + home.NextPlan.Title)
		if home.NextPlan.LocationName != "" {
			builder.WriteString(" · " + home.NextPlan.LocationName)
		}
	}
	builder.WriteString("\n")
	if home.LatestSession == nil {
		builder.WriteString(mutedStyle.Render("No completed sessions yet"))
	} else {
		builder.WriteString("Last: " + home.LatestSession.Title)
	}
	if home.CurrentLocation != "" {
		builder.WriteString("\nCurrent location: " + home.CurrentLocation)
	}
	builder.WriteString("\n\n")
	builder.WriteString(labelStyle.Render("ACTIVE CAST"))
	builder.WriteString("\n")
	if len(home.Cast) == 0 {
		builder.WriteString(mutedStyle.Render("No cast linked from prep or the latest sit"))
	} else {
		for _, record := range home.Cast {
			builder.WriteString("  " + record.Authority.Marker() + " " + record.Title + "\n")
		}
	}
	builder.WriteString("\n")
	builder.WriteString(labelStyle.Render(fmt.Sprintf("OPEN THREADS · %d", len(home.OpenThreads))))
	builder.WriteString("\n")
	if len(home.OpenThreads) == 0 {
		builder.WriteString(mutedStyle.Render("No threads tagged open"))
	} else {
		for index, thread := range home.OpenThreads {
			if index == 5 {
				builder.WriteString(mutedStyle.Render(fmt.Sprintf("  + %d more", len(home.OpenThreads)-index)))
				builder.WriteString("\n")
				break
			}
			builder.WriteString("  " + thread.Authority.Marker() + " " + thread.Title + "\n")
		}
	}
	builder.WriteString("\n")
	builder.WriteString(labelStyle.Render(fmt.Sprintf("REVIEW · %d REMAINING", home.UnresolvedReviews)))
	builder.WriteString("\n")
	if len(home.RecentChanges) == 0 {
		builder.WriteString(mutedStyle.Render("No approved changes from recent play"))
	} else {
		for _, change := range home.RecentChanges {
			builder.WriteString("  " + change.Title + "\n")
		}
	}
	builder.WriteString("\n\n")
	builder.WriteString(mutedStyle.Render("Tab actions · Enter run · tree stays one pane away"))
	return builder.String()
}
