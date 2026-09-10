package fivecli

import (
	"strings"
	"testing"
)

// An index without a configured data tree still answers every lookup; only
// re-ingest is blocked, so the rules scope must not read as broken.
func TestDoctorLookupIgnoresUningestableData(t *testing.T) {
	doctor := Doctor{
		IndexPath:   "/home/wit/.cache/5e-cli/index.sqlite",
		IndexExists: true,
		DataExists:  false,
		Ready:       false,
		Issue:       "data directory not set; use --data or FIVE_E_DATA",
	}
	if !doctor.CanLookup() {
		t.Fatal("an existing index can serve lookups")
	}
	summary := doctor.LookupSummary()
	if !strings.Contains(summary, "lookups ready") || !strings.Contains(summary, "data directory not set") {
		t.Fatalf("summary should report usable lookups and the blocked setup: %q", summary)
	}
	if state := doctor.State(); state == StateReady {
		t.Fatal("the tool's own verdict stays not-ready")
	}
}

func TestDoctorWithoutIndexCannotLookup(t *testing.T) {
	doctor := Doctor{DataExists: true, IndexExists: false, Issue: "index missing; run `5e ingest`"}
	if doctor.CanLookup() {
		t.Fatal("no index means no lookups")
	}
	if doctor.LookupSummary() != doctor.Summary() {
		t.Fatalf("an unusable tool keeps the tool's own summary: %q", doctor.LookupSummary())
	}
}
