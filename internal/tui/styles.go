package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

var (
	colorBackground = lipgloss.Color("#16161E")
	colorSurface    = lipgloss.Color("#1F2335")
	colorBorder     = lipgloss.Color("#414868")
	colorText       = lipgloss.Color("#C0CAF5")
	colorMuted      = lipgloss.Color("#7F849C")
	colorAccent     = lipgloss.Color("#7AA2F7")
	colorCanon      = lipgloss.Color("#9ECE6A")
	colorSecret     = lipgloss.Color("#BB9AF7")
	colorDraft      = lipgloss.Color("#E0AF68")
	colorProposal   = lipgloss.Color("#F7768E")

	appStyle = lipgloss.NewStyle().
			Background(colorBackground).
			Foreground(colorText)

	headerStyle = lipgloss.NewStyle().
			Background(colorSurface).
			Padding(0, 1)

	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(colorText)
	detailTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText).MarginTop(1)
	mutedStyle       = lipgloss.NewStyle().Foreground(colorMuted)
	sectionStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	typeStyle        = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	labelStyle       = lipgloss.NewStyle().Bold(true).Foreground(colorMuted)
	helpStyle        = lipgloss.NewStyle().Foreground(colorMuted).MarginTop(1)
	footerStyle      = lipgloss.NewStyle().Background(colorSurface).Foreground(colorMuted).Padding(0, 1)
	filterStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	searchTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccent).
				Padding(1, 2)

	normalItemStyle   = lipgloss.NewStyle().Foreground(colorText)
	selectedItemStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	proposalWarningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorProposal).
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorProposal).
				Padding(0, 1)

	draftNoticeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorDraft).
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorDraft).
				Padding(0, 1)

	searchPanelStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSurface).
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorAccent).
				Padding(1, 2)

	brokenRefStyle = lipgloss.NewStyle().Foreground(colorProposal)
	refStyle       = lipgloss.NewStyle().Foreground(colorAccent)

	searchResultStyle         = lipgloss.NewStyle().Foreground(colorText)
	selectedSearchResultStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
)

func authorityStyle(authority domain.Authority) lipgloss.Style {
	color := colorMuted
	switch authority {
	case domain.Canon:
		color = colorCanon
	case domain.Secret:
		color = colorSecret
	case domain.Draft:
		color = colorDraft
	case domain.Proposal:
		color = colorProposal
	case domain.Unknown:
		color = colorMuted
	}
	return lipgloss.NewStyle().Bold(true).Foreground(color)
}
