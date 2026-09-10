package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	// synced mirrors what the open database already holds. While it is
	// present a save writes only the rows that differ from it; a nil value
	// means the contents are unknown and the next save rewrites everything.
	synced *snapshot
	// backedUp is when this store last copied the database aside. A backup
	// costs a copy of the whole file, so it is taken once per open and then
	// throttled rather than repeated for every keystroke of live capture.
	backedUp time.Time
}

// backupInterval bounds how stale the sidecar backup may be. Writes are
// transactional, so the backup guards against a corrupt file or a mistaken
// bulk edit rather than against a torn write, and does not need to track
// every transcript line.
const backupInterval = 5 * time.Minute

// entityRow fingerprints one persisted row without keeping its payload.
type entityRow struct {
	ord int
	sum [32]byte
}

// snapshot records the rows and search documents last written by this store.
type snapshot struct {
	entities map[string]map[string]entityRow
	meta     map[string]string
	docs     search.DocIndex
}

func digest(payload []byte) [32]byte {
	return sha256.Sum256(payload)
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
	s.synced = nil
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

// Save persists the workspace. Once this store knows what the database holds
// it writes only the rows that changed, so a save during live capture costs
// the entry that was added rather than the whole campaign (D-045).
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
	return s.syncLocked(workspace)
}

func (s *SQLiteStore) ExportTo(path string) error {
	ws, err := s.Load()
	if err != nil {
		return err
	}
	return NewJSON(path).Save(ws)
}

// Replace overwrites the workspace with the given one, even when the current
// database cannot be read. Callers that already hold a validated workspace
// (a git sync pull, for instance) use this instead of round-tripping a file.
func (s *SQLiteStore) Replace(workspace domain.Workspace) error {
	return s.write(workspace, true)
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
	return s.Replace(ws)
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
	// Replace rebuilds the docs table outside this store's bookkeeping, so
	// what it now holds is no longer described by the snapshot.
	if s.synced != nil {
		s.synced.docs = nil
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

// backupLocked copies the database aside, at most once per backupInterval.
// Live capture saves after every entry, and a whole-file copy per entry made
// the cost of one typed line scale with the size of the campaign.
func (s *SQLiteStore) backupLocked() error {
	if s.db == nil {
		return nil
	}
	if !s.backedUp.IsZero() && time.Since(s.backedUp) < backupInterval {
		return nil
	}
	os.Remove(s.BackupPath())
	q := "VACUUM INTO " + quoteSQL(s.BackupPath())
	if _, err := s.db.Exec(q); err != nil {
		return fmt.Errorf("back up workspace: %w", err)
	}
	s.backedUp = time.Now()
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

// encodedKind is one entity kind rendered as the rows it persists, in order.
type encodedKind struct {
	kind string
	rows []encodedRow
}

type encodedRow struct {
	id      string
	payload []byte
	sum     [32]byte
}

// encodeWorkspace renders every persisted list once, so a save marshals the
// workspace a single time whether it ends up writing one row or all of them.
func encodeWorkspace(workspace domain.Workspace) ([]encodedKind, error) {
	var kinds []encodedKind
	var err error
	if kinds, err = appendKind(kinds, entityWorld, idsWorlds(workspace.Library), workspace.Library); err != nil {
		return nil, err
	}
	if kinds, err = appendKind(kinds, entitySource, idsSources(workspace.Sources), workspace.Sources); err != nil {
		return nil, err
	}
	if kinds, err = appendKind(kinds, entityRecord, idsRecords(workspace.Records), workspace.Records); err != nil {
		return nil, err
	}
	if kinds, err = appendKind(kinds, entitySession, idsSessions(workspace.Sessions), workspace.Sessions); err != nil {
		return nil, err
	}
	if kinds, err = appendKind(kinds, entityPlan, idsPlans(workspace.PlannedNotes), workspace.PlannedNotes); err != nil {
		return nil, err
	}
	if kinds, err = appendKind(kinds, entityRecon, idsRecons(workspace.Reconciliations), workspace.Reconciliations); err != nil {
		return nil, err
	}
	if kinds, err = appendKind(kinds, entityCollection, idsCollections(workspace.Collections), workspace.Collections); err != nil {
		return nil, err
	}
	return kinds, nil
}

func appendKind[T any](kinds []encodedKind, kind string, ids []string, items []T) ([]encodedKind, error) {
	rows := make([]encodedRow, len(items))
	for i, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", kind, err)
		}
		rows[i] = encodedRow{id: ids[i], payload: payload, sum: digest(payload)}
	}
	return append(kinds, encodedKind{kind: kind, rows: rows}), nil
}

func encodeMeta(workspace domain.Workspace) (map[string]string, error) {
	scopeJSON, err := json.Marshal(workspace.Scope)
	if err != nil {
		return nil, fmt.Errorf("encode workspace scope: %w", err)
	}
	return map[string]string{
		"schema_version": fmt.Sprintf("%d", CurrentSchemaVersion),
		"scope":          string(scopeJSON),
	}, nil
}

// syncLocked persists the workspace, writing only what changed when this store
// already knows what the database holds.
//
// The whole workspace used to be deleted and reinserted on every save, which
// put the cost of one captured transcript line in proportion to the size of
// the campaign. The snapshot makes the common save touch a single row.
func (s *SQLiteStore) syncLocked(workspace domain.Workspace) error {
	kinds, err := encodeWorkspace(workspace)
	if err != nil {
		return err
	}
	meta, err := encodeMeta(workspace)
	if err != nil {
		return err
	}
	docs := search.DocumentsFromWorkspace(workspace)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin workspace write: %w", err)
	}
	next, err := s.writeTx(tx, kinds, meta, docs)
	if err != nil {
		tx.Rollback()
		// The rollback leaves the database as the old snapshot described,
		// but a partially applied plan is not worth trusting; the next save
		// rebuilds from scratch.
		s.synced = nil
		return err
	}
	if err := tx.Commit(); err != nil {
		s.synced = nil
		return fmt.Errorf("commit workspace: %w", err)
	}
	s.synced = next
	return nil
}

