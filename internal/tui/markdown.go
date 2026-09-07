package tui

import (
	"fmt"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

var (
	markdownMu     sync.Mutex
	markdownWidth  int
	markdownTerm   *glamour.TermRenderer
	markdownFailed bool
)

func paneMarkdownStyle() ansi.StyleConfig {
	style := styles.TokyoNightStyleConfig
	zero := uint(0)
	style.Document.Margin = &zero
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = ""
	style.H1.Prefix = ""
	style.H2.Prefix = ""
	style.H3.Prefix = ""
	style.H4.Prefix = ""
	style.H5.Prefix = ""
	style.H6.Prefix = ""
	style.Table.BlockSuffix = "\n"
	style.CodeBlock.Margin = &zero
	quoteToken := "│"
	style.BlockQuote.IndentToken = &quoteToken
	if style.CodeBlock.Chroma != nil {
		style.CodeBlock.Chroma.Background.BackgroundColor = nil
		style.CodeBlock.Chroma.Error.BackgroundColor = nil
	}
	return style
}

func isolateMarkdownTables(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+4)
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isRow := strings.HasPrefix(trimmed, "|")
		if isRow {
			inTable = true
			out = append(out, line)
			continue
		}
		if inTable {
			inTable = false
			if trimmed != "" {
				out = append(out, "")
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func flattenWideMarkdownTables(text string, width int) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+4)
	for i := 0; i < len(lines); {
		if !isPipeTableRow(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && isPipeTableRow(lines[i]) {
			i++
		}
		block := lines[start:i]
		headers, rows, ok := parseMarkdownTable(block)
		if !ok || len(rows) == 0 || !markdownTableExceedsWidth(headers, rows, width) {
			out = append(out, block...)
			continue
		}
		out = append(out, strings.Split(flattenMarkdownTable(headers, rows), "\n")...)
	}
	return strings.Join(out, "\n")
}

func isPipeTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

func splitTableRow(line string) []string {
	trim := strings.TrimSpace(line)
	trim = strings.TrimPrefix(trim, "|")
	trim = strings.TrimSuffix(trim, "|")
	parts := strings.Split(trim, "|")
	cells := make([]string, len(parts))
	for i, part := range parts {
		cells[i] = strings.TrimSpace(part)
	}
	return cells
}

func isTableSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if cell == "" || !strings.ContainsRune(cell, '-') {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return true
}

func parseMarkdownTable(block []string) ([]string, [][]string, bool) {
	var rows [][]string
	sep := -1
	for i, line := range block {
		cells := splitTableRow(line)
		if isTableSeparator(cells) {
			if sep >= 0 {
				return nil, nil, false
			}
			sep = i
			continue
		}
		rows = append(rows, cells)
	}
	if sep != 1 || len(rows) == 0 {
		return nil, nil, false
	}
	return rows[0], rows[1:], true
}

func markdownTableExceedsWidth(headers []string, rows [][]string, width int) bool {
	return markdownTableMinWidth(headers, rows) > width
}

func markdownTableMinWidth(headers []string, rows [][]string) int {
	cols := len(headers)
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return 0
	}
	widths := make([]int, cols)
	grow := func(index int, value string) {
		if index < 0 || index >= cols {
			return
		}
		if w := lipgloss.Width(value); w > widths[index] {
			widths[index] = w
		}
	}
	for i, header := range headers {
		grow(i, header)
	}
	for _, row := range rows {
		for i, cell := range row {
			grow(i, cell)
		}
	}
	total := 0
	for _, w := range widths {
		if w < 1 {
			w = 1
		}
		// Glamour tables add cell padding plus a column rule.
		total += w + 3
	}
	return total
}

func flattenMarkdownTable(headers []string, rows [][]string) string {
	cell := func(row []string, index int) string {
		if index < 0 || index >= len(row) {
			return ""
		}
		return row[index]
	}
	formatPair := func(header, value string) string {
		header = strings.TrimSpace(header)
		value = strings.TrimSpace(value)
		switch {
		case header != "" && value != "":
			return "**" + header + "** " + value
		case header != "":
			return "**" + header + "**"
		default:
			return value
		}
	}
	if len(rows) == 1 {
		parts := make([]string, 0, len(headers))
		for i, header := range headers {
			if pair := formatPair(header, cell(rows[0], i)); pair != "" {
				parts = append(parts, pair)
			}
		}
		return strings.Join(parts, "  ")
	}
	var builder strings.Builder
	for r, row := range rows {
		if r > 0 {
			builder.WriteByte('\n')
		}
		for i, header := range headers {
			if pair := formatPair(header, cell(row, i)); pair != "" {
				builder.WriteString("- ")
				builder.WriteString(pair)
				builder.WriteByte('\n')
			}
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func clampANSIWidth(s string, width int) string {
	if width < 1 {
		width = 1
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = trimANSIRight(line)
		if lipgloss.Width(line) <= width {
			out = append(out, line)
			continue
		}
		for _, part := range strings.Split(lipgloss.Wrap(line, width, ""), "\n") {
			out = append(out, trimANSIRight(part))
		}
	}
	return strings.Join(out, "\n")
}

func trimANSIRight(s string) string {
	plain := xansi.Strip(s)
	want := strings.TrimRight(plain, " \t")
	drop := lipgloss.Width(plain) - lipgloss.Width(want)
	if drop <= 0 {
		return s
	}
	keep := lipgloss.Width(s) - drop
	if keep <= 0 {
		return ""
	}
	return xansi.Truncate(s, keep, "")
}

func panelInnerWidth(panelWidth int) int {
	const chrome = 6 // left/right border + horizontal padding
	return max(1, panelWidth-chrome)
}

func (m Model) defaultMarkdownWidth() int {
	if m.width <= 0 {
		return 72
	}
	return max(24, panelInnerWidth(m.width*2/5))
}

func stripRedundantTitleHeading(body, title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return body
	}
	trimmed := strings.TrimLeft(body, "\n")
	line, rest, found := strings.Cut(trimmed, "\n")
	heading := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
	if !strings.EqualFold(heading, title) {
		return body
	}
	if !found {
		return ""
	}
	return strings.TrimLeft(rest, "\n")
}

func (m Model) renderMarkdown(text string, width int) string {
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if width < 16 {
		width = 16
	}
	prepared, restore := m.protectMentions(flattenWideMarkdownTables(isolateMarkdownTables(text), width))
	rendered, err := renderMarkdownANSI(prepared, width)
	if err != nil {
		return clampANSIWidth(m.renderProseWithMentions(text), width)
	}
	return clampANSIWidth(restore(rendered), width)
}

func (m Model) protectMentions(text string) (string, func(string) string) {
	mentions := m.resolveMentions(text)
	if len(mentions) == 0 {
		return text, func(s string) string { return s }
	}
	type slot struct {
		token  string
		styled string
	}
	slots := make([]slot, 0, len(mentions))
	var builder strings.Builder
	last := 0
	for index, mention := range mentions {
		if mention.Start < last || mention.End > len(text) {
			continue
		}
		builder.WriteString(text[last:mention.Start])
		token := fmt.Sprintf("⟦m%d⟧", index)
		raw := text[mention.Start:mention.End]
		styled := refStyle.Render(raw)
		if mention.RecordID == "" {
			styled = brokenRefStyle.Render(raw)
		}
		slots = append(slots, slot{token: token, styled: styled})
		builder.WriteString(token)
		last = mention.End
	}
	builder.WriteString(text[last:])
	return builder.String(), func(rendered string) string {
		for _, item := range slots {
			rendered = strings.ReplaceAll(rendered, item.token, item.styled)
		}
		return rendered
	}
}

func renderMarkdownANSI(text string, width int) (string, error) {
	markdownMu.Lock()
	defer markdownMu.Unlock()
	if markdownFailed {
		return "", fmt.Errorf("markdown renderer unavailable")
	}
	if markdownTerm == nil || markdownWidth != width {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStyles(paneMarkdownStyle()),
			glamour.WithWordWrap(width),
			glamour.WithPreservedNewLines(),
		)
		if err != nil {
			markdownFailed = true
			return "", err
		}
		markdownTerm = renderer
		markdownWidth = width
	}
	out, err := markdownTerm.Render(text)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
