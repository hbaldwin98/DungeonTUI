package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/search"
	_ "modernc.org/sqlite"
)

const (
	entityWorld      = "world"
	entitySource     = "source"
	entityRecord     = "record"
	entitySession    = "session"
	entityPlan       = "plan"
	entityRecon      = "recon"
	entityCollection = "collection"
	entityAdventure  = "adventure"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS meta (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS entities (
  kind TEXT NOT NULL,
  id TEXT NOT NULL,
  ord INTEGER NOT NULL,
  payload TEXT NOT NULL,
  PRIMARY KEY (kind, id)
);
`

type Blob struct {
	ID   string
	Data []byte
}

type SQLiteStore struct {
	Path  string
	mu    sync.Mutex
	db    *sql.DB
	index *search.Index
}

func NewSQLite(path string) *SQLiteStore {
	return &SQLiteStore{Path: path}
}

func (s *SQLiteStore) BackupPath() string {
	if s == nil {
		return ""
	}
	return s.Path + ".bak"
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "dungeon", "workspace.sqlite"), nil
}

func LegacyJSONPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(dir, "dungeon", "workspace.json"), nil
}

func OpenDefault() (*SQLiteStore, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	store := NewSQLite(path)
	jsonPath, err := LegacyJSONPath()
	if err != nil {
		return store, err
	}
	if err := store.MigrateFromJSON(jsonPath); err != nil {
		return store, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
}

func (s *SQLiteStore) closeLocked() {
	if s.index != nil {
		s.index = nil
	}
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

func (s *SQLiteStore) Load() (domain.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.Workspace{}, fmt.Errorf("workspace not found: %w", err)
		}
		return domain.Workspace{}, fmt.Errorf("stat workspace: %w", err)
	}
	if err := s.ensureOpenLocked(); err != nil {
		return domain.Workspace{}, fmt.Errorf("read workspace: %w", err)
	}
	return s.loadLocked()
}

func (s *SQLiteStore) Save(workspace domain.Workspace) error {
	return s.write(workspace, false)
}

func (s *SQLiteStore) write(workspace domain.Workspace, replaceCorrupt bool) error {
	if s == nil || s.Path == "" {
		return fmt.Errorf("workspace path is required")
	}
	workspace.EnsureLibrary()
	workspace.SchemaVersion = CurrentSchemaVersion
	if err := workspace.Validate(); err != nil {
		return fmt.Errorf("validate workspace: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Path); err == nil && !replaceCorrupt {
		if err := s.ensureOpenLocked(); err != nil {
			return fmt.Errorf("refuse to replace unreadable workspace: %w", err)
		}
		if err := s.backupLocked(); err != nil {
			return err
		}
	} else if replaceCorrupt {
		s.closeLocked()
		if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove unreadable workspace: %w", err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat workspace: %w", err)
	}
	if err := s.ensureOpenLocked(); err != nil {
		return err
	}
	return s.replaceLocked(workspace)
}

func (s *SQLiteStore) ExportTo(path string) error {
	ws, err := s.Load()
	if err != nil {
		return err
	}
	return NewJSON(path).Save(ws)
}

func (s *SQLiteStore) RestoreFrom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read restore source: %w", err)
	}
	ws, err := decodeWorkspace(data)
	if err != nil {
		return fmt.Errorf("invalid restore source: %w", err)
	}
	return s.write(ws, true)
}

func (s *SQLiteStore) RestoreBackup() error {
	bak := NewSQLite(s.BackupPath())
	defer bak.Close()
	ws, err := bak.Load()
	if err != nil {
		return fmt.Errorf("read workspace backup: %w", err)
	}
	return s.write(ws, true)
}

func (s *SQLiteStore) MigrateFromJSON(jsonPath string) error {
	if _, err := os.Stat(s.Path); err == nil {
		if _, loadErr := s.Load(); loadErr == nil {
			return nil
		}
		return fmt.Errorf("refuse to migrate over unreadable sqlite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat workspace: %w", err)
	}
	ws, err := NewJSON(jsonPath).Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("migrate workspace json: %w", err)
	}
	return s.Save(ws)
}

func (s *SQLiteStore) Reindex(docs []search.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Path); err != nil {
		return err
	}
	if err := s.ensureOpenLocked(); err != nil {
		return err
	}
	return s.index.Replace(docs)
}

func (s *SQLiteStore) Search() search.Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	return search.FromIndex(s.index)
}

func (s *SQLiteStore) SaveAdventures(books []Blob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpenLocked(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin adventure write: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM entities WHERE kind = ?`, entityAdventure); err != nil {
		tx.Rollback()
		return fmt.Errorf("clear adventures: %w", err)
	}
	for i, book := range books {
		if _, err := tx.Exec(
			`INSERT INTO entities (kind, id, ord, payload) VALUES (?, ?, ?, ?)`,
			entityAdventure, book.ID, i, string(book.Data),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("insert adventure %s: %w", book.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit adventures: %w", err)
	}
	return nil
}

func (s *SQLiteStore) LoadAdventures() ([]Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.ensureOpenLocked(); err != nil {
		return nil, err
	}
	return loadEntities(s.db, entityAdventure)
}

func (s *SQLiteStore) ensureOpenLocked() error {
	if s.db != nil {
		return nil
	}
	if s.Path == "" {
		return fmt.Errorf("workspace path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	db, err := sql.Open("sqlite", sqliteDSN(s.Path))
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("ping workspace: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return fmt.Errorf("configure workspace: %w", err)
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return fmt.Errorf("create workspace schema: %w", err)
	}
	if _, err := db.Exec(`SELECT count(*) FROM meta`); err != nil {
		db.Close()
		return fmt.Errorf("unreadable workspace: %w", err)
	}
	s.db = db
	s.index = search.Attach(db)
	return nil
}

func (s *SQLiteStore) backupLocked() error {
	if s.db == nil {
		return nil
	}
	os.Remove(s.BackupPath())
	q := "VACUUM INTO " + quoteSQL(s.BackupPath())
	if _, err := s.db.Exec(q); err != nil {
		return fmt.Errorf("back up workspace: %w", err)
	}
	return nil
}

func (s *SQLiteStore) loadLocked() (domain.Workspace, error) {
	version, err := metaInt(s.db, "schema_version")
	if err != nil {
		return domain.Workspace{}, err
	}
	if version < 0 || version > CurrentSchemaVersion {
		return domain.Workspace{}, fmt.Errorf("unsupported workspace schema version %d", version)
	}
	var workspace domain.Workspace
	workspace.SchemaVersion = CurrentSchemaVersion
	if raw, ok, err := metaValue(s.db, "scope"); err != nil {
		return domain.Workspace{}, err
	} else if ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &workspace.Scope); err != nil {
			return domain.Workspace{}, fmt.Errorf("decode workspace scope: %w", err)
		}
	}
	if workspace.Library, err = loadJSONList[domain.WorldRef](s.db, entityWorld); err != nil {
		return domain.Workspace{}, err
	}
	if workspace.Sources, err = loadJSONList[domain.SourceDocument](s.db, entitySource); err != nil {
		return domain.Workspace{}, err
	}
	if workspace.Records, err = loadJSONList[domain.Record](s.db, entityRecord); err != nil {
		return domain.Workspace{}, err
	}
	if workspace.Sessions, err = loadJSONList[domain.SessionRecord](s.db, entitySession); err != nil {
		return domain.Workspace{}, err
	}
	if workspace.PlannedNotes, err = loadJSONList[domain.PlannedNotes](s.db, entityPlan); err != nil {
		return domain.Workspace{}, err
	}
	if workspace.Reconciliations, err = loadJSONList[domain.ReconciliationRecord](s.db, entityRecon); err != nil {
		return domain.Workspace{}, err
	}
	if workspace.Collections, err = loadJSONList[domain.Collection](s.db, entityCollection); err != nil {
		return domain.Workspace{}, err
	}
	workspace.EnsureLibrary()
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, fmt.Errorf("validate workspace: %w", err)
	}
	return workspace, nil
}

func (s *SQLiteStore) replaceLocked(workspace domain.Workspace) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin workspace write: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM entities WHERE kind != ?`, entityAdventure); err != nil {
		tx.Rollback()
		return fmt.Errorf("clear workspace: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM meta`); err != nil {
		tx.Rollback()
		return fmt.Errorf("clear workspace meta: %w", err)
	}
	if err := putMeta(tx, "schema_version", fmt.Sprintf("%d", CurrentSchemaVersion)); err != nil {
		tx.Rollback()
		return err
	}
	scopeJSON, err := json.Marshal(workspace.Scope)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("encode workspace scope: %w", err)
	}
	if err := putMeta(tx, "scope", string(scopeJSON)); err != nil {
		tx.Rollback()
		return err
	}
	writers := []func() error{
		func() error { return insertJSONList(tx, entityWorld, idsWorlds(workspace.Library), workspace.Library) },
		func() error {
			return insertJSONList(tx, entitySource, idsSources(workspace.Sources), workspace.Sources)
		},
		func() error {
			return insertJSONList(tx, entityRecord, idsRecords(workspace.Records), workspace.Records)
		},
		func() error {
			return insertJSONList(tx, entitySession, idsSessions(workspace.Sessions), workspace.Sessions)
		},
		func() error {
			return insertJSONList(tx, entityPlan, idsPlans(workspace.PlannedNotes), workspace.PlannedNotes)
		},
		func() error {
			return insertJSONList(tx, entityRecon, idsRecons(workspace.Reconciliations), workspace.Reconciliations)
		},
		func() error {
			return insertJSONList(tx, entityCollection, idsCollections(workspace.Collections), workspace.Collections)
		},
	}
	for _, write := range writers {
		if err := write(); err != nil {
			tx.Rollback()
			return err
		}
	}
	docs := search.DocumentsFromWorkspace(workspace)
	if err := search.WriteDocs(tx, docs); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workspace: %w", err)
	}
	return nil
}

