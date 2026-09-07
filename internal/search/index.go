package search

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hbaldwin98/dungeon/internal/domain"
	_ "modernc.org/sqlite"
)

const (
	maxQueryRunes    = 200
	maxReferenceHits = 8
	createDocsSQL    = `CREATE VIRTUAL TABLE docs USING fts5(
		title,
		body,
		tags,
		aliases,
		kind UNINDEXED,
		id UNINDEXED,
		session_id UNINDEXED,
		type_label UNINDEXED,
		authority UNINDEXED,
		world_id UNINDEXED,
		campaign_id UNINDEXED,
		source_id UNINDEXED,
		include_empty UNINDEXED,
		record_json UNINDEXED,
		snippet UNINDEXED
	);`
	selectDocsSQL = `SELECT kind, id, session_id, title, body, tags, aliases, type_label, authority, world_id, campaign_id, source_id, include_empty, record_json, snippet FROM docs WHERE 1=1`
)

// IndexPath places a sidecar search.sqlite next to a JSON workspace (tests).
func IndexPath(workspacePath string) string {
	return filepath.Join(filepath.Dir(workspacePath), "search.sqlite")
}

type Index struct {
	mu    sync.RWMutex
	db    *sql.DB
	path  string
	owned bool
}

func Open(path string, docs []Document) (*Index, error) {
	idx, err := openAndReplace(path, docs)
	if err == nil {
		return idx, nil
	}
	if path == "" {
		return nil, err
	}
	removeIndexFiles(path)
	return openAndReplace(path, docs)
}

func openAndReplace(path string, docs []Document) (*Index, error) {
	idx, err := openIndex(path)
	if err != nil {
		return nil, err
	}
	if err := idx.Replace(docs); err != nil {
		idx.Close()
		return nil, err
	}
	return idx, nil
}

func openIndex(path string) (*Index, error) {
	dsn := ":memory:"
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create search index directory: %w", err)
		}
		dsn = fileDSN(path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open search index: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping search index: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure search index: %w", err)
	}
	return &Index{db: db, path: path, owned: true}, nil
}

// Attach uses an already-open campaign database. Close does not close db.
func Attach(db *sql.DB) *Index {
	return &Index{db: db, owned: false}
}

func fileDSN(path string) string {
	return "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(DELETE)"
}

