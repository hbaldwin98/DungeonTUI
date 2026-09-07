package classify

import (
	"fmt"
	"strings"
)

// Harness is an optional cleanup pass after ingest. When on, ingested
// records are retyped from structure and those cleaned rows are what
// land in the workspace. Harnesses do not write dump files.
type Harness string

const (
	HarnessOff Harness = ""
	HarnessOn  Harness = "on"
)

func ParseHarness(value string) (Harness, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "off", "none":
		return HarnessOff, nil
	case "on", "clean", "apply", "dump", "diagnose":
		return HarnessOn, nil
	default:
		return "", fmt.Errorf("unknown harness %q (off, on)", value)
	}
}

func NextHarness(value Harness) Harness {
	if value == HarnessOff {
		return HarnessOn
	}
	return HarnessOff
}

func (h Harness) Label() string {
	if h == HarnessOff {
		return "off"
	}
	return string(HarnessOn)
}
