package ingest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
)

const MaxMarkdownBytes int64 = 16 << 20

// ApplyFile reads markdown from path and Apply it.
func ApplyFile(ws domain.Workspace, path string, opts Options) (domain.Workspace, Report, error) {
	opts.report(StageRead, "Reading "+filepath.Base(path), 0, 0)
	file, err := os.Open(path)
	if err != nil {
		return ws, Report{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxMarkdownBytes+1))
	if err != nil {
		return ws, Report{}, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(data)) > MaxMarkdownBytes {
		return ws, Report{}, fmt.Errorf("read %s: file exceeds %d MiB limit", path, MaxMarkdownBytes>>20)
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
