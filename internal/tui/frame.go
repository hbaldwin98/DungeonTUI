package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// fillFrame pads composited output to exactly width×height cells. The
// compositor drops trailing blank cells, so an overlay frame would otherwise
// leave rows shorter than the terminal and let stale cells show through.
func fillFrame(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for index, line := range lines {
		if gap := width - lipgloss.Width(line); gap > 0 {
			lines[index] = line + strings.Repeat(" ", gap)
		}
	}
	return strings.Join(lines, "\n")
}
