package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store loads and saves layout preferences.
type Store interface {
	Load() (Layout, error)
	Save(Layout) error
}

// JSONStore persists layout preferences beside the campaign workspace file.
type JSONStore struct {
	Path string
}

func NewJSON(path string) JSONStore { return JSONStore{Path: path} }

func (s JSONStore) Load() (Layout, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Layout{}, fmt.Errorf("preferences not found: %w", err)
		}
		return Layout{}, fmt.Errorf("read preferences: %w", err)
	}
	var layout Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		return Layout{}, fmt.Errorf("decode preferences: %w", err)
	}
	return layout.Normalize(), nil
}

func (s JSONStore) Save(layout Layout) error {
	if s.Path == "" {
		return fmt.Errorf("preferences path is required")
	}
	layout = layout.Normalize()
	layout.PaneLayout = 0
	layout.PaneSplit = 0
	layout.HorizontalSplit = 0
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create preferences directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".preferences-*.tmp")
	if err != nil {
		return fmt.Errorf("create preferences temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("secure preferences temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write preferences: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close preferences temporary file: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace preferences: %w", err)
	}
	return nil
}

// DefaultPath returns ~/.config/dungeon/preferences.json (or platform equivalent).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "dungeon", "preferences.json"), nil
}

// BesideWorkspace derives preferences.json next to a workspace.json path.
func BesideWorkspace(workspacePath string) string {
	return filepath.Join(filepath.Dir(workspacePath), "preferences.json")
}
