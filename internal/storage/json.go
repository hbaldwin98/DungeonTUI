package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

// Store is the persistence boundary used by the application layer. The TUI
// does not need to know whether records are stored as JSON, SQLite, or through
// a future remote API.
type Store interface {
	Load() (domain.Workspace, error)
	Save(domain.Workspace) error
}

type JSONStore struct {
	Path string
}

func NewJSON(path string) JSONStore { return JSONStore{Path: path} }

func (s JSONStore) Load() (domain.Workspace, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.Workspace{}, fmt.Errorf("workspace not found: %w", err)
		}
		return domain.Workspace{}, fmt.Errorf("read workspace: %w", err)
	}
	var workspace domain.Workspace
	if err := json.Unmarshal(data, &workspace); err != nil {
		return domain.Workspace{}, fmt.Errorf("decode workspace: %w", err)
	}
	if workspace.Scope.WorldID == "" || workspace.Scope.CampaignID == "" {
		return domain.Workspace{}, fmt.Errorf("decode workspace: missing active scope")
	}
	for _, record := range workspace.Records {
		if err := record.Validate(); err != nil {
			return domain.Workspace{}, fmt.Errorf("decode workspace: %w", err)
		}
	}
	return workspace, nil
}

func (s JSONStore) Save(workspace domain.Workspace) error {
	if s.Path == "" {
		return fmt.Errorf("workspace path is required")
	}
	if _, err := domain.NewWorkspace(workspace.Scope, workspace.Records); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}
	data, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".workspace-*.tmp")
	if err != nil {
		return fmt.Errorf("create workspace temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("secure workspace temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write workspace: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close workspace temporary file: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace workspace: %w", err)
	}
	return nil
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "dungeon", "workspace.json"), nil
}
