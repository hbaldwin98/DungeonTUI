package classify

import (
	"fmt"
	"strings"
)

// Harness is the agent that cleans ingested records after import.
// Off skips the agent. Harnesses do not write dump files.
type Harness string

const (
	HarnessOff      Harness = ""
	HarnessClaude   Harness = "claude"
	HarnessOpenCode Harness = "opencode"
	HarnessCodex    Harness = "codex"
	HarnessCursor   Harness = "cursor"
)

var harnessCycle = []Harness{HarnessOff, HarnessClaude, HarnessOpenCode, HarnessCodex, HarnessCursor}

func ParseHarness(value string) (Harness, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "off", "none":
		return HarnessOff, nil
	case "claude":
		return HarnessClaude, nil
	case "opencode", "open-code":
		return HarnessOpenCode, nil
	case "codex":
		return HarnessCodex, nil
	case "cursor", "cursor-agent", "agent":
		return HarnessCursor, nil
	case "on", "clean", "apply", "dump", "diagnose":
		return HarnessClaude, nil
	default:
		return "", fmt.Errorf("unknown harness %q (off, claude, opencode, codex, cursor)", value)
	}
}

func NextHarness(value Harness) Harness {
	for i, h := range harnessCycle {
		if h == value {
			return harnessCycle[(i+1)%len(harnessCycle)]
		}
	}
	return HarnessClaude
}

func (h Harness) Label() string {
	if h == HarnessOff {
		return "off"
	}
	return string(h)
}

func (h Harness) AgentName() string {
	switch h {
	case HarnessClaude:
		return "Claude"
	case HarnessOpenCode:
		return "OpenCode"
	case HarnessCodex:
		return "Codex"
	case HarnessCursor:
		return "Cursor Agent"
	default:
		return ""
	}
}
