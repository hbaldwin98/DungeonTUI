package fivecli

import (
	"context"
	"fmt"
	"strings"
)

// Doctor is the decoded body of `5e doctor --json`.
//
// Field names follow the tool's JSON contract. `5e doctor` exits nonzero when
// the data or index is missing or stale, but still reports a complete body, so
// a not-ready result is data rather than an error.
type Doctor struct {
	DataPath         string `json:"dataPath"`
	DataExists       bool   `json:"dataExists"`
	DataFingerprint  string `json:"dataFingerprint"`
	IndexPath        string `json:"indexPath"`
	IndexExists      bool   `json:"indexExists"`
	IndexFingerprint string `json:"indexFingerprint"`
	Current          bool   `json:"current"`
	Ready            bool   `json:"ready"`
	Issue            string `json:"issue"`
}

// State classifies a diagnostic for presentation without re-deriving the rules.
type State int

const (
	// StateUnavailable means the binary could not be found or run.
	StateUnavailable State = iota
	// StateMissingData means the 5etools data tree is absent.
	StateMissingData
	// StateMissingIndex means the data is present but was never ingested.
	StateMissingIndex
	// StateStale means data and index both exist but disagree.
	StateStale
	// StateReady means lookups can be served.
	StateReady
)

func (s State) String() string {
	switch s {
	case StateMissingData:
		return "missing data"
	case StateMissingIndex:
		return "missing index"
	case StateStale:
		return "stale index"
	case StateReady:
		return "ready"
	default:
		return "unavailable"
	}
}

// State classifies the diagnostic. The order matters: data is the input to the
// index, so a missing data tree is reported ahead of an index built from it.
func (d Doctor) State() State {
	switch {
	case !d.DataExists:
		return StateMissingData
	case !d.IndexExists:
		return StateMissingIndex
	case !d.Current:
		return StateStale
	case d.Ready:
		return StateReady
	default:
		return StateStale
	}
}

// CanLookup reports whether search and get can be served.
//
// Lookups read the sqlite index only. The 5etools data tree is needed to
// (re-)ingest that index, so `5e doctor` reports not-ready when the data
// directory is unset even though every lookup still answers. Dungeon only
// consults the tool for lookups, so it gates on the index, not on readiness.
func (d Doctor) CanLookup() bool { return d.IndexExists }

// LookupSummary describes the tool from a lookup caller's point of view: an
// index that works despite an unconfigured data tree reads as usable, with the
// tool's own remedy kept for the setup it does block.
func (d Doctor) LookupSummary() string {
	if !d.CanLookup() {
		return d.Summary()
	}
	if issue := strings.TrimSpace(d.Issue); issue != "" && !d.Ready {
		return "5e-cli lookups ready · re-ingest blocked: " + issue
	}
	return "5e-cli ready: " + d.IndexPath
}

// Summary is a single line naming the state and, when the tool supplied one,
// its own remedy. The tool's issue text already names the command to run, so
// it is preferred over anything reconstructed here.
func (d Doctor) Summary() string {
	state := d.State()
	if issue := strings.TrimSpace(d.Issue); issue != "" {
		return fmt.Sprintf("5e-cli %s: %s", state, issue)
	}
	if state == StateReady {
		return "5e-cli ready: " + d.IndexPath
	}
	return "5e-cli " + state.String()
}

// Diagnose runs `5e doctor --json` and returns the typed diagnostic.
//
// A tool that is installed but not set up returns a Doctor with a nil error:
// that is a reportable condition, not a failure to consult it. Only an absent
// or unusable binary returns an error wrapping ErrUnavailable.
func (a Adapter) Diagnose(ctx context.Context) (Doctor, error) {
	var doctor Doctor
	if err := a.decodeJSON(ctx, &doctor, "doctor"); err != nil {
		return Doctor{}, err
	}
	return doctor, nil
}

// Status reports the diagnostic state, collapsing an unavailable binary into
// StateUnavailable so a caller that only wants to know whether lookups work
// does not have to distinguish absence from misconfiguration.
func (a Adapter) Status(ctx context.Context) (State, string) {
	doctor, err := a.Diagnose(ctx)
	if err != nil {
		return StateUnavailable, unavailableSummary(err)
	}
	return doctor.State(), doctor.Summary()
}

// LookupStatus reports whether lookups can be served and why not, for a caller
// that wants answers rather than a full setup verdict. See Doctor.CanLookup.
func (a Adapter) LookupStatus(ctx context.Context) (bool, string) {
	doctor, err := a.Diagnose(ctx)
	if err != nil {
		return false, unavailableSummary(err)
	}
	return doctor.CanLookup(), doctor.LookupSummary()
}

func unavailableSummary(err error) string {
	return "5e-cli unavailable: " + err.Error()
}
