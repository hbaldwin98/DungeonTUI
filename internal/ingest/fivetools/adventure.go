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

// AdventureHit is one named heading or creature mention inside a book.
type AdventureHit struct {
	Name    string
	Heading string
	Summary string
	Body    string
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
	seen := map[string]bool{}
	addHit := func(name, heading, text string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		text = strings.TrimSpace(text)
		hits = append(hits, AdventureHit{
			Name:    name,
			Heading: heading,
			Summary: summaryOf(text),
			Body:    text,
		})
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
			addHit(name, name, rendered)
		}
		if rendered != "" {
			body.WriteString(rendered)
			body.WriteString("\n\n")
		}
		indexAdventureNames(block["entries"], name, rendered, addHit)
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
		lower := strings.ToLower(hit.Name)
		if lower == needle {
			return hit, true
		}
		if !fallbackOK && (strings.HasPrefix(lower, needle) || strings.HasPrefix(needle, lower)) {
			fallback = hit
			fallbackOK = true
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
		blob := strings.ToLower(hit.Name + " " + hit.Heading + " " + hit.Summary + " " + hit.Body)
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
	return domain.Record{
		ID:        ReferenceRecordID(sourceID, hit.Name),
		Type:      domain.Note,
		Title:     hit.Name,
		Summary:   firstNonEmpty(hit.Summary, "Reference from "+b.Title+"."),
		Body:      body,
		Authority: domain.Canon,
		Source:    b.Title,
		SourceID:  sourceID,
		Aliases:   []string{hit.Name},
		Tags:      []string{"reference", "adventure"},
	}
}

func indexAdventureNames(v any, heading, fallback string, add func(name, heading, text string)) {
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
			add(titleCaseName(label), heading, fallback)
		}
	case []any:
		for _, item := range t {
			indexAdventureNames(item, heading, fallback, add)
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
			add(name, heading, rendered)
		}
		if asString(t["type"]) == "table" {
			indexTableNames(t, childHeading, rendered, add)
		}
		indexAdventureNames(t["entries"], childHeading, rendered, add)
		indexAdventureNames(t["items"], childHeading, rendered, add)
		indexAdventureNames(t["rows"], childHeading, rendered, add)
	}
}

func indexTableNames(table map[string]any, heading, text string, add func(name, heading, text string)) {
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
		add(titleCaseName(label), heading, text)
	}
}
