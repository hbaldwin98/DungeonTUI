// Package gitsync mirrors the local workspace into a git repository so the
// owner can back it up and carry it between machines. The repository holds
// one JSON file per entity: a plain export is one enormous blob, and a
// per-entity tree lets git show a readable diff and merge edits that touched
// different records (D-046).
package gitsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

// workspaceDir is the subdirectory of the repository this package owns. Files
// outside it are the owner's own (a README, notes) and are never touched.
const workspaceDir = "workspace"

const (
	dirWorlds      = "worlds"
	dirSources     = "sources"
	dirRecords     = "records"
	dirSessions    = "sessions"
	dirPlans       = "plans"
	dirRecons      = "reconciliations"
	dirCollections = "collections"
	dirAdventures  = "adventures"
)

// metaFile carries what is not a list: the schema version and active scope.
type metaFile struct {
	SchemaVersion int          `json:"schemaVersion"`
	Scope         domain.Scope `json:"scope"`
}

// Snapshot is the workspace as the repository stores it.
type Snapshot struct {
	Workspace  domain.Workspace
	Adventures []storage.Blob
}

// Write replaces the workspace tree under root with the snapshot. Entity files
// that no longer correspond to anything are removed, so a delete travels.
func Write(root string, snap Snapshot) error {
	base := filepath.Join(root, workspaceDir)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	files, err := snapshotFiles(snap)
	if err != nil {
		return err
	}
	if err := writeFiles(base, files); err != nil {
		return err
	}
	return pruneFiles(base, files)
}

// snapshotFiles renders the snapshot as repository-relative paths and bodies.
func snapshotFiles(snap Snapshot) (map[string][]byte, error) {
	ws := snap.Workspace
	meta, err := encode(metaFile{SchemaVersion: ws.SchemaVersion, Scope: ws.Scope})
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"meta.json": meta}
	adders := []func(map[string][]byte) error{
		func(f map[string][]byte) error { return addAll(f, dirWorlds, ws.Library, worldID) },
		func(f map[string][]byte) error { return addAll(f, dirSources, ws.Sources, sourceID) },
		func(f map[string][]byte) error { return addAll(f, dirRecords, ws.Records, recordID) },
		func(f map[string][]byte) error { return addAll(f, dirSessions, ws.Sessions, sessionID) },
		func(f map[string][]byte) error { return addAll(f, dirPlans, ws.PlannedNotes, planID) },
		func(f map[string][]byte) error { return addAll(f, dirRecons, ws.Reconciliations, reconID) },
		func(f map[string][]byte) error { return addAll(f, dirCollections, ws.Collections, collectionID) },
	}
	for _, add := range adders {
		if err := add(files); err != nil {
			return nil, err
		}
	}
	addAdventures(files, snap.Adventures)
	return files, nil
}

func addAdventures(files map[string][]byte, books []storage.Blob) {
	for _, book := range books {
		files[path(dirAdventures, book.ID)] = book.Data
	}
}

// addAll encodes one entity list into the file set, refusing duplicate names
// rather than silently dropping an entity.
func addAll[T any](files map[string][]byte, dir string, items []T, id func(T) string) error {
	for _, item := range items {
		data, err := encode(item)
		if err != nil {
			return fmt.Errorf("encode %s: %w", dir, err)
		}
		name := path(dir, id(item))
		if _, taken := files[name]; taken {
			return fmt.Errorf("duplicate %s id %q", dir, id(item))
		}
		files[name] = data
	}
	return nil
}

func worldID(w domain.WorldRef) string             { return w.ID }
func sourceID(s domain.SourceDocument) string      { return s.ID }
func recordID(r domain.Record) string              { return r.ID }
func sessionID(s domain.SessionRecord) string      { return s.ID }
func planID(p domain.PlannedNotes) string          { return p.ID }
func reconID(r domain.ReconciliationRecord) string { return r.ID }
func collectionID(c domain.Collection) string      { return c.ID }

