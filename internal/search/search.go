package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

type Scope int

const (
	CurrentCampaign Scope = iota
	CurrentWorld
	EntireLibrary
	RulesReference
)

func (s Scope) Label() string {
	switch s {
	case CurrentCampaign:
		return "campaign"
	case CurrentWorld:
		return "world"
	case EntireLibrary:
		return "library"
	case RulesReference:
		return "5e rules"
	default:
		return "unknown"
	}
}

type Kind string

const (
	KindRecord     Kind = "record"
	KindPrep       Kind = "prep"
	KindSession    Kind = "session"
	KindTranscript Kind = "transcript"
	KindRecon      Kind = "recon"
	KindReference  Kind = "reference"
	KindRule       Kind = "rule"
)

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
	Kind      Kind
	ID        string
	SessionID string
	Title     string
	Snippet   string
	Record    domain.Record
	Score     int

	// RuleKind and RuleSource describe an external 5e-cli hit (KindRule).
	RuleKind   string
	RuleSource string
}

func (r Result) TargetID() string {
	if r.ID != "" {
		return r.ID
	}
	return r.Record.ID
}

func (r Result) DisplayTitle() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Record.Title
}

func (r Result) TypeLabel() string {
	switch r.Kind {
	case KindPrep:
		return "prep"
	case KindSession:
		return "session"
	case KindTranscript:
		return "note"
	case KindRecon:
		return "recon"
	case KindReference:
		return "reference"
	case KindRule:
		if r.RuleKind != "" {
			return r.RuleKind
		}
		return "rule"
	default:
		return string(r.Record.Type)
	}
}

func (r Result) Authority() domain.Authority {
	if r.Record.ID != "" {
		return r.Record.Authority
	}
	switch r.Kind {
	case KindPrep, KindRecon:
		return domain.Draft
	case KindRule:
		return domain.Reference
	default:
		return domain.Canon
	}
}

type Service struct {
	workspace domain.Workspace
	index     *Index
}

func New(records []domain.Record) Service {
	return FromWorkspace(domain.Workspace{Records: records})
}

func FromWorkspace(workspace domain.Workspace) Service {
	svc := FromDocuments(DocumentsFromWorkspace(workspace))
	svc.workspace = workspace
	return svc
}

func FromDocuments(docs []Document) Service {
	idx, err := Open("", docs)
	if err != nil {
		return Service{}
	}
	return Service{index: idx}
}

func FromIndex(idx *Index) Service {
	if idx == nil {
		return Service{}
	}
	return Service{index: idx}
}

func OpenPath(path string, docs []Document) (Service, error) {
	idx, err := Open(path, docs)
	if err != nil {
		return FromDocuments(docs), err
	}
	return Service{index: idx}, nil
}

func (s Service) Close() {
	s.index.Close()
}

func (s Service) Find(filter Filter) []Result {
	if s.index != nil {
		if results, err := s.index.Query(filter); err == nil {
			sortResults(results)
			return capReferences(results)
		}
	}
	results := s.findScan(filter)
	sortResults(results)
	return capReferences(results)
}

func sortResults(results []Result) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].DisplayTitle() < results[j].DisplayTitle()
		}
		return results[i].Score > results[j].Score
	})
}

func capReferences(results []Result) []Result {
	n := 0
	out := make([]Result, 0, len(results))
	for _, result := range results {
		if result.Kind == KindReference {
			if n >= maxReferenceHits {
				continue
			}
			n++
		}
		out = append(out, result)
	}
	return out
}

func (s Service) findScan(filter Filter) []Result {
	query := normalize(filter.Query)
	results := s.findRecords(filter, query)
	if includeExtras(filter) {
		results = append(results, s.findPrep(filter, query)...)
		results = append(results, s.findSessions(filter, query)...)
		results = append(results, s.findTranscripts(filter, query)...)
		results = append(results, s.findRecons(filter, query)...)
	}
	return results
}

func includeExtras(filter Filter) bool {
	return strings.TrimSpace(filter.Query) != "" && len(filter.Types) == 0 && len(filter.Tags) == 0
}

func (s Service) findRecords(filter Filter, query string) []Result {
	results := make([]Result, 0, len(s.workspace.Records))
	for _, record := range s.workspace.Records {
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
		results = append(results, Result{Kind: KindRecord, ID: record.ID, Record: record, Title: record.Title, Score: score})
	}
	return results
}