func (idx *Index) Close() {
	if idx == nil {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if !idx.owned || idx.db == nil {
		return
	}
	idx.db.Close()
	idx.db = nil
}

func (idx *Index) Replace(docs []Document) error {
	if idx == nil {
		return fmt.Errorf("search index is not open")
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.path == "" {
		return replaceDB(idx.db, docs)
	}
	return idx.replaceFile(docs)
}

func (idx *Index) replaceFile(docs []Document) error {
	tmp := idx.path + ".tmp"
	removeIndexFiles(tmp)
	db, err := sql.Open("sqlite", fileDSN(tmp))
	if err != nil {
		return fmt.Errorf("open search index temp: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := replaceDB(db, docs); err != nil {
		db.Close()
		removeIndexFiles(tmp)
		return err
	}
	if err := db.Close(); err != nil {
		removeIndexFiles(tmp)
		return fmt.Errorf("close search index temp: %w", err)
	}
	if idx.db != nil {
		idx.db.Close()
		idx.db = nil
	}
	if err := os.Rename(tmp, idx.path); err != nil {
		removeIndexFiles(tmp)
		reopened, openErr := sql.Open("sqlite", fileDSN(idx.path))
		if openErr == nil {
			reopened.SetMaxOpenConns(1)
			idx.db = reopened
		}
		return fmt.Errorf("replace search index: %w", err)
	}
	reopened, err := sql.Open("sqlite", fileDSN(idx.path))
	if err != nil {
		return fmt.Errorf("reopen search index: %w", err)
	}
	reopened.SetMaxOpenConns(1)
	idx.db = reopened
	return nil
}

func replaceDB(db *sql.DB, docs []Document) error {
	if db == nil {
		return fmt.Errorf("search index is not open")
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin search index write: %w", err)
	}
	if err := WriteDocs(tx, docs); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit search index: %w", err)
	}
	return nil
}

// WriteDocs replaces the FTS table inside an existing transaction so campaign
// rows and search can commit together.
func WriteDocs(tx *sql.Tx, docs []Document) error {
	if tx == nil {
		return fmt.Errorf("search index transaction is required")
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS docs`); err != nil {
		return fmt.Errorf("reset search index: %w", err)
	}
	if _, err := tx.Exec(createDocsSQL); err != nil {
		return fmt.Errorf("create search index: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO docs (
		title, body, tags, aliases, kind, id, session_id, type_label, authority,
		world_id, campaign_id, source_id, include_empty, record_json, snippet
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare search index insert: %w", err)
	}
	defer stmt.Close()
	for _, doc := range docs {
		recordJSON, err := marshalRecord(doc)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(
			doc.Title,
			doc.Body,
			strings.Join(doc.Tags, " "),
			strings.Join(doc.Aliases, " "),
			string(doc.Kind),
			doc.ID,
			doc.SessionID,
			doc.TypeLabel,
			string(doc.Authority),
			doc.WorldID,
			doc.CampaignID,
			doc.SourceID,
			emptyFlag(doc.IncludeEmpty),
			recordJSON,
			doc.Snippet,
		); err != nil {
			return fmt.Errorf("insert search document %s/%s: %w", doc.Kind, doc.ID, err)
		}
	}
	return nil
}

func marshalRecord(doc Document) (string, error) {
	if doc.Record.ID == "" {
		return "", nil
	}
	data, err := json.Marshal(doc.Record)
	if err != nil {
		return "", fmt.Errorf("encode search record %s: %w", doc.ID, err)
	}
	return string(data), nil
}

func emptyFlag(include bool) string {
	if include {
		return "1"
	}
	return "0"
}

func (idx *Index) Query(filter Filter) ([]Result, error) {
	if idx == nil {
		return nil, fmt.Errorf("search index is not open")
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if idx.db == nil {
		return nil, fmt.Errorf("search index is not open")
	}
	filter.Query = clampQuery(filter.Query)
	normalized := normalize(filter.Query)
	hits := map[string]Result{}
	if match := ftsMatchQuery(normalized); match != "" {
		if docs, err := idx.selectDocs(filter, match, false); err == nil {
			for _, doc := range docs {
				addHit(hits, doc, normalized, true)
			}
		}
	}
	docs, err := idx.selectDocs(filter, "", normalized != "")
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		addHit(hits, doc, normalized, false)
	}
	results := make([]Result, 0, len(hits))
	for _, result := range hits {
		results = append(results, result)
	}
	return results, nil
}

func addHit(hits map[string]Result, doc Document, query string, fts bool) {
	body := ""
	if fts || query == "" {
		body = doc.scoreBody()
	}
	score, ok := textScore(doc.Title, doc.Aliases, body, query)
	if !ok {
		if !fts {
			return
		}
		score = 200
	}
	key := doc.key()
	if current, exists := hits[key]; exists && current.Score >= score {
		return
	}
	hits[key] = doc.result(score, query)
}

func (idx *Index) selectDocs(filter Filter, match string, titlesOnly bool) ([]Document, error) {
	query, args := buildSelect(filter, match, titlesOnly)
	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query search index: %w", err)
	}
	defer rows.Close()
	docs := make([]Document, 0)
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		if !documentAllowed(doc, filter) {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func buildSelect(filter Filter, match string, titlesOnly bool) (string, []any) {
	var b strings.Builder
	b.WriteString(selectDocsSQL)
	args := make([]any, 0, 8)
	if match != "" {
		b.WriteString(` AND docs MATCH ?`)
		args = append(args, match)
	} else if strings.TrimSpace(filter.Query) == "" {
		b.WriteString(` AND include_empty = '1'`)
	} else if titlesOnly {
		b.WriteString(` AND kind != 'reference'`)
	}
	if !includeExtras(filter) {
		b.WriteString(` AND kind = 'record'`)
	}
	switch filter.Scope {
	case EntireLibrary:
	case CurrentWorld:
		b.WriteString(` AND world_id = ?`)
		args = append(args, filter.WorldID)
	default:
		b.WriteString(` AND (
			(campaign_id != '' AND campaign_id = ?)
			OR (campaign_id = '' AND world_id != '' AND world_id = ?)`)
		args = append(args, filter.CampaignID, filter.WorldID)
		if len(filter.EnabledSourceIDs) > 0 {
			b.WriteString(` OR (source_id != '' AND source_id IN (` + placeholders(len(filter.EnabledSourceIDs)) + `))`)
			for _, id := range filter.EnabledSourceIDs {
				args = append(args, id)
			}
		}
		b.WriteString(`)`)
	}
	if !filter.IncludeProposals {
		b.WriteString(` AND authority != ?`)
		args = append(args, string(domain.Proposal))
	}
	return b.String(), args
}

func placeholders(n int) string {
	if n < 1 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func scanDocument(rows *sql.Rows) (Document, error) {
	var (
		doc          Document
		kind         string
		tags         string
		aliases      string
		authority    string
		includeEmpty string
		recordJSON   string
	)
	if err := rows.Scan(
		&kind, &doc.ID, &doc.SessionID, &doc.Title, &doc.Body, &tags, &aliases,
		&doc.TypeLabel, &authority, &doc.WorldID, &doc.CampaignID, &doc.SourceID,
		&includeEmpty, &recordJSON, &doc.Snippet,
	); err != nil {
		return Document{}, fmt.Errorf("scan search document: %w", err)
	}
	doc.Kind = Kind(kind)
	doc.Authority = domain.Authority(authority)
	doc.IncludeEmpty = includeEmpty == "1"
	if tags != "" {
		doc.Tags = strings.Fields(tags)
	}
	if aliases != "" {
		doc.Aliases = strings.Fields(aliases)
	}
	if recordJSON != "" {
		if err := json.Unmarshal([]byte(recordJSON), &doc.Record); err != nil {
			return Document{}, fmt.Errorf("decode search record %s: %w", doc.ID, err)
		}
		doc.Aliases = doc.Record.Aliases
		doc.Tags = doc.Record.Tags
	}
	return doc, nil
}

func documentAllowed(doc Document, filter Filter) bool {
	if doc.Kind == KindRecord {
		rec := doc.scopeRecord()
		return typeAllowed(rec, filter.Types) && tagsAllowed(rec, filter.Tags)
	}
	if !includeExtras(filter) {
		return false
	}
	if doc.Kind == KindReference {
		return inScope(doc.scopeRecord(), filter)
	}
	return true
}

func ftsMatchQuery(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.ReplaceAll(field, `"`, "")
		if field == "" {
			continue
		}
		parts = append(parts, `"`+field+`"`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

func clampQuery(query string) string {
	runes := []rune(query)
	if len(runes) <= maxQueryRunes {
		return query
	}
	return string(runes[:maxQueryRunes])
}

func removeIndexFiles(path string) {
	if path == "" {
		return
	}
	os.Remove(path)
	os.Remove(path + "-wal")
	os.Remove(path + "-shm")
	os.Remove(path + ".tmp")
}
