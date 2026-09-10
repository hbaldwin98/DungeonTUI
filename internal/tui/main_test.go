package tui

import (
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestMain disables status expiry timers for the whole test binary. The
// synchronous command helpers (flushCmd, runCmds) call every returned command
// inline, so a real tea.Tick would make each of them sleep for the status
// lifetime. tea.Batch drops the nil, so commands keep their original shape.
// Tests that exercise expiry install their own stub.
func TestMain(m *testing.M) {
	scheduleStatusExpiry = func(time.Duration, uint64) tea.Cmd { return nil }
	os.Exit(m.Run())
}