func sqliteDSN(path string) string {
	return "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(DELETE)"
}

func quoteSQL(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func putMeta(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO meta (k, v) VALUES (?, ?)`, key, value)
	if err != nil {
		return fmt.Errorf("write workspace meta %s: %w", key, err)
	}
	return nil
}

func metaValue(db *sql.DB, key string) (string, bool, error) {
	var value string
	err := db.QueryRow(`SELECT v FROM meta WHERE k = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read workspace meta %s: %w", key, err)
	}
	return value, true, nil
}

func metaInt(db *sql.DB, key string) (int, error) {
	raw, ok, err := metaValue(db, key)
	if err != nil || !ok {
		return 0, err
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return 0, fmt.Errorf("workspace meta %s: %w", key, err)
	}
	return n, nil
}

func loadEntities(db *sql.DB, kind string) ([]Blob, error) {
	rows, err := db.Query(`SELECT id, payload FROM entities WHERE kind = ? ORDER BY ord`, kind)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", kind, err)
	}
	defer rows.Close()
	out := make([]Blob, 0)
	for rows.Next() {
		var blob Blob
		var payload string
		if err := rows.Scan(&blob.ID, &payload); err != nil {
			return nil, fmt.Errorf("scan %s: %w", kind, err)
		}
		blob.Data = []byte(payload)
		out = append(out, blob)
	}
	return out, rows.Err()
}

func loadJSONList[T any](db *sql.DB, kind string) ([]T, error) {
	blobs, err := loadEntities(db, kind)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(blobs))
	for _, blob := range blobs {
		var item T
		if err := json.Unmarshal(blob.Data, &item); err != nil {
			return nil, fmt.Errorf("decode %s %s: %w", kind, blob.ID, err)
		}
		out = append(out, item)
	}
	return out, nil
}

func insertJSONList[T any](tx *sql.Tx, kind string, ids []string, items []T) error {
	for i, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("encode %s: %w", kind, err)
		}
		id := ids[i]
		if _, err := tx.Exec(
			`INSERT INTO entities (kind, id, ord, payload) VALUES (?, ?, ?, ?)`,
			kind, id, i, string(payload),
		); err != nil {
			return fmt.Errorf("insert %s %s: %w", kind, id, err)
		}
	}
	return nil
}

func idsWorlds(items []domain.WorldRef) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func idsSources(items []domain.SourceDocument) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func idsRecords(items []domain.Record) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func idsSessions(items []domain.SessionRecord) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func idsPlans(items []domain.PlannedNotes) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func idsRecons(items []domain.ReconciliationRecord) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func idsCollections(items []domain.Collection) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}
