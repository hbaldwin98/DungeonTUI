package ruleset

import "github.com/hbaldwin98/dungeon/internal/domain"

// Kind is a mechanical entity the plugin can display.
type Kind string

const (
	Creature Kind = "creature"
	Item     Kind = "item"
)

// Entity is composed source data ready for the TUI. It is not a wiki row and
// must not be written to Workspace.Records. The plugin corpus itself is
// persisted on the local machine (fetched JSON now; SQLite FTS5 later).
type Entity struct {
	PluginID string
	Kind     Kind
	Name     string
	Source   string
	Summary  string
	Body     string
	Record   domain.Record
}

// Plugin looks up mechanical data for one game system (D&D 5e, later others).
type Plugin interface {
	ID() string
	Name() string
	Lookup(kind Kind, name, source string) (Entity, bool)
	LookupName(name string) (Entity, bool)
	Search(query string, limit int) []Entity
	Record(id string) (domain.Record, bool)
}

// Registry queries plugins in registration order. Wiki records always win
// when the TUI looks them up first.
type Registry struct {
	plugins []Plugin
}

func New() *Registry {
	return &Registry{}
}

func (r *Registry) Register(plugin Plugin) {
	if r == nil || plugin == nil {
		return
	}
	id := plugin.ID()
	for i, existing := range r.plugins {
		if existing.ID() == id {
			r.plugins[i] = plugin
			return
		}
	}
	r.plugins = append(r.plugins, plugin)
}

func (r *Registry) Lookup(kind Kind, name, source string) (Entity, bool) {
	if r == nil {
		return Entity{}, false
	}
	for _, plugin := range r.plugins {
		if ent, ok := plugin.Lookup(kind, name, source); ok {
			return ent, true
		}
	}
	return Entity{}, false
}

func (r *Registry) LookupName(name string) (Entity, bool) {
	if r == nil {
		return Entity{}, false
	}
	for _, plugin := range r.plugins {
		if ent, ok := plugin.LookupName(name); ok {
			return ent, true
		}
	}
	return Entity{}, false
}

func (r *Registry) Search(query string, limit int) []Entity {
	if r == nil || limit < 1 {
		return nil
	}
	out := make([]Entity, 0, limit)
	for _, plugin := range r.plugins {
		for _, ent := range plugin.Search(query, limit-len(out)) {
			out = append(out, ent)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func (r *Registry) Record(id string) (domain.Record, bool) {
	if r == nil || id == "" {
		return domain.Record{}, false
	}
	for _, plugin := range r.plugins {
		if rec, ok := plugin.Record(id); ok {
			return rec, true
		}
	}
	return domain.Record{}, false
}
