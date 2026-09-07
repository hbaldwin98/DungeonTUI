package dnd5e

import (
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
	"github.com/hbaldwin98/dungeon/internal/ruleset"
)

const ID = "dnd5e"

// Plugin is the D&D Next / 5e ruleset. It reads composed 5e.tools JSON
// (bestiary, templates, legendary groups, items, magic variants) from the
// local persisted cache and returns markdown for display. Campaign wiki
// records stay separate.
type Plugin struct {
	catalog *fivetools.Catalog
}

type Options struct {
	Books []string
}

func Open(fetcher fivetools.Fetcher, opts Options) (*Plugin, error) {
	if fetcher == nil {
		return nil, nil
	}
	catalog := fivetools.NewCatalog(fetcher)
	if err := catalog.LoadCompose(); err != nil {
		return nil, err
	}
	_ = catalog.LoadItems()
	seen := map[string]bool{}
	for _, book := range opts.Books {
		book = strings.TrimSpace(book)
		if book == "" {
			continue
		}
		key := strings.ToLower(book)
		if seen[key] {
			continue
		}
		seen[key] = true
		_ = catalog.LoadBestiary(book)
		if core := fivetools.CoreBestiaryCode(book); core != "" && !seen[strings.ToLower(core)] {
			seen[strings.ToLower(core)] = true
			_ = catalog.LoadBestiary(core)
		}
	}
	catalog.SetPreferred(append(append([]string{}, opts.Books...), "MM", "XMM"))
	return &Plugin{catalog: catalog}, nil
}

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "D&D 5e" }

func (p *Plugin) Lookup(kind ruleset.Kind, name, source string) (ruleset.Entity, bool) {
	if p == nil || p.catalog == nil {
		return ruleset.Entity{}, false
	}
	hit, ok := p.catalog.Lookup(fivetools.Kind(kind), name, source)
	if !ok {
		return ruleset.Entity{}, false
	}
	return entity(hit), true
}

func (p *Plugin) LookupName(name string) (ruleset.Entity, bool) {
	if p == nil || p.catalog == nil {
		return ruleset.Entity{}, false
	}
	hit, ok := p.catalog.LookupName(name)
	if !ok {
		return ruleset.Entity{}, false
	}
	return entity(hit), true
}

func (p *Plugin) Search(query string, limit int) []ruleset.Entity {
	if p == nil || p.catalog == nil {
		return nil
	}
	hits := p.catalog.Search(query, limit)
	out := make([]ruleset.Entity, 0, len(hits))
	for _, hit := range hits {
		out = append(out, entity(hit))
	}
	return out
}

func (p *Plugin) Record(id string) (domain.Record, bool) {
	if p == nil || p.catalog == nil || !fivetools.IsPluginID(id) {
		return domain.Record{}, false
	}
	kind, source, name, ok := parseRecordID(id)
	if !ok {
		return domain.Record{}, false
	}
	if hit, ok := p.catalog.Lookup(kind, name, source); ok && hit.ID() == id {
		return hit.Record(), true
	}
	if hit, ok := p.catalog.LookupName(name); ok && hit.ID() == id {
		return hit.Record(), true
	}
	return domain.Record{}, false
}

func parseRecordID(id string) (kind fivetools.Kind, source, name string, ok bool) {
	rest, found := strings.CutPrefix(id, fivetools.PluginIDPrefix)
	if !found {
		return "", "", "", false
	}
	kindStr, rest, found := strings.Cut(rest, ":")
	if !found {
		return "", "", "", false
	}
	sourceSlug, nameSlug, found := strings.Cut(rest, ":")
	if !found || nameSlug == "" {
		return "", "", "", false
	}
	return fivetools.Kind(kindStr), unslug(sourceSlug), unslug(nameSlug), true
}

func unslug(s string) string {
	return strings.ReplaceAll(s, "-", " ")
}

func entity(hit fivetools.Hit) ruleset.Entity {
	rec := hit.Record()
	return ruleset.Entity{
		PluginID: ID,
		Kind:     ruleset.Kind(hit.Kind),
		Name:     hit.Name,
		Source:   hit.Source,
		Summary:  hit.Summary,
		Body:     hit.Body,
		Record:   rec,
	}
}
