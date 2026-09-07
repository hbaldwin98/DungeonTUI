package fivetools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

// Entry is one ingestible 5e.tools book or adventure.
type Entry struct {
	ID        string
	Name      string
	Group     string
	Published string
	Kind      string // "adventure" or "book"
	Storyline string
}

func (e Entry) SourceKind() domain.SourceKind {
	if e.Kind == "adventure" {
		return domain.SourceAdventure
	}
	lower := strings.ToLower(e.Name + " " + e.ID)
	if strings.Contains(lower, "monster manual") || strings.Contains(lower, "bestiary") ||
		e.ID == "MM" || e.ID == "XMM" || e.ID == "MPMM" || e.ID == "VGM" || e.ID == "MTF" {
		return domain.SourceBestiary
	}
	return domain.SourceRules
}

func (e Entry) Edition() string {
	if year := yearFrom(e.Published); year != "" {
		return year
	}
	if len(e.Published) >= 4 {
		return e.Published[:4]
	}
	return ""
}

func (e Entry) PageURL() string {
	id := strings.ToLower(e.ID)
	if e.Kind == "adventure" {
		return DefaultBaseURL + "/adventure.html#" + id + ",-1"
	}
	return DefaultBaseURL + "/book.html#" + id + ",-1"
}

// LoadCatalog lists books and adventures the owner can choose to ingest.
func LoadCatalog(fetcher Fetcher) ([]Entry, error) {
	var entries []Entry
	seen := map[string]bool{}
	add := func(list []Entry) {
		for _, entry := range list {
			key := strings.ToLower(entry.ID)
			if entry.ID == "" || seen[key] {
				continue
			}
			seen[key] = true
			entries = append(entries, entry)
		}
	}

	if data, ok, err := getOptional(fetcher, "data/adventures.json"); err != nil {
		return nil, err
	} else if ok {
		list, err := parseNamedList(data, "adventure", "adventure")
		if err != nil {
			return nil, fmt.Errorf("adventures: %w", err)
		}
		add(list)
	}

	if data, ok, err := getOptional(fetcher, "data/books.json"); err != nil {
		return nil, err
	} else if ok {
		list, err := parseNamedList(data, "book", "book")
		if err != nil {
			return nil, fmt.Errorf("books: %w", err)
		}
		add(list)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("5e.tools catalog was empty")
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func parseNamedList(data []byte, key, kind string) ([]Entry, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	raw, ok := envelope[key]
	if !ok {
		return nil, fmt.Errorf("missing %s array", key)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for _, row := range rows {
		id := asString(row["id"])
		if id == "" {
			id = asString(row["source"])
		}
		name := asString(row["name"])
		if id == "" || name == "" {
			continue
		}
		out = append(out, Entry{
			ID:        id,
			Name:      name,
			Group:     asString(row["group"]),
			Published: asString(row["published"]),
			Kind:      kind,
			Storyline: asString(row["storyline"]),
		})
	}
	return out, nil
}

func ResolveEntry(catalog []Entry, ref Ref) (Entry, bool) {
	want := strings.ToLower(ref.ID)
	for _, entry := range catalog {
		if strings.ToLower(entry.ID) == want {
			if ref.Kind != "" && entry.Kind != ref.Kind {
				continue
			}
			return entry, true
		}
	}
	if ref.Kind != "" {
		for _, entry := range catalog {
			if strings.ToLower(entry.ID) == want {
				return entry, true
			}
		}
	}
	return Entry{}, false
}

func FilterCatalog(entries []Entry, query string) []Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return entries
	}
	out := make([]Entry, 0)
	for _, entry := range entries {
		blob := strings.ToLower(entry.ID + " " + entry.Name + " " + entry.Group + " " + entry.Storyline)
		if strings.Contains(blob, query) {
			out = append(out, entry)
		}
	}
	return out
}

func SourceID(code string) string {
	return "src-5e-" + slug(code)
}

func yearFrom(published string) string {
	if t, err := time.Parse("2006-01-02", published); err == nil {
		return t.Format("2006")
	}
	return ""
}