func (s *SQLiteStore) writeTx(tx *sql.Tx, kinds []encodedKind, meta map[string]string, docs []search.Document) (*snapshot, error) {
	if s.synced == nil {
		return rewriteTx(tx, kinds, meta, docs)
	}
	return deltaTx(tx, s.synced, kinds, meta, docs)
}

// rewriteTx replaces every persisted row. It runs when the store has no
// snapshot: a freshly opened database, or one left in an unknown state.
func rewriteTx(tx *sql.Tx, kinds []encodedKind, meta map[string]string, docs []search.Document) (*snapshot, error) {
	if _, err := tx.Exec(`DELETE FROM entities WHERE kind != ?`, entityAdventure); err != nil {
		return nil, fmt.Errorf("clear workspace: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM meta`); err != nil {
		return nil, fmt.Errorf("clear workspace meta: %w", err)
	}
	next := &snapshot{
		entities: make(map[string]map[string]entityRow, len(kinds)),
		meta:     make(map[string]string, len(meta)),
	}
	for key, value := range meta {
		if err := putMeta(tx, key, value); err != nil {
			return nil, err
		}
		next.meta[key] = value
	}
	for _, kind := range kinds {
		held := make(map[string]entityRow, len(kind.rows))
		for i, row := range kind.rows {
			if err := insertEntity(tx, kind.kind, row, i); err != nil {
				return nil, err
			}
			held[row.id] = entityRow{ord: i, sum: row.sum}
		}
		next.entities[kind.kind] = held
	}
	index, err := search.WriteDocs(tx, docs)
	if err != nil {
		return nil, err
	}
	next.docs = index
	return next, nil
}

// deltaTx writes only the rows that differ from what prev records.
func deltaTx(tx *sql.Tx, prev *snapshot, kinds []encodedKind, meta map[string]string, docs []search.Document) (*snapshot, error) {
	next := &snapshot{
		entities: make(map[string]map[string]entityRow, len(kinds)),
		meta:     make(map[string]string, len(meta)),
	}
	for key, value := range meta {
		if prev.meta[key] != value {
			if _, err := tx.Exec(`INSERT INTO meta (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`, key, value); err != nil {
				return nil, fmt.Errorf("write workspace meta %s: %w", key, err)
			}
		}
		next.meta[key] = value
	}
	for key := range prev.meta {
		if _, ok := meta[key]; ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM meta WHERE k = ?`, key); err != nil {
			return nil, fmt.Errorf("clear workspace meta %s: %w", key, err)
		}
	}
	for _, kind := range kinds {
		held, err := syncKind(tx, prev.entities[kind.kind], kind)
		if err != nil {
			return nil, err
		}
		next.entities[kind.kind] = held
	}
	// A kind that disappeared entirely from the workspace still has rows.
	for kind, held := range prev.entities {
		if _, ok := next.entities[kind]; ok {
			continue
		}
		for id := range held {
			if err := deleteEntity(tx, kind, id); err != nil {
				return nil, err
			}
		}
	}
	index, err := search.SyncDocs(tx, docs, prev.docs)
	if err != nil {
		return nil, err
	}
	next.docs = index
	return next, nil
}

func syncKind(tx *sql.Tx, prev map[string]entityRow, kind encodedKind) (map[string]entityRow, error) {
	held := make(map[string]entityRow, len(kind.rows))
	for i, row := range kind.rows {
		if _, duplicate := held[row.id]; duplicate {
			return nil, fmt.Errorf("duplicate %s id %s", kind.kind, row.id)
		}
		held[row.id] = entityRow{ord: i, sum: row.sum}
		if old, ok := prev[row.id]; ok {
			if old.sum == row.sum && old.ord == i {
				continue
			}
			if _, err := tx.Exec(
				`UPDATE entities SET ord = ?, payload = ? WHERE kind = ? AND id = ?`,
				i, string(row.payload), kind.kind, row.id,
			); err != nil {
				return nil, fmt.Errorf("update %s %s: %w", kind.kind, row.id, err)
			}
			continue
		}
		if err := insertEntity(tx, kind.kind, row, i); err != nil {
			return nil, err
		}
	}
	for id := range prev {
		if _, ok := held[id]; ok {
			continue
		}
		if err := deleteEntity(tx, kind.kind, id); err != nil {
			return nil, err
		}
	}
	return held, nil
}

func insertEntity(tx *sql.Tx, kind string, row encodedRow, ord int) error {
	if _, err := tx.Exec(
		`INSERT INTO entities (kind, id, ord, payload) VALUES (?, ?, ?, ?)`,
		kind, row.id, ord, string(row.payload),
	); err != nil {
		return fmt.Errorf("insert %s %s: %w", kind, row.id, err)
	}
	return nil
}

func deleteEntity(tx *sql.Tx, kind, id string) error {
	if _, err := tx.Exec(`DELETE FROM entities WHERE kind = ? AND id = ?`, kind, id); err != nil {
		return fmt.Errorf("delete %s %s: %w", kind, id, err)
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
