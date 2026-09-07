package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

type Scope int

const (
	CurrentCampaign Scope = iota
	CurrentWorld
	EntireLibrary
)

func (s Scope) Label() string {
	switch s {
	case CurrentCampaign:
		return "campaign"
	case CurrentWorld:
		return "world"
	case EntireLibrary:
		return "library"
	default:
		return "unknown"
	}
}

type Filter struct {
	Query            string
	Scope            Scope
	WorldID          string
	CampaignID       string
	EnabledSourceIDs []string
	Types            map[domain.EntityType]bool
	Tags             []string // optional; all must match (AND)
	IncludeProposals bool
}

type Result struct {
	Record domain.Record
	Score  int
}

type Service struct {
	records []domain.Record
}

func New(records []domain.Record) Service {
	return Service{records: records}
}

func (s Service) Find(filter Filter) []Result {
	query := normalize(filter.Query)
	results := make([]Result, 0, len(s.records))

	for _, record := range s.records {
		if !inScope(record, filter) || !typeAllowed(record, filter.Types) || !tagsAllowed(record, filter.Tags) {
			continue
		}
		if record.Authority == domain.Proposal && !filter.IncludeProposals {
			continue
		}

		score, ok := recordScore(record, query)
		if !ok {
			continue
		}
		results = append(results, Result{Record: record, Score: score})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Record.Title < results[j].Record.Title
		}
		return results[i].Score > results[j].Score
	})

	return results
}

func inScope(record domain.Record, filter Filter) bool {
	switch filter.Scope {
	case CurrentCampaign:
		return domain.RecordVisibleIn(record, domain.Scope{
			WorldID:    filter.WorldID,
			CampaignID: filter.CampaignID,
		}, filter.EnabledSourceIDs)
	case CurrentWorld:
		return record.Scope.WorldID == filter.WorldID
	case EntireLibrary:
		return true
	default:
		return false
	}
}

func typeAllowed(record domain.Record, types map[domain.EntityType]bool) bool {
	return len(types) == 0 || types[record.Type]
}

func tagsAllowed(record domain.Record, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, want := range required {
		found := false
		for _, have := range record.Tags {
			if strings.EqualFold(have, want) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func recordScore(record domain.Record, query string) (int, bool) {
	if query == "" {
		return 1, true
	}

	title := normalize(record.Title)
	if title == query {
		return 1000, true
	}
	if strings.HasPrefix(title, query) {
		return 800 - len(title), true
	}
	if strings.Contains(title, query) {
		return 600 - strings.Index(title, query), true
	}

	for _, alias := range record.Aliases {
		if strings.Contains(normalize(alias), query) {
			return 500, true
		}
	}

	if score, ok := fuzzyScore(title, query); ok {
		return 400 + score, true
	}

	searchable := normalize(strings.Join([]string{
		record.Summary,
		record.Body,
		strings.Join(record.Tags, " "),
		record.Source,
	}, " "))
	if index := strings.Index(searchable, query); index >= 0 {
		return 200 - min(index, 150), true
	}

	return 0, false
}

func fuzzyScore(value, query string) (int, bool) {
	valueRunes := []rune(value)
	queryRunes := []rune(query)
	position := 0
	score := 0
	lastMatch := -2

	for _, wanted := range queryRunes {
		found := false
		for position < len(valueRunes) {
			if valueRunes[position] == wanted {
				if position == lastMatch+1 {
					score += 5
				} else {
					score++
				}
				lastMatch = position
				position++
				found = true
				break
			}
			position++
		}
		if !found {
			return 0, false
		}
	}

	return score, true
}

func normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
