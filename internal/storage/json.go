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

const CurrentSchemaVersion = 1

func NewJSON(path string) JSONStore { return JSONStore{Path: path} }

func (s JSONStore) BackupPath() string { return s.Path + ".bak" }

func (s JSONStore) Load() (domain.Workspace, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.Workspace{}, fmt.Errorf("workspace not found: %w", err)
		}
		return domain.Workspace{}, fmt.Errorf("read workspace: %w", err)
	}
	return decodeWorkspace(data)
}

func (s JSONStore) Save(workspace domain.Workspace) error {
	if s.Path == "" {
		return fmt.Errorf("workspace path is required")
	}
	workspace.EnsureLibrary()
	workspace.SchemaVersion = CurrentSchemaVersion
	if err := workspace.Validate(); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}
	data, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	if current, err := os.ReadFile(s.Path); err == nil {
		if _, err := decodeWorkspace(current); err != nil {
			return fmt.Errorf("refuse to replace unreadable workspace: %w", err)
		}
		if err := atomicWrite(s.BackupPath(), current); err != nil {
			return fmt.Errorf("back up workspace: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing workspace: %w", err)
	}
	if err := atomicWrite(s.Path, data); err != nil {
		return fmt.Errorf("save workspace: %w", err)
	}
	return nil
}

// RestoreBackup validates the backup before replacing an unreadable primary.
func (s JSONStore) RestoreBackup() error {
	data, err := os.ReadFile(s.BackupPath())
	if err != nil {
		return fmt.Errorf("read workspace backup: %w", err)
	}
	if _, err := decodeWorkspace(data); err != nil {
		return fmt.Errorf("validate workspace backup: %w", err)
	}
	if err := atomicWrite(s.Path, data); err != nil {
		return fmt.Errorf("restore workspace backup: %w", err)
	}
	return nil
}

func decodeWorkspace(data []byte) (domain.Workspace, error) {
	var workspace domain.Workspace
	if err := json.Unmarshal(data, &workspace); err != nil {
		return domain.Workspace{}, fmt.Errorf("decode workspace: %w", err)
	}
	if workspace.SchemaVersion < 0 || workspace.SchemaVersion > CurrentSchemaVersion {
		return domain.Workspace{}, fmt.Errorf("unsupported workspace schema version %d", workspace.SchemaVersion)
	}
	workspace.SchemaVersion = CurrentSchemaVersion
	workspace.EnsureLibrary()
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, fmt.Errorf("validate workspace: %w", err)
	}
	return workspace, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".workspace-*.tmp")
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
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync workspace temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close workspace temporary file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace workspace: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open workspace directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync workspace directory: %w", err)
	}
	return nil
}

// ExportTo writes a validated copy of the workspace to path.
func (s JSONStore) ExportTo(path string) error {
	ws, err := s.Load()
	if err != nil {
		return err
	}
	return NewJSON(path).replaceValidated(ws)
}

// RestoreFrom replaces the primary workspace with a validated export or backup
// file, including when the current primary is unreadable.
func (s JSONStore) RestoreFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read restore source: %w", err)
	}
	ws, err := decodeWorkspace(data)
	if err != nil {
		return fmt.Errorf("invalid restore source: %w", err)
	}
	return s.replaceValidated(ws)
}

func (s JSONStore) replaceValidated(workspace domain.Workspace) error {
	if s.Path == "" {
		return fmt.Errorf("workspace path is required")
	}
	workspace.EnsureLibrary()
	workspace.SchemaVersion = CurrentSchemaVersion
	if err := workspace.Validate(); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}
	data, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	if err := atomicWrite(s.Path, data); err != nil {
		return fmt.Errorf("write workspace: %w", err)
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
