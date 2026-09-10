// Package fivecli adapts the external 5e-cli tool as an optional data plane.
//
// Dungeon owns no mechanical 5e ruleset (D-029). Mechanical lookup is delegated
// to the separate `5e` binary across its documented JSON boundary, so the two
// tools stay independently installable and Dungeon never vendors a rules store.
// Every capability here is optional: a workstation without `5e` installed must
// keep working, so callers get a typed unavailability instead of a hard failure.
package fivecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultBinary is the command looked up on PATH when no path is configured.
const DefaultBinary = "5e"

// BinaryEnv overrides the binary location, for a build that is not on PATH.
const BinaryEnv = "DUNGEON_5E_BIN"

// DefaultTimeout bounds a single lookup so a wedged subprocess cannot hang
// the TUI. Callers wanting a different bound pass their own context.
const DefaultTimeout = 15 * time.Second

// waitDelay bounds how long a killed subprocess may hold its output pipes
// open before they are closed out from under it.
const waitDelay = 100 * time.Millisecond

// ErrUnavailable reports that the 5e binary could not be found or executed.
// It is the expected condition on a workstation that has not installed the
// tool, not a defect, so callers should degrade instead of surfacing a crash.
var ErrUnavailable = errors.New("5e-cli is not available")

// Adapter runs 5e-cli subcommands and decodes their JSON output.
// The zero value resolves the binary from the environment or PATH.
type Adapter struct {
	// Binary is the command to run. Empty means BinaryEnv, then DefaultBinary.
	Binary string
	// Dir is the working directory for the subprocess. Empty means inherited.
	Dir string
	// Env replaces the subprocess environment. Nil means inherited.
	Env []string
	// Timeout bounds one run when the caller's context has no deadline.
	// Zero means DefaultTimeout.
	Timeout time.Duration
}

// binary resolves the command to execute.
func (a Adapter) binary() string {
	if a.Binary != "" {
		return a.Binary
	}
	if fromEnv := strings.TrimSpace(os.Getenv(BinaryEnv)); fromEnv != "" {
		return fromEnv
	}
	return DefaultBinary
}

func (a Adapter) timeout() time.Duration {
	if a.Timeout > 0 {
		return a.Timeout
	}
	return DefaultTimeout
}

// Available reports whether the configured binary can be resolved.
func (a Adapter) Available() bool {
	binary := a.binary()
	if strings.ContainsRune(binary, os.PathSeparator) {
		info, err := os.Stat(binary)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(binary)
	return err == nil
}

// ExitError reports a subcommand that ran and failed without usable JSON.
// Stderr is carried because 5e-cli explains actionable setup problems there.
type ExitError struct {
	Args   []string
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	detail := strings.TrimSpace(e.Stderr)
	if detail == "" {
		detail = "no error output"
	}
	return fmt.Sprintf("5e %s: exit %d: %s", strings.Join(e.Args, " "), e.Code, detail)
}

// run executes a subcommand and returns its stdout, stderr, and exit code.
//
// A nonzero exit is not by itself an error here. `5e doctor --json` reports a
// complete, useful diagnostic body on stdout and still exits 1 when the index
// is missing or stale, so the decision to fail belongs to the typed caller
// that knows whether the body it wanted was produced.
func (a Adapter) run(ctx context.Context, args ...string) (stdout, stderr []byte, code int, err error) {
	if !a.Available() {
		return nil, nil, 0, fmt.Errorf("%w: %q not found", ErrUnavailable, a.binary())
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.timeout())
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, a.binary(), args...)
	cmd.Dir = a.Dir
	cmd.Env = a.Env
	// Killing the process does not close the output pipes a grandchild may
	// still hold, so bound the wait for I/O as well as for the process.
	// Without this a wedged subprocess outlives its own deadline.
	cmd.WaitDelay = waitDelay
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout, stderr = outBuf.Bytes(), errBuf.Bytes()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	// A cancelled or timed-out run reports the process as exiting, so the
	// context is consulted first; otherwise a killed tool would be
	// misreported as one that ran and rejected the request.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout, stderr, 0, fmt.Errorf("5e %s: %w", strings.Join(args, " "), ctxErr)
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	// The process could not be started at all.
	return stdout, stderr, 0, fmt.Errorf("%w: %v", ErrUnavailable, runErr)
}

// decodeJSON runs a subcommand with --json and decodes stdout into target.
//
// It tolerates a nonzero exit when the body decodes, and reports an ExitError
// only when the command produced no JSON to interpret.
func (a Adapter) decodeJSON(ctx context.Context, target any, args ...string) error {
	full := append(append([]string{}, args...), "--json")
	stdout, stderr, code, err := a.run(ctx, full...)
	if err != nil {
		return err
	}
	body := bytes.TrimSpace(stdout)
	if len(body) == 0 {
		return &ExitError{Args: full, Code: code, Stderr: string(stderr)}
	}
	if err := json.Unmarshal(body, target); err != nil {
		if code != 0 {
			return &ExitError{Args: full, Code: code, Stderr: string(stderr)}
		}
		return fmt.Errorf("5e %s: decoding JSON: %w", strings.Join(full, " "), err)
	}
	return nil
}
