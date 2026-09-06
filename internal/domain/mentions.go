package domain

import "strings"

// Mention is one @ reference in prose. RecordID is empty when the target
// cannot be resolved (deleted, renamed, or never existed).
type Mention struct {
	Text     string
	RecordID string
	Start    int
	End      int
}

func mentionBoundary(b byte) bool {
	switch b {
	case ' ', '\n', '\t', ',', '.', ';', ':', ')', ']', '!', '?', '\'', '"':
		return true
	default:
		return false
	}
}

func atBoundary(text string, index int) bool {
	if index == 0 {
		return true
	}
	prev := text[index-1]
	return mentionBoundary(prev) || prev == '(' || prev == '['
}

// MentionsIn finds @ references in text. Names match the longest title or
// alias. Unmatched tokens are returned with an empty RecordID so the prose
// can show a broken ref without rewriting the source.
func MentionsIn(text string, records []Record) []Mention {
	if text == "" {
		return nil
	}
	out := make([]Mention, 0)
	for index := 0; index < len(text); {
		at := strings.IndexByte(text[index:], '@')
		if at < 0 {
			break
		}
		at += index
		if !atBoundary(text, at) {
			index = at + 1
			continue
		}
		rest := text[at+1:]
		if rest == "" {
			break
		}
		if record, n := longestRecordName(records, rest); n > 0 {
			out = append(out, Mention{
				Text:     rest[:n],
				RecordID: record.ID,
				Start:    at,
				End:      at + 1 + n,
			})
			index = at + 1 + n
			continue
		}
		end := at + 1 + unmatchedMentionLen(rest)
		if end > at+1 {
			out = append(out, Mention{
				Text:  text[at+1 : end],
				Start: at,
				End:   end,
			})
		}
		index = end
	}
	return out
}

func longestRecordName(records []Record, rest string) (Record, int) {
	bestLen := 0
	var best Record
	lower := strings.ToLower(rest)
	for _, record := range records {
		if record.Authority == Proposal {
			continue
		}
		names := append([]string{record.Title}, record.Aliases...)
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			n := len(name)
			if n <= bestLen || n > len(rest) {
				continue
			}
			if !strings.HasPrefix(lower, strings.ToLower(name)) {
				continue
			}
			if n < len(rest) && !mentionBoundary(rest[n]) && rest[n] != '@' {
				continue
			}
			best = record
			bestLen = n
		}
	}
	return best, bestLen
}

func unmatchedMentionLen(rest string) int {
	if rest == "" {
		return 0
	}
	i := 0
	consumeWord := func() int {
		start := i
		for i < len(rest) && isNameChar(rest[i]) {
			i++
		}
		return i - start
	}
	if consumeWord() == 0 {
		return 0
	}
	for i < len(rest) && rest[i] == ' ' {
		if i+1 >= len(rest) || !isUpperName(rest[i+1]) {
			break
		}
		space := i
		i++
		if consumeWord() == 0 {
			i = space
			break
		}
	}
	return i
}

func isNameChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '\''
}

func isUpperName(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

// RecordProse is summary plus body, the wiki text that can hold @ mentions.
func RecordProse(record Record) string {
	switch {
	case record.Summary != "" && record.Body != "":
		return record.Summary + "\n\n" + record.Body
	case record.Body != "":
		return record.Body
	default:
		return record.Summary
	}
}

// EntityOutgoingRefs lists unique mentions from a wiki record. Mentions that
// resolve to the record itself are omitted.
func EntityOutgoingRefs(record Record, records []Record) (resolved, broken []Mention) {
	seenID := map[string]bool{}
	seenBroken := map[string]bool{}
	for _, mention := range MentionsIn(RecordProse(record), records) {
		if mention.RecordID != "" {
			if mention.RecordID == record.ID || seenID[mention.RecordID] {
				continue
			}
			seenID[mention.RecordID] = true
			resolved = append(resolved, mention)
			continue
		}
		key := strings.ToLower(mention.Text)
		if key == "" || seenBroken[key] {
			continue
		}
		seenBroken[key] = true
		broken = append(broken, mention)
	}
	return resolved, broken
}
