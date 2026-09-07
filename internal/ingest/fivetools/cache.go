package fivetools

import (
	"encoding/json"
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
		if !json.Valid(data) {
			return nil, fmt.Errorf("cache %s: cached response is not valid JSON", path)
		}
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
		return nil, fmt.Errorf("create 5e.tools cache directory: %w", err)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("cache %s: response is not valid JSON", path)
	}
	if err := writeCacheFile(full, data); err != nil {
		return nil, fmt.Errorf("cache %s: %w", path, err)
	}
	return data, nil
}

func writeCacheFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".5etools-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
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
	if !json.Valid(data) {
		return nil, fmt.Errorf("cache %s: cached response is not valid JSON", path)
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
