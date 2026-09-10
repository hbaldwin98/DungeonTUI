package domain

import (
	"fmt"
	"strings"
	"unicode"
)

type BeatKind string

const (
	BeatScene     BeatKind = "scene"
	BeatEncounter BeatKind = "encounter"
	BeatClue      BeatKind = "clue"
	BeatNPC       BeatKind = "npc"
	BeatLocation  BeatKind = "location"
	BeatTreasure  BeatKind = "treasure"
	BeatFreeform  BeatKind = "freeform"
)

type PlannedBeat struct {
	ID        string
	Kind      BeatKind
	Title     string
	Level     int
	StartLine int
	EndLine   int
}

func ParsePlannedOutline(body string) []PlannedBeat {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var beats []PlannedBeat
	inFence := false
	seen := map[string]int{}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		level, heading, ok := plannedHeading(trimmed)
		if !ok {
			continue
		}
		kind, title := classifyPlannedBeat(heading)
		key := normalizeBeatID(heading)
		seen[key]++
		beats = append(beats, PlannedBeat{ID: fmt.Sprintf("%s-%d", key, seen[key]), Kind: kind, Title: title, Level: level, StartLine: index})
	}
	for index := range beats {
		beats[index].EndLine = len(lines)
		if index+1 < len(beats) {
			beats[index].EndLine = beats[index+1].StartLine
		}
	}
	if len(beats) == 0 && strings.TrimSpace(body) != "" {
		return []PlannedBeat{{ID: "notes-1", Kind: BeatFreeform, Title: "Notes", EndLine: len(lines)}}
	}
	return beats
}

func plannedHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[level+1:]), "#"))
	return level, title, title != ""
}

func classifyPlannedBeat(heading string) (BeatKind, string) {
	lower := strings.ToLower(heading)
	prefixes := []struct {
		labels []string
		kind   BeatKind
	}{
		{[]string{"scene"}, BeatScene},
		{[]string{"encounter"}, BeatEncounter},
		{[]string{"clue", "revelation"}, BeatClue},
		{[]string{"npc beat", "npc"}, BeatNPC},
		{[]string{"location"}, BeatLocation},
		{[]string{"treasure"}, BeatTreasure},
		{[]string{"beat"}, BeatFreeform},
	}
	for _, candidate := range prefixes {
		for _, label := range candidate.labels {
			prefix := label + ":"
			if strings.HasPrefix(lower, prefix) {
				title := strings.TrimSpace(heading[len(prefix):])
				if title == "" {
					title = heading
				}
				return candidate.kind, title
			}
		}
	}
	return BeatFreeform, heading
}

func normalizeBeatID(value string) string {
	var builder strings.Builder
	separator := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			separator = false
		} else if !separator && builder.Len() > 0 {
			builder.WriteByte('-')
			separator = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
