package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

const markdownEditorMaxLines = 10000

func dungeonTextAreaStyles() textarea.Styles {
	styles := textarea.DefaultDarkStyles()
	styles.Focused = textarea.StyleState{
		Base:             lipgloss.NewStyle().Foreground(colorText),
		Text:             lipgloss.NewStyle().Foreground(colorText),
		CursorLine:       lipgloss.NewStyle(),
		CursorLineNumber: lipgloss.NewStyle().Foreground(colorAccent),
		LineNumber:       lipgloss.NewStyle().Foreground(colorMuted),
		Placeholder:      lipgloss.NewStyle().Foreground(colorMuted),
		Prompt:           lipgloss.NewStyle().Foreground(colorMuted),
		EndOfBuffer:      lipgloss.NewStyle().Foreground(colorBorder),
		Selection:        lipgloss.NewStyle().Background(lipgloss.Color("#3D59A1")).Foreground(colorText),
	}
	styles.Blurred = styles.Focused
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(colorMuted)
	styles.Cursor.Color = colorAccent
	styles.Cursor.Blink = true
	return styles
}

func newMarkdownTextArea(width, height int) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	// Bubbles derives the gutter width from MaxHeight. Match its hard line
	// limit so line-number digit changes never shift the document text.
	ta.MaxHeight = markdownEditorMaxLines
	ta.MaxWidth = 0
	ta.SetStyles(dungeonTextAreaStyles())
	// Ctrl+T is reserved for cycling entity type in the entity editor.
	ta.KeyMap.TransposeCharacterBackward = key.NewBinding()
	sizeMarkdownTextArea(&ta, width, height)
	return ta
}

func insertMarkdownText(ta *textarea.Model, msg tea.KeyPressMsg) bool {
	if msg.Text == "" || msg.Code == tea.KeyTab || ta.HasSelection() {
		return false
	}
	// Let Update handle a key that may move the cursor onto another visual
	// row; it owns viewport content and can scroll that new row into view.
	if ta.LineInfo().CharOffset+lipgloss.Width(msg.Text) >= ta.Width()-1 {
		return false
	}
	ta.InsertString(msg.Text)
	return true
}

// editorListKey moves through the @ suggestions with ↑↓ and scrolls the peek
// with PgUp/PgDn. It reports false when that list is not showing, so the
// key reaches the text instead.
func (m *Model) editorListKey(key string) bool {
	switch key {
	case "up", "down":
		if len(m.suggestions) == 0 {
			return false
		}
		delta := map[string]int{"up": -1, "down": 1}[key]
		m.suggestion = clamp(m.suggestion+delta, 0, len(m.suggestions)-1)
		m.refreshPeek()
		return true
	case "pgup", "pgdown":
		if m.peek == nil {
			return false
		}
		m.scrollPeek(map[string]int{"pgup": -1, "pgdown": 1}[key])
		return true
	}
	return false
}

func sizeMarkdownTextArea(ta *textarea.Model, termWidth, termHeight int) {
	sizeMarkdownTextAreaReserved(ta, termWidth, termHeight, 0)
}

func sizeMarkdownTextAreaReserved(ta *textarea.Model, termWidth, termHeight, reserve int) {
	width := max(40, termWidth-6)
	height := max(6, termHeight-6-reserve)
	ta.SetWidth(width)
	ta.SetHeight(height)
}

func focusTitleHeading(ta *textarea.Model) {
	lines := strings.Split(ta.Value(), "\n")
	target := -1
	col := 0
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			target = index
			col = strings.Index(line, "# ")
			if col < 0 {
				col = 0
			}
			col += 2
			break
		}
	}
	if target < 0 {
		ta.MoveToBegin()
		return
	}
	restoreTextAreaCursor(ta, target, col)
}

// restoreTextAreaCursor places the cursor at a logical line and column.
// CursorDown steps visual rows, so a soft-wrapped line takes several steps;
// it walks until the logical line is reached or the cursor stops moving.
func restoreTextAreaCursor(ta *textarea.Model, line, col int) {
	ta.MoveToBegin()
	line = clamp(line, 0, max(0, ta.LineCount()-1))
	for ta.Line() < line {
		row, column := ta.Line(), ta.Column()
		ta.CursorDown()
		if ta.Line() == row && ta.Column() == column {
			break
		}
	}
	ta.SetCursorColumn(col)
}

func rewriteTypePreservingCursor(ta *textarea.Model, entityType domain.EntityType) {
	line := ta.Line()
	col := ta.Column()
	ta.SetValue(rewriteMarkdownType(ta.Value(), entityType))
	if line >= ta.LineCount() {
		line = max(0, ta.LineCount()-1)
	}
	restoreTextAreaCursor(ta, line, col)
}

func editorChrome(label, meta, help, status string, width int) string {
	var builder strings.Builder
	builder.WriteString(searchTitleStyle.Render(label))
	if meta != "" {
		builder.WriteString("  ")
		builder.WriteString(mutedStyle.Render(meta))
	}
	builder.WriteString("\n")
	if status != "" {
		builder.WriteString(draftNoticeStyle.Render(status))
		builder.WriteString("\n")
	}
	builder.WriteString(mutedStyle.Render(strings.Repeat("─", max(8, width-4))))
	builder.WriteString("\n")
	_ = help
	return builder.String()
}

func editorHelp(help string) string {
	return helpStyle.UnsetMarginTop().Render(help)
}

func editorFooter(termWidth int, help string) string {
	return footerStyle.Width(max(1, termWidth)).Render(help)
}

func renderFullScreenEditor(termWidth, termHeight int, chrome, body, help string) string {
	footer := editorFooter(termWidth, help)
	chromeHeight := lipgloss.Height(chrome)
	footerHeight := 1
	bodyHeight := max(1, termHeight-chromeHeight-footerHeight)
	framed := lipgloss.NewStyle().
		Width(max(1, termWidth)).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(fitLines(body, bodyHeight))
	content := lipgloss.JoinVertical(lipgloss.Left, chrome, framed, footer)
	return appStyle.
		Width(max(1, termWidth)).
		Height(max(1, termHeight)).
		MaxHeight(max(1, termHeight)).
		Render(content)
}

func markdownEditorHelp(entityType domain.EntityType) string {
	return fmt.Sprintf("Ctrl+G help · Ctrl+S save · Ctrl+T type (%s) · @ suggest · Esc", entityType)
}
