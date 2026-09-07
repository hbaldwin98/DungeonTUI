package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

// ApplyFile reads markdown from path and Apply it.
func ApplyFile(ws domain.Workspace, path string, opts Options) (domain.Workspace, Report, error) {
	opts.report(StageRead, "Reading "+filepath.Base(path), 0, 0)
	data, err := os.ReadFile(path)
	if err != nil {
		return ws, Report{}, fmt.Errorf("read %s: %w", path, err)
	}
	opts.Path = path
	return Apply(ws, string(data), opts)
}

func ParseKind(value string) (domain.SourceKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto":
		return "", nil
	case "bestiary", "mm", "monsters":
		return domain.SourceBestiary, nil
	case "adventure":
		return domain.SourceAdventure, nil
	case "rules", "phb", "srd":
		return domain.SourceRules, nil
	default:
		return "", fmt.Errorf("unknown kind %q (bestiary, adventure, rules)", value)
	}
}
