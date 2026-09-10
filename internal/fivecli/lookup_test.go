package fivecli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSearchDecodesHits(t *testing.T) {
	body := `[{"kind":"spell","name":"Fireball","source":"PHB","score":12.5,"snippet":"a bright streak"}]`
	adapter := Adapter{Binary: stubBinary(t, "printf '%s' '"+body+"'\n")}

	hits, err := adapter.Search(context.Background(), "fire", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "Fireball" || hits[0].Kind != "spell" {
		t.Fatalf("unexpected hits: %#v", hits)
	}
}

func TestSearchEmptyQuerySkipsBinary(t *testing.T) {
	adapter := Adapter{Binary: stubBinary(t, "exit 99\n")}

	hits, err := adapter.Search(context.Background(), "  ", 5)
	if err != nil {
		t.Fatal(err)
	}
	if hits != nil {
		t.Fatalf("expected nil hits, got %#v", hits)
	}
}

func TestGetDecodesEntity(t *testing.T) {
	body := `{"kind":"spell","name":"Fireball","source":"PHB","page":"241","text":"A bright streak flashes"}`
	adapter := Adapter{Binary: stubBinary(t, "printf '%s' '"+body+"'\n")}

	entity, err := adapter.Get(context.Background(), "spell", "Fireball", "")
	if err != nil {
		t.Fatal(err)
	}
	if entity.Text == "" || entity.Source != "PHB" {
		t.Fatalf("unexpected entity: %#v", entity)
	}
}

func TestGetReportsAmbiguousMatch(t *testing.T) {
	body := `{"error":"ambiguous","matches":[{"kind":"skill","name":"Athletics","source":"PHB"},{"kind":"skill","name":"Athletics","source":"XPHB"}]}`
	adapter := Adapter{Binary: stubBinary(t, "printf '%s' '"+body+"'\nexit 2\n")}

	_, err := adapter.Get(context.Background(), "skill", "Athletics", "")
	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("expected AmbiguousError, got %v", err)
	}
	if len(amb.Matches) != 2 {
		t.Fatalf("matches: %#v", amb.Matches)
	}
}

func TestGetPassesSourceFlag(t *testing.T) {
	adapter := Adapter{Binary: stubBinary(t, `case "$1" in get) printf '{"kind":"spell","name":"Fireball","source":"PHB","text":"ok"}' ;; esac`)}

	entity, err := adapter.Get(context.Background(), "spell", "Fireball", "PHB")
	if err != nil {
		t.Fatal(err)
	}
	if entity.Name != "Fireball" || !strings.Contains(entity.Text, "ok") {
		t.Fatalf("entity: %#v", entity)
	}
}

// 5e-cli prints `page` as a JSON number for most entities and as a string for
// sources with non-numeric pages; neither may fail the lookup.
func TestEntityPageDecodesNumberAndString(t *testing.T) {
	for _, tc := range []struct {
		body string
		want string
	}{
		{`{"kind":"spell","name":"Fireball","source":"XPHB","page":274,"text":"…"}`, "274"},
		{`{"kind":"spell","name":"Fireball","source":"XPHB","page":"A5","text":"…"}`, "A5"},
		{`{"kind":"spell","name":"Fireball","source":"XPHB","page":null,"text":"…"}`, ""},
		{`{"kind":"spell","name":"Fireball","source":"XPHB","text":"…"}`, ""},
	} {
		var entity Entity
		if err := json.Unmarshal([]byte(tc.body), &entity); err != nil {
			t.Fatalf("decoding %s: %v", tc.body, err)
		}
		if entity.Page.String() != tc.want {
			t.Fatalf("page from %s = %q, want %q", tc.body, entity.Page, tc.want)
		}
	}
}

// A contract check against a really installed 5e-cli. It skips unless the tool
// is present and ingested, so it costs nothing on a bare machine, but it is
// what catches the tool's JSON drifting from these structs.
func TestInstalledToolMatchesDecodedShape(t *testing.T) {
	adapter := Adapter{}
	if !adapter.Available() {
		t.Skip("5e-cli is not installed")
	}
	ctx := context.Background()
	if ready, summary := adapter.LookupStatus(ctx); !ready {
		t.Skipf("5e-cli cannot serve lookups: %s", summary)
	}

	hits, err := adapter.Search(ctx, "fireball", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 || hits[0].Name == "" || hits[0].Kind == "" {
		t.Fatalf("search returned nothing usable: %#v", hits)
	}

	entity, err := adapter.Get(ctx, hits[0].Kind, hits[0].Name, hits[0].Source)
	if err != nil {
		t.Fatalf("get %s %q (%s): %v", hits[0].Kind, hits[0].Name, hits[0].Source, err)
	}
	if strings.TrimSpace(entity.Text) == "" {
		t.Fatalf("get returned no text: %#v", entity)
	}
}
