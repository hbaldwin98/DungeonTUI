package main

import (
	"strings"
	"testing"
)

func TestRunSyncRequiresASubcommand(t *testing.T) {
	err := runSync(nil)
	if err == nil || !strings.Contains(err.Error(), "dungeon sync init") {
		t.Fatalf("expected usage, got %v", err)
	}
}

func TestNewSyncConfigRequiresRemote(t *testing.T) {
	_, err := newSyncConfig("  ", "", "main")
	if err == nil {
		t.Fatal("empty remote must fail")
	}
}

func TestNewSyncConfigFillsBranch(t *testing.T) {
	cfg, err := newSyncConfig("git@example.com:me/campaigns.git", "/tmp/sync", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Branch != "main" || cfg.Dir != "/tmp/sync" {
		t.Fatalf("config %#v", cfg)
	}
}
