package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// PlannedNotes is prep for an upcoming sit. It is not a live session and does
// not turn into one by flipping status; live play may consume it as context.
type PlannedNotes struct {
	ID              string
	Title           string
	Scope           Scope
	Body            string // markdown prep notes
	Links           []EntityLink
	LocationID      string
	LocationName    string
	PriorSessionIDs []string // optional context from earlier live sits
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (p PlannedNotes) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("planned notes ID is required")
	}
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("planned notes %q title is required", p.ID)
	}
	return nil
}

// ParsePlannedBody extracts @ entity links and an optional #location marker
// from markdown prep notes without mutating the body text.
func ParsePlannedBody(body string, records []Record) (links []EntityLink, locationID, locationName string) {
	seen := map[string]bool{}
	for index := 0; index < len(body); {
		at := strings.IndexByte(body[index:], '@')
		if at < 0 {
			break
		}
		at += index
		if at > 0 {
			prev := body[at-1]
			if prev != ' ' && prev != '\n' && prev != '(' && prev != '[' {
				index = at + 1
				continue
			}
		}
		end := at + 1
		for end < len(body) {
			r := body[end]
			if r == ' ' || r == '\n' || r == ',' || r == '.' || r == ';' || r == ':' || r == ')' || r == ']' {
				break
			}
			end++
		}
		token := body[at+1 : end]
		if token == "" {
			index = end
			continue
		}
		if record, ok := matchRecordTitle(records, token); ok {
			if !seen[record.ID] {
				links = append(links, EntityLink{Text: token, RecordID: record.ID})
				seen[record.ID] = true
			}
		}
		index = end
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(trimmed), "#location ") {
			continue
		}
		name := strings.TrimSpace(trimmed[len("#location "):])
		if name == "" {
			continue
		}
		locationName = name
		if record, ok := matchRecordTitle(records, name); ok && record.Type == Location {
			locationID = record.ID
			locationName = record.Title
		}
		break
	}
	return links, locationID, locationName
}

func matchRecordTitle(records []Record, query string) (Record, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return Record{}, false
	}
	var best Record
	bestScore := 0
	for _, record := range records {
		if record.Authority == Proposal {
			continue
		}
		title := strings.ToLower(record.Title)
		score := 0
		switch {
		case title == needle:
			score = 4
		case strings.HasPrefix(title, needle):
			score = 3
		case strings.Contains(title, needle):
			score = 2
		}
		for _, alias := range record.Aliases {
			alias = strings.ToLower(alias)
			if alias == needle && score < 4 {
				score = 4
			} else if strings.HasPrefix(alias, needle) && score < 3 {
				score = 3
			}
		}
		if score > bestScore {
			best = record
			bestScore = score
		}
	}
	if bestScore == 0 {
		return Record{}, false
	}
	return best, true
}

// RefreshPlannedLinks re-resolves @ / #location markers from Body into Links
// and location fields. Body text is left unchanged.
func (p PlannedNotes) RefreshPlannedLinks(records []Record) PlannedNotes {
	links, locationID, locationName := ParsePlannedBody(p.Body, records)
	p.Links = links
	p.LocationID = locationID
	if locationName != "" {
		p.LocationName = locationName
	}
	p.UpdatedAt = time.Now().UTC()
	return p
}

const defaultPriorSitLimit = 3

// PriorSit is derived context from an earlier live session for prep notes.
type PriorSit struct {
	ID           string
	Title        string
	LocationName string
	Cast         []string
	LastLine     string
	StartedAt    time.Time
}

func (s PriorSit) Summary() string {
	parts := []string{s.Title}
	if s.LocationName != "" {
		parts = append(parts, s.LocationName)
	}
	if len(s.Cast) > 0 {
		cast := s.Cast
		if len(cast) > 3 {
			cast = cast[:3]
		}
		parts = append(parts, strings.Join(cast, ", "))
	}
	line := strings.Join(parts, " · ")
	if s.LastLine != "" {
		line += " · “" + truncate(s.LastLine, 40) + "”"
	}
	return line
}

// DefaultPriorSessionIDs returns the most recent ended sessions in scope.
func DefaultPriorSessionIDs(sessions []SessionRecord, scope Scope, limit int) []string {
	if limit <= 0 {
		limit = defaultPriorSitLimit
	}
	sits := endedSessionsInScope(sessions, scope)
	if len(sits) > limit {
		sits = sits[:limit]
	}
	ids := make([]string, 0, len(sits))
	for _, session := range sits {
		ids = append(ids, session.ID)
	}
	return ids
}

// ResolvePriorSits loads prep context for the given session IDs, newest first.
func ResolvePriorSits(sessions []SessionRecord, records []Record, ids []string) []PriorSit {
	if len(ids) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		if id != "" {
			want[id] = true
		}
	}
	out := make([]PriorSit, 0, len(want))
	for _, session := range sessions {
		if !want[session.ID] {
			continue
		}
		out = append(out, priorSitFrom(session, records))
	}
	sortPriorSitsNewestFirst(out)
	return out
}

func endedSessionsInScope(sessions []SessionRecord, scope Scope) []SessionRecord {
	out := make([]SessionRecord, 0)
	for _, session := range sessions {
		if session.EndedAt == nil {
			continue
		}
		if session.Scope.WorldID != scope.WorldID || session.Scope.CampaignID != scope.CampaignID {
			continue
		}
		out = append(out, session)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

func priorSitFrom(session SessionRecord, records []Record) PriorSit {
	cast := make([]string, 0, len(session.Links))
	seen := map[string]bool{}
	for _, link := range session.Links {
		title := link.Text
		for _, record := range records {
			if record.ID == link.RecordID {
				title = record.Title
				break
			}
		}
		if title == "" || seen[title] {
			continue
		}
		seen[title] = true
		cast = append(cast, title)
	}
	last := ""
	for i := len(session.Entries) - 1; i >= 0; i-- {
		if session.Entries[i].Undone {
			continue
		}
		last = strings.TrimSpace(session.Entries[i].Text)
		if last != "" {
			break
		}
	}
	return PriorSit{
		ID:           session.ID,
		Title:        session.Title,
		LocationName: session.LocationName,
		Cast:         cast,
		LastLine:     last,
		StartedAt:    session.StartedAt,
	}
}

func sortPriorSitsNewestFirst(sits []PriorSit) {
	sort.Slice(sits, func(i, j int) bool {
		return sits[i].StartedAt.After(sits[j].StartedAt)
	})
}
