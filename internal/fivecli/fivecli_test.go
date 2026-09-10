package fivecli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubBinary writes an executable shell script standing in for `5e`.
func stubBinary(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub binary uses a POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "5e")
	body := "#!/bin/sh\n" + script
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const readyBody = `{"dataPath":"/data","dataExists":true,"dataFingerprint":"abc",` +
	`"indexPath":"/cache/index.sqlite","indexExists":true,"indexFingerprint":"abc",` +
	`"current":true,"ready":true}`

func TestDiagnoseDecodesReadyDoctor(t *testing.T) {
	adapter := Adapter{Binary: stubBinary(t, "printf '%s' '"+readyBody+"'\n")}

	doctor, err := adapter.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("ready doctor should not error: %v", err)
	}
	if !doctor.Ready || doctor.State() != StateReady {
		t.Fatalf("expected ready state, got %v: %#v", doctor.State(), doctor)
	}
	if doctor.IndexPath != "/cache/index.sqlite" {
		t.Fatalf("index path not decoded: %#v", doctor)
	}
	if !strings.Contains(doctor.Summary(), "/cache/index.sqlite") {
		t.Fatalf("ready summary should name the index: %q", doctor.Summary())
	}
}

// A tool that is installed but not set up exits 1 while still reporting a
// complete body. That is a diagnosis, not a failure to consult the tool.
func TestDiagnoseKeepsBodyFromNonzeroExit(t *testing.T) {
	body := `{"dataPath":"/data","dataExists":true,"indexPath":"/i.sqlite",` +
		`"indexExists":false,"current":false,"ready":false,` +
		`"issue":"no index at /i.sqlite; run ` + "`5e ingest`" + `"}`
	adapter := Adapter{Binary: stubBinary(t,
		"printf '%s' '"+body+"'\nprintf 'no index\\n' >&2\nexit 1\n")}

	doctor, err := adapter.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("a not-ready diagnosis must not be an error: %v", err)
	}
	if doctor.State() != StateMissingIndex {
		t.Fatalf("expected missing index, got %v", doctor.State())
	}
	if !strings.Contains(doctor.Summary(), "5e ingest") {
		t.Fatalf("summary should carry the tool's own remedy: %q", doctor.Summary())
	}
}

func TestDiagnoseReportsMissingBinary(t *testing.T) {
	adapter := Adapter{Binary: filepath.Join(t.TempDir(), "absent-5e")}

	_, err := adapter.Diagnose(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if adapter.Available() {
		t.Fatal("an absent binary must not report available")
	}
	state, summary := adapter.Status(context.Background())
	if state != StateUnavailable {
		t.Fatalf("expected unavailable state, got %v", state)
	}
	if !strings.Contains(summary, "unavailable") {
		t.Fatalf("summary should say unavailable: %q", summary)
	}
}

func TestDiagnoseReportsExitWithoutJSON(t *testing.T) {
	adapter := Adapter{Binary: stubBinary(t, "printf 'boom\\n' >&2\nexit 3\n")}

	_, err := adapter.Diagnose(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected an ExitError, got %v", err)
	}
	if exitErr.Code != 3 {
		t.Fatalf("expected exit code 3, got %d", exitErr.Code)
	}
	if !strings.Contains(exitErr.Error(), "boom") {
		t.Fatalf("stderr should reach the caller: %q", exitErr.Error())
	}
}

func TestDiagnoseReportsUndecodableSuccess(t *testing.T) {
	adapter := Adapter{Binary: stubBinary(t, "printf 'not json'\n")}

	_, err := adapter.Diagnose(context.Background())
	if err == nil {
		t.Fatal("undecodable output should error")
	}
	if errors.Is(err, ErrUnavailable) {
		t.Fatalf("a running tool is available even when its output is bad: %v", err)
	}
	if !strings.Contains(err.Error(), "decoding JSON") {
		t.Fatalf("expected a decode error, got %v", err)
	}
}

func TestRunPassesJSONFlagAndArguments(t *testing.T) {
	adapter := Adapter{Binary: stubBinary(t, `printf '{"issue":"%s"}' "$*"`)}

	var doctor Doctor
	if err := adapter.decodeJSON(context.Background(), &doctor, "doctor"); err != nil {
		t.Fatal(err)
	}
	if doctor.Issue != "doctor --json" {
		t.Fatalf("expected the subcommand to receive --json, got %q", doctor.Issue)
	}
}

func TestRunHonorsTimeout(t *testing.T) {
	adapter := Adapter{
		Binary:  stubBinary(t, "sleep 5\n"),
		Timeout: 50 * time.Millisecond,
	}

	start := time.Now()
	_, err := adapter.Diagnose(context.Background())
	if err == nil {
		t.Fatal("a hung subprocess should not block indefinitely")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout was not enforced, waited %v", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a deadline error, got %v", err)
	}
}

func TestBinaryResolvesFromEnvironment(t *testing.T) {
	path := stubBinary(t, "printf '%s' '"+readyBody+"'\n")
	t.Setenv(BinaryEnv, path)

	var adapter Adapter
	if adapter.binary() != path {
		t.Fatalf("zero-value adapter should read %s, got %q", BinaryEnv, adapter.binary())
	}
	if _, err := adapter.Diagnose(context.Background()); err != nil {
		t.Fatalf("configured binary should run: %v", err)
	}
}

func TestStateClassification(t *testing.T) {
	cases := []struct {
		name   string
		doctor Doctor
		want   State
	}{
		{"missing data outranks index", Doctor{DataExists: false, IndexExists: true}, StateMissingData},
		{"missing index", Doctor{DataExists: true, IndexExists: false}, StateMissingIndex},
		{"fingerprints disagree", Doctor{DataExists: true, IndexExists: true, Current: false}, StateStale},
		{"current but not ready", Doctor{DataExists: true, IndexExists: true, Current: true}, StateStale},
		{"ready", Doctor{DataExists: true, IndexExists: true, Current: true, Ready: true}, StateReady},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.doctor.State(); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
