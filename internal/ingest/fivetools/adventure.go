package fivetools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

const ReferenceIDPrefix = "ref:"

// AdventureBook is a cached 5e.tools adventure for the reader and @ peeks.
// It is not wiki canon.
type AdventureBook struct {
	ID    string
	Title string
	TOC   []string
	Body  string
	Hits  []AdventureHit
}

// AdventureHit is one unique document inside a book. Extra names that share
// this body are Aliases (creature tags, table cells, reprint headings).
type AdventureHit struct {
	Name    string
	Chapter string
	Heading string
	Summary string
	Body    string
	Aliases []string
}

func AdventureJSONPath(id string) string {
	return "data/adventure/adventure-" + strings.ToLower(strings.TrimSpace(id)) + ".json"
}

func IsReferenceID(id string) bool {
	return strings.HasPrefix(id, ReferenceIDPrefix)
}

func ReferenceRecordID(sourceID, name string) string {
	return ReferenceIDPrefix + sourceID + ":" + slug(name)
}

// ParseAdventure turns 5e.tools adventure JSON into a readable book and name index.
func ParseAdventure(data []byte) (AdventureBook, error) {
	var raw struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return AdventureBook{}, fmt.Errorf("adventure json: %w", err)
	}
	var toc []string
	var body strings.Builder
	var hits []AdventureHit
	nameIndex := map[string]int{}
	bodyIndex := map[string]int{}
	addHit := func(name, chapter, heading, text string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := nameKey(name)
		text = strings.TrimSpace(text)
		doc := documentKey(text)
		chapter = firstNonEmpty(strings.TrimSpace(chapter), name)
		heading = firstNonEmpty(strings.TrimSpace(heading), chapter)

		if idx, ok := nameIndex[key]; ok {
			existing := hits[idx]
			if doc == "" || documentKey(existing.Body) == doc {
				return
			}
			if !strings.EqualFold(existing.Name, name) {
				removeAlias(&hits[idx], name)
				delete(nameIndex, key)
			} else {
				return
			}
		}

		if doc != "" {
			if idx, ok := bodyIndex[doc]; ok {
				appendAlias(&hits[idx], name)
				nameIndex[key] = idx
				return
			}
		}

		hits = append(hits, AdventureHit{
			Name:    name,
			Chapter: chapter,
			Heading: heading,
			Summary: summaryOf(text),
			Body:    text,
		})
		idx := len(hits) - 1
		nameIndex[key] = idx
		if doc != "" {
			bodyIndex[doc] = idx
		}
	}
	for _, item := range raw.Data {
		block := asMap(item)
		if block == nil {
			continue
		}
		name := asString(block["name"])
		rendered := strings.TrimSpace(renderValue(block["entries"]))
		if name != "" {
			toc = append(toc, name)
			body.WriteString("## ")
			body.WriteString(name)
			body.WriteString("\n\n")
			addHit(name, name, name, rendered)
		}
		if rendered != "" {
			body.WriteString(rendered)
			body.WriteString("\n\n")
		}
		indexAdventureNames(block["entries"], name, name, rendered, addHit)
	}
	return AdventureBook{
		TOC:  toc,
		Body: strings.TrimSpace(body.String()) + "\n",
		Hits: hits,
	}, nil
}

func LoadAdventure(fetcher Fetcher, code, title string) (AdventureBook, error) {
	if fetcher == nil {
		return AdventureBook{}, fmt.Errorf("fetcher is required")
	}
	data, err := fetcher.Get(AdventureJSONPath(code))
	if err != nil {
		return AdventureBook{}, err
	}
	book, err := ParseAdventure(data)
	if err != nil {
		return AdventureBook{}, err
	}
	book.ID = code
	book.Title = firstNonEmpty(title, code)
	return book, nil
}

func (b AdventureBook) Lookup(name string) (AdventureHit, bool) {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return AdventureHit{}, false
	}
	var fallback AdventureHit
	fallbackOK := false
	for _, hit := range b.Hits {
		if hit.hasName(name) {
			return hit, true
		}
		lower := strings.ToLower(hit.Name)
		if !fallbackOK && (strings.HasPrefix(lower, needle) || strings.HasPrefix(needle, lower)) {
			fallback = hit
			fallbackOK = true
			continue
		}
		for _, alias := range hit.Aliases {
			al := strings.ToLower(alias)
			if !fallbackOK && (strings.HasPrefix(al, needle) || strings.HasPrefix(needle, al)) {
				fallback = hit
				fallbackOK = true
				break
			}
		}
	}
	return fallback, fallbackOK
}