func (s Service) findPrep(filter Filter, query string) []Result {
	results := make([]Result, 0)
	for _, plan := range s.workspace.PlannedNotes {
		if !scopeVisible(plan.Scope, filter) {
			continue
		}
		score, ok := textScore(plan.Title, nil, plan.Body, query)
		if !ok {
			continue
		}
		results = append(results, Result{
			Kind:    KindPrep,
			ID:      plan.ID,
			Title:   plan.Title,
			Snippet: snippetAround(plan.Body, query),
			Score:   score,
		})
	}
	return results
}

func (s Service) findSessions(filter Filter, query string) []Result {
	results := make([]Result, 0)
	for _, session := range s.workspace.Sessions {
		if !scopeVisible(session.Scope, filter) {
			continue
		}
		score, ok := textScore(session.Title, nil, session.LocationName, query)
		if !ok {
			continue
		}
		results = append(results, Result{
			Kind:    KindSession,
			ID:      session.ID,
			Title:   session.Title,
			Snippet: session.LocationName,
			Score:   score,
		})
	}
	return results
}

func (s Service) findTranscripts(filter Filter, query string) []Result {
	results := make([]Result, 0)
	for _, session := range s.workspace.Sessions {
		if !scopeVisible(session.Scope, filter) {
			continue
		}
		for _, entry := range session.Entries {
			if entry.Undone || strings.TrimSpace(entry.Text) == "" {
				continue
			}
			score, ok := textScore("", nil, entry.Text, query)
			if !ok {
				continue
			}
			results = append(results, Result{
				Kind:      KindTranscript,
				ID:        entry.ID,
				SessionID: session.ID,
				Title:     session.Title + "  “" + truncate(entry.Text, 40) + "”",
				Snippet:   snippetAround(entry.Text, query),
				Score:     score,
			})
		}
	}
	return results
}

func (s Service) findRecons(filter Filter, query string) []Result {
	results := make([]Result, 0)
	for _, recon := range s.workspace.Reconciliations {
		session, ok := s.sessionByID(recon.SessionID)
		if !ok || !scopeVisible(session.Scope, filter) {
			continue
		}
		body := recon.Title
		for _, item := range recon.Items {
			body += " " + item.Summary
		}
		score, ok := textScore(recon.Title, nil, body, query)
		if !ok {
			continue
		}
		results = append(results, Result{
			Kind:      KindRecon,
			ID:        recon.ID,
			SessionID: recon.SessionID,
			Title:     recon.Title,
			Snippet:   snippetAround(body, query),
			Score:     score,
		})
	}
	return results
}

func (s Service) sessionByID(id string) (domain.SessionRecord, bool) {
	for _, session := range s.workspace.Sessions {
		if session.ID == id {
			return session, true
		}
	}
	return domain.SessionRecord{}, false
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

func scopeVisible(scope domain.Scope, filter Filter) bool {
	return inScope(domain.Record{Scope: scope}, filter)
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
	return textScore(record.Title, record.Aliases, strings.Join([]string{
		record.Summary,
		record.Body,
		strings.Join(record.Tags, " "),
		record.Source,
	}, " "), query)
}

func textScore(title string, aliases []string, body, query string) (int, bool) {
	if query == "" {
		return 1, true
	}

	normalizedTitle := normalize(title)
	if normalizedTitle == query {
		return 1000, true
	}
	if strings.HasPrefix(normalizedTitle, query) {
		return 800 - len(normalizedTitle), true
	}
	if strings.Contains(normalizedTitle, query) {
		return 600 - strings.Index(normalizedTitle, query), true
	}

	for _, alias := range aliases {
		if strings.Contains(normalize(alias), query) {
			return 500, true
		}
	}

	if score, ok := fuzzyScore(normalizedTitle, query); ok {
		return 400 + score, true
	}

	searchable := normalize(body)
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

func snippetAround(text, query string) string {
	normalized := normalize(text)
	index := strings.Index(normalized, query)
	if index < 0 {
		return truncate(strings.TrimSpace(text), 72)
	}
	runes := []rune(strings.TrimSpace(text))
	start := max(0, index-24)
	end := min(len(runes), index+len([]rune(query))+48)
	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return snippet
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
