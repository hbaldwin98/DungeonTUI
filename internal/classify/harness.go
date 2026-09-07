package classify

import (
	"fmt"
	"strings"
)

// Harness is the optional post-ingest agent pass (dump JSON, optional structural apply).
type Harness string

const (
	HarnessOff   Harness = ""
	HarnessDump  Harness = "dump"
	HarnessApply Harness = "apply"
)

func ParseHarness(value string) (Harness, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "off", "none":
		return HarnessOff, nil
	case "dump", "diagnose", "on":
		return HarnessDump, nil
	case "apply":
		return HarnessApply, nil
	default:
		return "", fmt.Errorf("unknown harness %q (off, dump, apply)", value)
	}
}

func NextHarness(value Harness) Harness {
	switch value {
	case HarnessOff:
		return HarnessDump
	case HarnessDump:
		return HarnessApply
	default:
		return HarnessOff
	}
}

func (h Harness) Label() string {
	if h == HarnessOff {
		return "off"
	}
	return string(h)
}
