package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/fivecli"
	searchsvc "github.com/hbaldwin98/DungeonTUI/internal/search"
)

// rulesSearchDebounce delays a keystroke's lookup so typing a word spawns one
// subprocess instead of one per character.
const rulesSearchDebounce = 180 * time.Millisecond

// rulesSearchLimit bounds how many hits the overlay asks 5e-cli for.
const rulesSearchLimit = 12

// rulesDebounceMsg fires when typing has paused long enough to search.
type rulesDebounceMsg struct {
	seq   uint64
	query string
}

type rulesSearchMsg struct {
	seq   uint64
	query string
	hits  []searchsvc.Result
	err   error
}

// rulesStatusMsg carries a `5e doctor` diagnosis back to the update loop.
// The diagnosis is a subprocess, so it is never run inline: a wedged or
// missing tool must not freeze capture.
type rulesStatusMsg struct {
	binary  string
	ready   bool
	summary string
}

// rulesStatus caches the last diagnosis and the binary it describes, so
// changing the configured path invalidates it without an explicit reset.
type rulesStatus struct {
	loaded  bool
	binary  string
	ready   bool
	summary string
}

type ruleEntityMsg struct {
	hop  detailHop
	text string
	err  error
}

// fivecliAdapter builds the adapter from the configured binary. Empty means
// the adapter falls back to DUNGEON_5E_BIN and then PATH.
func (m Model) fivecliAdapter() fivecli.Adapter {
	return fivecli.Adapter{Binary: strings.TrimSpace(m.layout.FiveCLIBinary)}
}

// fivecliBinaryKey names the configured binary for cache comparison.
func (m Model) fivecliBinaryKey() string {
	return strings.TrimSpace(m.layout.FiveCLIBinary)
}

func resultsFromRuleHits(hits []fivecli.Hit) []searchsvc.Result {
	results := make([]searchsvc.Result, 0, len(hits))
	for index, hit := range hits {
		score := int(hit.Score)
		if score == 0 {
			score = len(hits) - index
		}
		results = append(results, searchsvc.Result{
			Kind:       searchsvc.KindRule,
			Title:      hit.Name,
			Snippet:    hit.Snippet,
			Score:      score,
			RuleKind:   hit.Kind,
			RuleSource: hit.Source,
		})
	}
	return results
}

// rulesStatusCmd diagnoses the configured 5e-cli off the update loop.
func (m Model) rulesStatusCmd() tea.Cmd {
	adapter := m.fivecliAdapter()
	binary := m.fivecliBinaryKey()
	return func() tea.Msg {
		ready, summary := adapter.LookupStatus(context.Background())
		return rulesStatusMsg{binary: binary, ready: ready, summary: summary}
	}
}

// ensureRulesStatus schedules a diagnosis only when none is cached for the
// currently configured binary.
func (m Model) ensureRulesStatus() tea.Cmd {
	if m.rulesStatus.loaded && m.rulesStatus.binary == m.fivecliBinaryKey() {
		return nil
	}
	return m.rulesStatusCmd()
}

func (m Model) handleRulesStatus(msg rulesStatusMsg) (tea.Model, tea.Cmd) {
	if msg.binary != m.fivecliBinaryKey() {
		return m, nil
	}
	m.rulesStatus = rulesStatus{loaded: true, binary: msg.binary, ready: msg.ready, summary: msg.summary}
	if m.searching && m.searchScope == searchsvc.RulesReference && len(m.results) == 0 {
		m.rulesNote = m.rulesSetupHint()
	}
	return m, nil
}

// rulesSetupHint reads the cached diagnosis; it never runs a subprocess.
func (m Model) rulesSetupHint() string {
	switch {
	case !m.rulesStatus.loaded || m.rulesStatus.binary != m.fivecliBinaryKey():
		return "Checking 5e-cli…"
	case m.rulesStatus.ready:
		return "Type to search spells, monsters, rules text, and tables."
	default:
		return m.rulesStatus.summary + " · Ctrl+G sets the 5e path"
	}
}

// scheduleRulesSearch clears stale hits and waits out the debounce before
// spending a subprocess on the current query.
func (m *Model) scheduleRulesSearch() tea.Cmd {
	query := strings.TrimSpace(m.searchInput.Value())
	m.results = nil
	if query == "" {
		m.rulesQuerySeq++
		m.rulesNote = m.rulesSetupHint()
		return m.ensureRulesStatus()
	}
	m.rulesQuerySeq++
	seq := m.rulesQuerySeq
	m.rulesNote = "Searching the 5e index…"
	cmds := []tea.Cmd{tea.Tick(rulesSearchDebounce, func(time.Time) tea.Msg {
		return rulesDebounceMsg{seq: seq, query: query}
	})}
	if status := m.ensureRulesStatus(); status != nil {
		cmds = append(cmds, status)
	}
	return tea.Batch(cmds...)
}

func (m Model) handleRulesDebounce(msg rulesDebounceMsg) (tea.Model, tea.Cmd) {
	if !m.searching || m.searchScope != searchsvc.RulesReference {
		return m, nil
	}
	if msg.seq != m.rulesQuerySeq {
		return m, nil
	}
	if strings.TrimSpace(m.searchInput.Value()) != msg.query {
		return m, nil
	}
	return m, m.runRulesSearch(msg.query)
}

