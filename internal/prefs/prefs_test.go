package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	store := NewJSON(path)
	want := DefaultLayout()
	want.Session.Root.Children[0].Ratio = 0.41
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Session.Root.Children[0].Ratio != 0.41 {
		t.Fatalf("ratio got=%v", got.Session.Root.Children[0].Ratio)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("preferences should be owner-only, mode=%v", info.Mode())
	}
}

func TestBesideWorkspace(t *testing.T) {
	got := BesideWorkspace("/tmp/dungeon/workspace.json")
	if got != "/tmp/dungeon/preferences.json" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadMissing(t *testing.T) {
	store := NewJSON(filepath.Join(t.TempDir(), "missing.json"))
	if _, err := store.Load(); err == nil {
		t.Fatal("expected missing preferences error")
	}
}
