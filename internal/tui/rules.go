package tui

import (
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	"github.com/hbaldwin98/dungeon/internal/ruleset"
	"github.com/hbaldwin98/dungeon/internal/ruleset/dnd5e"
)

func (m *Model) attachRules() {
	m.rules = ruleset.New()
	plugin, err := dnd5e.Open(m.rulesFetcher(), dnd5e.Options{Books: fiveEBooks(m.workspace)})
	if err != nil || plugin == nil {
		return
	}
	m.rules.Register(plugin)
}

func (m Model) rulesFetcher() fivetools.Fetcher {
	if m.toolsFetcher != nil {
		return m.toolsFetcher
	}
	return fivetools.CacheOnly(fivetools.DefaultCacheDir())
}

func fiveEBooks(ws domain.Workspace) []string {
	enabled := ws.EnabledSourceIDs(ws.Scope)
	allow := map[string]bool{}
	for _, id := range enabled {
		allow[id] = true
	}
	var books []string
	for _, doc := range ws.Sources {
		if !strings.HasPrefix(doc.ID, "src-5e-") {
			continue
		}
		if !allow[doc.ID] {
			continue
		}
		books = append(books, fivetools.SourceCode(doc.ID))
	}
	return books
}

func (m Model) lookupRuleset(name string) (domain.Record, bool) {
	if m.rules == nil {
		return domain.Record{}, false
	}
	ent, ok := m.rules.LookupName(name)
	if !ok {
		return domain.Record{}, false
	}
	return ent.Record, true
}

func (m Model) lookupAny(id string) (domain.Record, bool) {
	if rec, ok := recordByID(m.workspace.Records, id); ok {
		return rec, true
	}
	if m.rules != nil {
		return m.rules.Record(id)
	}
	return domain.Record{}, false
}

func (m Model) bindRulesetMentions(mentions []domain.Mention) []domain.Mention {
	if m.rules == nil || len(mentions) == 0 {
		return mentions
	}
	out := append([]domain.Mention(nil), mentions...)
	for i, mention := range out {
		if mention.RecordID != "" {
			continue
		}
		if rec, ok := m.lookupRuleset(mention.Text); ok {
			out[i].RecordID = rec.ID
		}
	}
	return out
}

func (m Model) resolveMentions(text string) []domain.Mention {
	return m.bindRulesetMentions(domain.MentionsIn(text, m.workspace.Records))
}

func (m Model) pluginSuggestions(query string, limit int) []Suggestion {
	if m.rules == nil || limit < 1 {
		return nil
	}
	out := make([]Suggestion, 0, limit)
	for _, ent := range m.rules.Search(query, limit) {
		rec := ent.Record
		label := string(rec.Type) + "  " + rec.Title
		if rec.Source != "" {
			label += "  (" + rec.Source + ")"
		}
		out = append(out, Suggestion{
			Label:  label,
			Insert: "@" + rec.Title + " ",
			Record: &rec,
		})
	}
	return out
}
