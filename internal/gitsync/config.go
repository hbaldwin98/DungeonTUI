package gitsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config is machine-local: the remote to push to and the clone that holds
// the mirror. It is not campaign data and is not itself synced (D-005).
type Config struct {
	Remote string `json:"remote"`
	Branch string `json:"branch,omitempty"`
	Dir    string `json:"dir,omitempty"`
}

// DefaultConfigPath is ~/.config/dungeon/sync.json (or platform equivalent).
func DefaultConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sync.json"), nil
}

// DefaultRepoDir is ~/.config/dungeon/sync-repo.
func DefaultRepoDir() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sync-repo"), nil
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "dungeon"), nil
}

// LoadConfig reads a sync.json. A missing file is os.ErrNotExist.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("sync is not configured: %w", err)
		}
		return Config{}, fmt.Errorf("read sync config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode sync config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes sync.json.
func SaveConfig(path string, cfg Config) error {
	if path == "" {
		return fmt.Errorf("sync config path is required")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sync config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create sync config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sync-*.tmp")
	if err != nil {
		return fmt.Errorf("create sync config temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("secure sync config temporary file: %w", err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close sync config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace sync config: %w", err)
	}
	return nil
}

// Client builds a git client from config, filling default clone path and branch.
func (cfg Config) Client() (Client, error) {
	dir := cfg.Dir
	if dir == "" {
		var err error
		dir, err = DefaultRepoDir()
		if err != nil {
			return Client{}, err
		}
	}
	branch := cfg.Branch
	if branch == "" {
		branch = "main"
	}
	return Client{Dir: dir, Remote: cfg.Remote, Branch: branch}, nil
}