func (b AdventureBook) Search(query string, limit int) []AdventureHit {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || limit < 1 {
		return nil
	}
	out := make([]AdventureHit, 0, limit)
	for _, hit := range b.Hits {
		if len(out) >= limit {
			break
		}
		blob := strings.ToLower(hit.Name + " " + strings.Join(hit.Aliases, " ") + " " + hit.Heading + " " + hit.Summary + " " + hit.Body)
		if strings.Contains(blob, query) {
			out = append(out, hit)
		}
	}
	return out
}

func (b AdventureBook) Record(hit AdventureHit, sourceID string) domain.Record {
	body := strings.TrimSpace(hit.Body)
	if body == "" {
		body = hit.Name
	}
	aliases := append([]string{hit.Name}, hit.Aliases...)
	return domain.Record{
		ID:        ReferenceRecordID(sourceID, hit.Name),
		Type:      domain.Note,
		Title:     hit.Name,
		Summary:   firstNonEmpty(hit.Summary, "Reference from "+b.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Source:    b.Title,
		SourceID:  sourceID,
		Aliases:   aliases,
		Tags:      []string{"reference", "adventure"},
	}
}

func (h AdventureHit) hasName(name string) bool {
	key := nameKey(name)
	if key == "" {
		return false
	}
	if nameKey(h.Name) == key {
		return true
	}
	for _, alias := range h.Aliases {
		if nameKey(alias) == key {
			return true
		}
	}
	return false
}

func nameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func documentKey(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func appendAlias(hit *AdventureHit, name string) {
	if hit == nil || hit.hasName(name) {
		return
	}
	hit.Aliases = append(hit.Aliases, name)
}

func removeAlias(hit *AdventureHit, name string) {
	if hit == nil {
		return
	}
	key := nameKey(name)
	out := hit.Aliases[:0]
	for _, alias := range hit.Aliases {
		if nameKey(alias) != key {
			out = append(out, alias)
		}
	}
	hit.Aliases = out
}

func indexAdventureNames(v any, chapter, heading, fallback string, add func(name, chapter, heading, text string)) {
	switch t := v.(type) {
	case string:
		for _, m := range tagPattern.FindAllStringSubmatch(t, -1) {
			if len(m) < 3 || !strings.EqualFold(m[1], "creature") {
				continue
			}
			parts := splitTagArg(m[2])
			name := ""
			display := ""
			if len(parts) > 0 {
				name = parts[0]
			}
			if len(parts) >= 3 {
				display = parts[2]
			}
			label := firstNonEmpty(display, name)
			add(titleCaseName(label), chapter, heading, fallback)
		}
	case []any:
		for _, item := range t {
			indexAdventureNames(item, chapter, heading, fallback, add)
		}
	case map[string]any:
		name := asString(t["name"])
		childHeading := heading
		rendered := strings.TrimSpace(renderValue(t["entries"]))
		if rendered == "" {
			rendered = fallback
		}
		if name != "" {
			childHeading = name
			add(name, chapter, heading, rendered)
		}
		if asString(t["type"]) == "table" {
			indexTableNames(t, chapter, childHeading, rendered, add)
		}
		indexAdventureNames(t["entries"], chapter, childHeading, rendered, add)
		indexAdventureNames(t["items"], chapter, childHeading, rendered, add)
		indexAdventureNames(t["rows"], chapter, childHeading, rendered, add)
	}
}

func indexTableNames(table map[string]any, chapter, heading, text string, add func(name, chapter, heading, text string)) {
	for _, row := range asList(table["rows"]) {
		cells, ok := row.([]any)
		if !ok {
			continue
		}
		if len(cells) == 0 {
			continue
		}
		label := strings.TrimSpace(renderTags(asString(cells[0])))
		label = strings.TrimPrefix(label, "@")
		if label == "" || strings.Contains(label, ".") {
			continue
		}
		add(titleCaseName(label), chapter, heading, text)
	}
}