func encode(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// path names the file for one entity. The id lives inside the file, so the
// name only has to be stable, filesystem-safe, and unique; ids that are not
// both keep a readable prefix and gain a digest suffix.
func path(dir, id string) string {
	return filepath.Join(dir, fileName(id)+".json")
}

func fileName(id string) string {
	safe := sanitize(id)
	if safe == id && len(id) <= 96 {
		return safe
	}
	sum := sha256.Sum256([]byte(id))
	return trim(safe, 64) + "-" + hex.EncodeToString(sum[:4])
}

func sanitize(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

func trim(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func writeFiles(base string, files map[string][]byte) error {
	for name, data := range files {
		full := filepath.Join(base, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(name), err)
		}
		if err := os.WriteFile(full, data, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// pruneFiles deletes workspace files the snapshot no longer contains.
func pruneFiles(base string, files map[string][]byte) error {
	return filepath.Walk(base, func(full string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(base, full)
		if relErr != nil {
			return relErr
		}
		if _, kept := files[rel]; kept {
			return nil
		}
		return os.Remove(full)
	})
}

// Read loads the snapshot a repository holds. A missing meta file means the
// repository has never been written to by Dungeon.
func Read(root string) (Snapshot, error) {
	base := filepath.Join(root, workspaceDir)
	var meta metaFile
	if err := readJSON(filepath.Join(base, "meta.json"), &meta); err != nil {
		return Snapshot{}, err
	}
	ws := domain.Workspace{SchemaVersion: meta.SchemaVersion, Scope: meta.Scope}
	if err := readLists(base, &ws); err != nil {
		return Snapshot{}, err
	}
	books, err := readAdventures(base)
	if err != nil {
		return Snapshot{}, err
	}
	ws.EnsureLibrary()
	if err := ws.Validate(); err != nil {
		return Snapshot{}, fmt.Errorf("validate synced workspace: %w", err)
	}
	return Snapshot{Workspace: ws, Adventures: books}, nil
}

func readLists(base string, ws *domain.Workspace) error {
	readers := []func() error{
		func() (err error) { ws.Library, err = readDir[domain.WorldRef](base, dirWorlds); return },
		func() (err error) { ws.Sources, err = readDir[domain.SourceDocument](base, dirSources); return },
		func() (err error) { ws.Records, err = readDir[domain.Record](base, dirRecords); return },
		func() (err error) { ws.Sessions, err = readDir[domain.SessionRecord](base, dirSessions); return },
		func() (err error) { ws.PlannedNotes, err = readDir[domain.PlannedNotes](base, dirPlans); return },
		func() (err error) {
			ws.Reconciliations, err = readDir[domain.ReconciliationRecord](base, dirRecons)
			return
		},
		func() (err error) { ws.Collections, err = readDir[domain.Collection](base, dirCollections); return },
	}
	for _, read := range readers {
		if err := read(); err != nil {
			return err
		}
	}
	return nil
}

// readDir decodes every file in one entity directory, in file-name order. The
// stored order is presentational and the browser sorts what it shows, so a
// stable name order is enough to make a pull deterministic.
func readDir[T any](base, dir string) ([]T, error) {
	names, err := entryNames(filepath.Join(base, dir))
	if err != nil {
		return nil, err
	}
	items := make([]T, 0, len(names))
	for _, name := range names {
		var item T
		if err := readJSON(filepath.Join(base, dir, name), &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func readAdventures(base string) ([]storage.Blob, error) {
	dir := filepath.Join(base, dirAdventures)
	names, err := entryNames(dir)
	if err != nil {
		return nil, err
	}
	books := make([]storage.Blob, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		books = append(books, storage.Blob{ID: adventureID(data, name), Data: data})
	}
	return books, nil
}

// adventureID recovers the cached book id from its payload; the file name is
// only a sanitized form of it.
func adventureID(data []byte, name string) string {
	var head struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(data, &head); err == nil && head.ID != "" {
		return head.ID
	}
	return strings.TrimSuffix(name, ".json")
}

func entryNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

// snapshotsEqual compares the repository form of two snapshots, so a nil
// entity list and an empty one are the same: neither writes any files.
func snapshotsEqual(a, b Snapshot) bool {
	left, err := snapshotFiles(a)
	if err != nil {
		return false
	}
	right, err := snapshotFiles(b)
	if err != nil {
		return false
	}
	if len(left) != len(right) {
		return false
	}
	for name, data := range left {
		if !bytes.Equal(data, right[name]) {
			return false
		}
	}
	return true
}

// hasCampaignData reports whether a snapshot holds anything the owner would
// miss if a pull replaced it. An unwritten first-run library is not precious.
func hasCampaignData(snap Snapshot) bool {
	ws := snap.Workspace
	return len(snap.Adventures) > 0 ||
		len(ws.Library) > 0 ||
		len(ws.Sources) > 0 ||
		len(ws.Records) > 0 ||
		len(ws.Sessions) > 0 ||
		len(ws.PlannedNotes) > 0 ||
		len(ws.Reconciliations) > 0 ||
		len(ws.Collections) > 0
}
