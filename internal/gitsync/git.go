package gitsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultBinary is the git command looked up on PATH when none is configured.
const DefaultBinary = "git"

// BinaryEnv overrides the git binary, for a build that is not on PATH.
const BinaryEnv = "DUNGEON_GIT_BIN"

// DefaultTimeout bounds a single git invocation so a hung remote cannot
// freeze the CLI. Clone/push/pull of a large campaign may need more; callers
// pass a context with a deadline when they want a different bound.
const DefaultTimeout = 2 * time.Minute

const waitDelay = 100 * time.Millisecond

// ErrUnavailable reports that git could not be found or executed. Sync is
// optional: a workstation without git still runs Dungeon locally.
var ErrUnavailable = errors.New("git is not available")

// Git runs the workstation git binary. The zero value resolves git from the
// environment or PATH and inherits the process environment, minus GIT_DIR and
// friends that would point at some other repository.
type Git struct {
	Binary  string
	Env     []string
	Extra   []string
	Timeout time.Duration
}

func (g Git) binary() string {
	if g.Binary != "" {
		return g.Binary
	}
	if fromEnv := strings.TrimSpace(os.Getenv(BinaryEnv)); fromEnv != "" {
		return fromEnv
	}
	return DefaultBinary
}

func (g Git) timeout() time.Duration {
	if g.Timeout > 0 {
		return g.Timeout
	}
	return DefaultTimeout
}

// Available reports whether the configured binary can be resolved.
func (g Git) Available() bool {
	binary := g.binary()
	if strings.ContainsRune(binary, os.PathSeparator) {
		info, err := os.Stat(binary)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(binary)
	return err == nil
}

// Run executes git in dir and returns stdout. A missing binary is
// ErrUnavailable; a failing git command wraps stderr.
func (g Git) Run(dir string, args ...string) (string, error) {
	return g.run(context.Background(), dir, args...)
}

func (g Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	if !g.Available() {
		return "", fmt.Errorf("%w: %q not found", ErrUnavailable, g.binary())
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout())
		defer cancel()
	}

	full := append(append([]string{}, g.Extra...), args...)
	cmd := exec.CommandContext(ctx, g.binary(), full...)
	cmd.Dir = dir
	cmd.Env = g.env()
	cmd.WaitDelay = waitDelay
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout, stderr := strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String())
	if runErr == nil {
		return stdout, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctxErr)
	}
	detail := stderr
	if detail == "" {
		detail = runErr.Error()
	}
	return stdout, fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
}

func (g Git) env() []string {
	if g.Env != nil {
		return g.Env
	}
	return sanitizeGitEnv(os.Environ())
}

var gitDirVars = []string{
	"GIT_DIR=",
	"GIT_WORK_TREE=",
	"GIT_INDEX_FILE=",
	"GIT_OBJECT_DIRECTORY=",
	"GIT_NAMESPACE=",
	"GIT_COMMON_DIR=",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES=",
}

func sanitizeGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		drop := false
		for _, prefix := range gitDirVars {
			if strings.HasPrefix(kv, prefix) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}