// runRulesSearch queries 5e-cli for one settled query.
func (m *Model) runRulesSearch(query string) tea.Cmd {
	m.rulesSearchSeq++
	seq := m.rulesSearchSeq
	adapter := m.fivecliAdapter()
	return func() tea.Msg {
		hits, err := adapter.Search(context.Background(), query, rulesSearchLimit)
		return rulesSearchMsg{seq: seq, query: query, hits: resultsFromRuleHits(hits), err: err}
	}
}

func (m Model) handleRulesSearch(msg rulesSearchMsg) (tea.Model, tea.Cmd) {
	if !m.searching || m.searchScope != searchsvc.RulesReference {
		return m, nil
	}
	if msg.seq != m.rulesSearchSeq {
		return m, nil
	}
	if strings.TrimSpace(m.searchInput.Value()) != msg.query {
		return m, nil
	}
	if msg.err != nil {
		m.results = nil
		if errors.Is(msg.err, fivecli.ErrUnavailable) {
			// The tool moved or was removed since the cached diagnosis.
			m.rulesStatus = rulesStatus{}
			m.rulesNote = m.rulesSetupHint()
			return m, m.rulesStatusCmd()
		}
		m.rulesNote = "5e search failed: " + msg.err.Error()
		return m, nil
	}
	m.results = msg.hits
	m.rulesNote = ""
	if len(m.results) == 0 {
		m.rulesNote = "No matching rules in the 5e index."
	}
	if m.selected >= len(m.results) {
		m.selected = max(0, len(m.results)-1)
	}
	return m, nil
}

func (m Model) fetchRulePreviewCmd(hop detailHop) tea.Cmd {
	adapter := m.fivecliAdapter()
	return func() tea.Msg {
		entity, err := adapter.Get(context.Background(), hop.RuleKind, hop.Label, hop.RuleSource)
		if err != nil {
			return ruleEntityMsg{hop: hop, err: err}
		}
		text := strings.TrimSpace(entity.Text)
		if entity.Source != "" {
			header := strings.ToUpper(entity.Kind) + " · " + entity.Name + " (" + entity.Source
			if page := strings.TrimSpace(entity.Page.String()); page != "" {
				header += " p." + page
			}
			header += ")\n\n"
			text = header + text
		}
		return ruleEntityMsg{hop: hop, text: text}
	}
}

func (m Model) handleRuleEntity(msg ruleEntityMsg) (tea.Model, tea.Cmd) {
	if m.preview == nil || m.preview.Hop.Kind != hopRule {
		return m, nil
	}
	if msg.hop.RuleKind != m.preview.Hop.RuleKind ||
		msg.hop.Label != m.preview.Hop.Label ||
		msg.hop.RuleSource != m.preview.Hop.RuleSource {
		return m, nil
	}
	m.preview.bodyValid = false
	m.preview.frameValid = false
	if msg.err != nil {
		var amb *fivecli.AmbiguousError
		if errors.As(msg.err, &amb) {
			m.preview.ruleText = formatAmbiguousRule(amb)
		} else {
			m.preview.ruleText = "Could not load rule: " + msg.err.Error()
		}
		return m, nil
	}
	m.preview.ruleText = msg.text
	return m, nil
}

// formatAmbiguousRule explains a name that several sourcebooks reprint. A hit
// picked from search already carries its source, so this is the fallback for a
// hit that reported none.
func formatAmbiguousRule(amb *fivecli.AmbiguousError) string {
	var b strings.Builder
	b.WriteString("This name appears in several sources. Search again and pick the printing you want:\n\n")
	for _, match := range amb.Matches {
		fmt.Fprintf(&b, "  %s  %s  (%s)\n", match.Kind, match.Name, match.Source)
	}
	return b.String()
}

func (m *Model) openRulePreview(hop detailHop) {
	m.openRulePreviewFrom(hop, m.snapshotSearchLocation())
}

func (m *Model) openRulePreviewFrom(hop detailHop, origin searchLocation) {
	m.preview = &previewBuf{
		Hop:      hop,
		ruleText: "Loading 5e reference…",
		origin:   origin,
		parent:   m.preview,
	}
	if m.session != nil {
		m.status = "5e reference · Enter returns to capture · Esc closes"
	} else {
		m.status = "5e reference · not campaign canon · Esc closes"
	}
}

func (m Model) openRuleSearchResult(result searchsvc.Result) (tea.Model, tea.Cmd) {
	hop := detailHop{
		Kind:       hopRule,
		Label:      result.Title,
		RuleKind:   result.RuleKind,
		RuleSource: result.RuleSource,
		Prefix:     strings.TrimSpace(result.RuleKind + " · " + result.RuleSource + " · "),
		Relation:   "5e reference · not campaign canon",
	}
	origin := m.snapshotSearchLocation()
	m.searching = false
	m.searchInput.Blur()
	m.rulesNote = ""
	m.openRulePreviewFrom(hop, origin)
	return m, m.fetchRulePreviewCmd(hop)
}

// searchResultContextFor adds the printing and snippet to a rules hit; every
// other kind keeps the campaign context line.
func (m Model) searchResultContextFor(result searchsvc.Result) string {
	if result.Kind != searchsvc.KindRule {
		return m.searchResultContext(result)
	}
	parts := make([]string, 0, 2)
	if source := strings.TrimSpace(result.RuleSource); source != "" {
		parts = append(parts, source)
	}
	if snippet := strings.TrimSpace(result.Snippet); snippet != "" {
		parts = append(parts, snippet)
	}
	return strings.Join(parts, " · ")
}
