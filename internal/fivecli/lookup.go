package fivecli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Hit is one ranked result from `5e search --json`.
type Hit struct {
	Kind    string  `json:"kind"`
	Name    string  `json:"name"`
	Source  string  `json:"source"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet,omitempty"`
}

// Entity is the decoded body of `5e get --json`.
type Entity struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Page   Page   `json:"page"`
	Text   string `json:"text"`
}

// Page is a printed page reference. 5e-cli emits it as a JSON number for most
// entities, but 5etools sources also carry non-numeric pages, so both decode
// into the string the UI prints. A missing or null page decodes to empty.
type Page string

func (p *Page) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	switch {
	case text == "" || text == "null":
		*p = ""
		return nil
	case strings.HasPrefix(text, `"`):
		var quoted string
		if err := json.Unmarshal(data, &quoted); err != nil {
			return err
		}
		*p = Page(strings.TrimSpace(quoted))
		return nil
	default:
		var number json.Number
		if err := json.Unmarshal(data, &number); err != nil {
			return fmt.Errorf("page: %w", err)
		}
		*p = Page(number.String())
		return nil
	}
}

// String renders the page for display.
func (p Page) String() string { return string(p) }

// AmbiguousError reports several source-specific matches for one name.
type AmbiguousError struct {
	Matches []Hit
}

func (e *AmbiguousError) Error() string {
	if len(e.Matches) == 0 {
		return "ambiguous match"
	}
	first := e.Matches[0]
	return fmt.Sprintf("ambiguous %s %q (%d matches; pass source %s)",
		first.Kind, first.Name, len(e.Matches), first.Source)
}

// Search runs `5e search --json`. An empty query returns nil without invoking
// the tool.
func (a Adapter) Search(ctx context.Context, query string, limit int) ([]Hit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	var hits []Hit
	if err := a.decodeJSON(ctx, &hits, "search", query, "--limit", strconv.Itoa(limit)); err != nil {
		return nil, err
	}
	return hits, nil
}

// Get runs `5e get --json`. Source disambiguates reprints when several match.
func (a Adapter) Get(ctx context.Context, kind, name, source string) (Entity, error) {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" {
		return Entity{}, fmt.Errorf("kind and name are required")
	}
	args := []string{"get", kind, name}
	if source = strings.TrimSpace(source); source != "" {
		args = append(args, "--source", source)
	}
	body, err := a.decodeRaw(ctx, args...)
	if err != nil {
		return Entity{}, err
	}
	var probe struct {
		Error   string `json:"error"`
		Matches []Hit  `json:"matches"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return Entity{}, fmt.Errorf("5e get: decoding JSON: %w", err)
	}
	if probe.Error == "ambiguous" {
		return Entity{}, &AmbiguousError{Matches: probe.Matches}
	}
	var entity Entity
	if err := json.Unmarshal(body, &entity); err != nil {
		return Entity{}, fmt.Errorf("5e get: decoding JSON: %w", err)
	}
	return entity, nil
}

func (a Adapter) decodeRaw(ctx context.Context, args ...string) (json.RawMessage, error) {
	full := append(append([]string{}, args...), "--json")
	stdout, stderr, code, err := a.run(ctx, full...)
	if err != nil {
		return nil, err
	}
	body := bytesTrimSpace(stdout)
	if len(body) == 0 {
		return nil, &ExitError{Args: full, Code: code, Stderr: string(stderr)}
	}
	return body, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
