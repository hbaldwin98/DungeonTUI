package fivetools

import (
	"net/url"
	"strings"
)

// Ref is a 5e.tools source the owner chose to ingest.
type Ref struct {
	ID   string // canonical catalog id, e.g. LMoP, XMM
	Kind string // "adventure" or "book" when known from the URL
	Raw  string
}

// ParseRef reads a 5e.tools page URL, a 5e:ID token, or a bare source id.
func ParseRef(value string) (Ref, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Ref{}, false
	}
	raw := value
	if strings.HasPrefix(strings.ToLower(value), "5e:") {
		id := strings.TrimSpace(value[3:])
		if id == "" {
			return Ref{}, false
		}
		return Ref{ID: id, Raw: raw}, true
	}

	if strings.Contains(value, "://") || strings.Contains(value, "5e.tools") {
		parsed, err := url.Parse(value)
		if err != nil {
			return Ref{}, false
		}
		host := strings.ToLower(parsed.Host)
		if host != "" && host != "5e.tools" && host != "www.5e.tools" {
			return Ref{}, false
		}
		kind := ""
		switch {
		case strings.Contains(parsed.Path, "adventure"):
			kind = "adventure"
		case strings.Contains(parsed.Path, "book"):
			kind = "book"
		}
		id := hashID(parsed.Fragment)
		if id == "" {
			id = strings.TrimSpace(parsed.Query().Get("id"))
		}
		if id == "" {
			return Ref{}, false
		}
		return Ref{ID: id, Kind: kind, Raw: raw}, true
	}

	if looksLikeSourceID(value) {
		return Ref{ID: value, Raw: raw}, true
	}
	return Ref{}, false
}

func hashID(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	fragment = strings.TrimPrefix(fragment, "#")
	if fragment == "" {
		return ""
	}
	id, _, _ := strings.Cut(fragment, ",")
	id = strings.TrimSpace(id)
	return id
}

func looksLikeSourceID(value string) bool {
	if strings.ContainsAny(value, `/\`) {
		return false
	}
	if strings.Contains(value, ".") {
		return false
	}
	if len(value) < 2 || len(value) > 24 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// LooksLikeRef reports whether value should be treated as a 5e.tools ingest
// target rather than a local markdown path.
func LooksLikeRef(value string) bool {
	_, ok := ParseRef(value)
	return ok
}
