package fivetools

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultBaseURL = "https://5e.tools"

const MaxResponseBytes int64 = 64 << 20

var ErrNotFound = errors.New("5e.tools path not found")

// Fetcher loads a 5e.tools site-relative path such as "data/books.json".
// Implementations must not write into the git working tree.
type Fetcher interface {
	Get(path string) ([]byte, error)
}

// HTTPFetcher reads from a live 5e.tools (or mirror) origin.
type HTTPFetcher struct {
	BaseURL string
	Client  *http.Client
}

func DefaultFetcher() Fetcher {
	return HTTPFetcher{
		BaseURL: DefaultBaseURL,
		Client:  &http.Client{Timeout: 90 * time.Second},
	}
}

func (f HTTPFetcher) Get(path string) ([]byte, error) {
	base := strings.TrimRight(f.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	url := base + "/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "DungeonCampaignWorkstation/0.1 (personal library ingest)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %s", path, resp.Status)
	}
	if resp.ContentLength > MaxResponseBytes {
		return nil, fmt.Errorf("GET %s: response exceeds %d MiB limit", path, MaxResponseBytes>>20)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxResponseBytes {
		return nil, fmt.Errorf("GET %s: response exceeds %d MiB limit", path, MaxResponseBytes>>20)
	}
	return data, nil
}

// DirFetcher reads a local 5e.tools checkout (the directory that contains data/).
type DirFetcher struct {
	Root string
}

func (f DirFetcher) Get(path string) ([]byte, error) {
	full := filepath.Join(f.Root, filepath.FromSlash(path))
	file, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxResponseBytes {
		return nil, fmt.Errorf("read %s: response exceeds %d MiB limit", path, MaxResponseBytes>>20)
	}
	return data, nil
}

// MapFetcher is an in-memory stub for tests. Missing keys are not found.
type MapFetcher map[string][]byte

func (m MapFetcher) Get(path string) ([]byte, error) {
	path = strings.TrimPrefix(path, "/")
	data, ok := m[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	return data, nil
}

func getOptional(fetcher Fetcher, path string) ([]byte, bool, error) {
	data, err := fetcher.Get(path)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func ResolveFetcher(fetcher Fetcher, dataDir string) Fetcher {
	if fetcher != nil {
		return fetcher
	}
	if strings.TrimSpace(dataDir) != "" {
		return DirFetcher{Root: dataDir}
	}
	return NewCacheFetcher(DefaultFetcher(), DefaultCacheDir())
}
