package fivetools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultCacheDir is beside the owner's workspace, never inside the git tree.
func DefaultCacheDir() string {
	if dir := strings.TrimSpace(os.Getenv("DUNGEON_5ETOOLS_CACHE")); dir != "" {
		return dir
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfg, "dungeon", "5etools-cache")
	}
	return filepath.Join(".", ".5etools-cache")
}

// CacheFetcher reads a site-relative path from Dir, then from Inner, and
// writes successful Inner fetches back to Dir. That directory is the durable
// 5e plugin store until SQLite FTS5. JSON is never committed to git.
type CacheFetcher struct {
	Inner Fetcher
	Dir   string
}

func NewCacheFetcher(inner Fetcher, dir string) Fetcher {
	if strings.TrimSpace(dir) == "" {
		return inner
	}
	return CacheFetcher{Inner: inner, Dir: dir}
}

func (c CacheFetcher) Get(path string) ([]byte, error) {
	full, err := cacheFile(c.Dir, path)
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(full); err == nil {
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if c.Inner == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	data, err := c.Inner.Get(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return data, nil
	}
	_ = os.WriteFile(full, data, 0o644)
	return data, nil
}

// CacheOnlyFetcher never hits the network. The TUI uses it so @ peeks read
// JSON primed at ingest time.
type CacheOnlyFetcher struct {
	Dir string
}

func CacheOnly(dir string) Fetcher {
	return CacheOnlyFetcher{Dir: dir}
}

func (c CacheOnlyFetcher) Get(path string) ([]byte, error) {
	full, err := cacheFile(c.Dir, path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, err
	}
	return data, nil
}

func cacheFile(dir, path string) (string, error) {
	path = strings.TrimPrefix(filepath.ToSlash(path), "/")
	if path == "" || strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid 5e.tools cache path %q", path)
	}
	return filepath.Join(dir, filepath.FromSlash(path)), nil
}
