package fivetools

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

func slug(parts ...string) string {
	var b strings.Builder
	for _, part := range parts {
		cleaned := strings.Map(func(r rune) rune {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				return unicode.ToLower(r)
			case r == ' ' || r == '-' || r == '_' || r == '\'':
				return '-'
			default:
				return -1
			}
		}, part)
		cleaned = strings.Trim(cleaned, "-")
		for strings.Contains(cleaned, "--") {
			cleaned = strings.ReplaceAll(cleaned, "--", "-")
		}
		if cleaned == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
		b.WriteString(cleaned)
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case json.Number:
		n, err := t.Int64()
		return int(n), err == nil
	case string:
		n := 0
		for _, r := range t {
			if r < '0' || r > '9' {
				return 0, false
			}
			n = n*10 + int(r-'0')
		}
		return n, t != ""
	default:
		return 0, false
	}
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		if t == float64(int(t)) {
			return fmt.Sprintf("%d", int(t))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := asString(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		if s := asString(t["name"]); s != "" {
			return s
		}
		if s := asString(t["special"]); s != "" {
			return s
		}
		if s := asString(t["source"]); s != "" {
			return s
		}
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case nil:
		return nil
	default:
		return []any{v}
	}
}

func sourceOf(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return asString(t["source"])
	default:
		return asString(v)
	}
}

func itemSource(item map[string]any) string {
	return sourceOf(item["source"])
}

func sourceMatch(item map[string]any, code string) bool {
	return strings.EqualFold(itemSource(item), code)
}

func jsonObjects(data []byte, key string) ([]map[string]any, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	raw, ok := envelope[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func jsonIndex(data []byte) (map[string]string, error) {
	out := map[string]string{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		out := make(map[string]any, len(in))
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return in
	}
	return out
}

func mergeMaps(base, overlay map[string]any) map[string]any {
	out := cloneMap(base)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range overlay {
		if k == "_copy" || k == "_mod" {
			continue
		}
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return strings.TrimRight(s[:n], " \n") + "…"
}

func summaryOf(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") {
			continue
		}
		if strings.HasPrefix(line, "*") && strings.HasSuffix(line, "*") && !strings.Contains(line, "**") {
			continue
		}
		return truncate(line, 220)
	}
	return ""
}
