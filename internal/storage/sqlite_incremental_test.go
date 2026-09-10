package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/search"
)

func benchScope() domain.Scope {
	return domain.Scope{WorldID: "world", WorldName: "World", CampaignID: "campaign", Campaign: "Campaign"}
}

// growingWorkspace builds a campaign of the given size with one live sit, so a
// save can be measured against workspaces that differ only in how much they
// already hold.
func growingWorkspace(t testing.TB, records int) domain.Workspace {
	t.Helper()
	scope := benchScope()
	items := make([]domain.Record, records)
	for i := range items {
		items[i] = domain.Record{
			ID: fmt.Sprintf("npc-%d", i), Type: domain.NPC,
			Title: fmt.Sprintf("Villager %d", i), Authority: domain.Draft,
			Scope: scope, Body: "A villager of no particular consequence.",
		}
	}
	ws, err := domain.NewWorkspace(scope, items)
	if err != nil {
		t.Fatal(err)
	}
	ws.Sessions = []domain.SessionRecord{{
		ID: "sit-1", Title: "Session One", Scope: scope,
		StartedAt: time.Now(), LocationName: "The Road",
	}}
	return ws
}

func appendEntry(ws domain.Workspace, n int) domain.Workspace {
	entries := append([]domain.TranscriptEntry{}, ws.Sessions[0].Entries...)
	entries = append(entries, domain.TranscriptEntry{
		ID: fmt.Sprintf("entry-%d", n), Text: fmt.Sprintf("The party pressed on, %d.", n),
		CreatedAt: time.Now(),
	})
	sessions := append([]domain.SessionRecord{}, ws.Sessions...)
	sessions[0].Entries = entries
	ws.Sessions = sessions
	return ws
}

func openStore(t testing.TB) *SQLiteStore {
	t.Helper()
	store := NewSQLite(filepath.Join(t.TempDir(), "workspace.sqlite"))
	t.Cleanup(store.Close)
	return store
}

// rowIDs snapshots the physical identity of every persisted row. A row that is
// deleted and reinserted gets a new rowid, so a stable set proves the save
// left it alone rather than rewriting it.
func rowIDs(t testing.TB, db *sql.DB, query string) map[string]int64 {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var key string
		var id int64
		if err := rows.Scan(&key, &id); err != nil {
			t.Fatal(err)
		}
		out[key] = id
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

const entityRowIDs = `SELECT kind || '/' || id, rowid FROM entities`
const docRowIDs = `SELECT kind || '/' || id, rowid FROM docs`

func TestIncrementalSaveLeavesUnchangedRowsInPlace(t *testing.T) {
	store := openStore(t)
	ws := growingWorkspace(t, 25)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	beforeEntities := rowIDs(t, store.db, entityRowIDs)
	beforeDocs := rowIDs(t, store.db, docRowIDs)

	if err := store.Save(appendEntry(ws, 1)); err != nil {
		t.Fatal(err)
	}
	afterEntities := rowIDs(t, store.db, entityRowIDs)
	for key, id := range beforeEntities {
		if afterEntities[key] != id {
			t.Fatalf("%s was rewritten (rowid %d -> %d); appending a transcript entry must not touch it", key, id, afterEntities[key])
		}
	}
	afterDocs := rowIDs(t, store.db, docRowIDs)
	for key, id := range beforeDocs {
		if afterDocs[key] != id {
			t.Fatalf("search document %s was rebuilt (rowid %d -> %d)", key, id, afterDocs[key])
		}
	}
	if len(afterDocs) != len(beforeDocs)+1 {
		t.Fatalf("expected one added search document, got %d -> %d", len(beforeDocs), len(afterDocs))
	}
}

func TestIncrementalSaveRoundTripsEdits(t *testing.T) {
	store := openStore(t)
	ws := growingWorkspace(t, 5)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	// Change one record, drop another, reorder the rest, and add an entry.
	ws.Records[0].Title = "Villager Renamed"
	ws.Records = append(ws.Records[:1], ws.Records[2:]...)
	ws.Records[1], ws.Records[2] = ws.Records[2], ws.Records[1]
	ws = appendEntry(ws, 1)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	store.Close()

	reopened := NewSQLite(store.Path)
	defer reopened.Close()
	got, err := reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != len(ws.Records) {
		t.Fatalf("expected %d records, got %d", len(ws.Records), len(got.Records))
	}
	for i, want := range ws.Records {
		if got.Records[i].ID != want.ID || got.Records[i].Title != want.Title {
			t.Fatalf("record %d: got %s/%s, want %s/%s", i, got.Records[i].ID, got.Records[i].Title, want.ID, want.Title)
		}
	}
	if len(got.Sessions[0].Entries) != 1 || got.Sessions[0].Entries[0].ID != "entry-1" {
		t.Fatalf("transcript entry did not persist: %#v", got.Sessions[0].Entries)
	}
}

func TestIncrementalSaveKeepsSearchIndexInStep(t *testing.T) {
	store := openStore(t)
	ws := growingWorkspace(t, 3)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	find := func(query string) []search.Result {
		return store.Search().Find(search.Filter{
			Query: query, Scope: search.CurrentCampaign,
			WorldID: ws.Scope.WorldID, CampaignID: ws.Scope.CampaignID,
		})
	}
	if len(find("Villager")) == 0 {
		t.Fatal("expected the seeded villagers to be searchable")
	}
	ws.Records[0].Title = "Harbinger"
	ws.Records[0].Body = "Newly named."
	ws.Records = ws.Records[:1]
	ws = appendEntry(ws, 1)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	if hits := find("Harbinger"); len(hits) != 1 {
		t.Fatalf("renamed record is not searchable: %#v", hits)
	}
	if hits := find("Villager"); len(hits) != 0 {
		t.Fatalf("removed records still in the index: %#v", hits)
	}
	if hits := find("pressed"); len(hits) != 1 {
		t.Fatalf("new transcript entry is not searchable: %#v", hits)
	}
}

func TestSaveAfterReopenRebuildsFromUnknownState(t *testing.T) {
	store := openStore(t)
	ws := growingWorkspace(t, 3)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if store.synced != nil {
		t.Fatal("closing must forget what the database held")
	}
	ws.Records[0].Title = "Rewritten"
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Records[0].Title != "Rewritten" {
		t.Fatalf("expected the full rewrite to land, got %q", got.Records[0].Title)
	}
}

func TestBackupIsNotTakenForEverySave(t *testing.T) {
	store := openStore(t)
	ws := growingWorkspace(t, 3)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(appendEntry(ws, 1)); err != nil {
		t.Fatal(err)
	}
	taken := store.backedUp
	if taken.IsZero() {
		t.Fatal("expected the first save over an existing file to back it up")
	}
	if err := store.Save(appendEntry(ws, 2)); err != nil {
		t.Fatal(err)
	}
	if !store.backedUp.Equal(taken) {
		t.Fatal("a backup per transcript entry copies the whole file; it must be throttled")
	}
	store.backedUp = taken.Add(-backupInterval)
	if err := store.Save(appendEntry(ws, 3)); err != nil {
		t.Fatal(err)
	}
	if !store.backedUp.After(taken) {
		t.Fatal("expected a backup once the interval had elapsed")
	}
}

// BenchmarkAppendEntry measures the cost of capturing one transcript line
// against campaigns of different sizes. The whole-workspace save this replaced
// scaled with the record count; an incremental save should not.
func BenchmarkAppendEntry(b *testing.B) {
	for _, records := range []int{100, 1000} {
		b.Run(fmt.Sprintf("records=%d", records), func(b *testing.B) {
			store := openStore(b)
			ws := growingWorkspace(b, records)
			if err := store.Save(ws); err != nil {
				b.Fatal(err)
			}
			// Warm the snapshot so the measured saves are incremental.
			ws = appendEntry(ws, 0)
			if err := store.Save(ws); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ws = appendEntry(ws, i+1)
				if err := store.Save(ws); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestIncrementalSaveHandlesUndoneEntries(t *testing.T) {
	store := openStore(t)
	ws := growingWorkspace(t, 3)
	ws = appendEntry(ws, 1)
	ws = appendEntry(ws, 2)
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	find := func(query string) []search.Result {
		return store.Search().Find(search.Filter{
			Query: query, Scope: search.CurrentCampaign,
			WorldID: ws.Scope.WorldID, CampaignID: ws.Scope.CampaignID,
		})
	}
	if len(find("pressed")) != 2 {
		t.Fatalf("expected both entries indexed, got %d", len(find("pressed")))
	}
	// Undo drops the entry from the index but keeps it on the record, which
	// is the save that fires most often during live capture.
	sessions := append([]domain.SessionRecord{}, ws.Sessions...)
	entries := append([]domain.TranscriptEntry{}, sessions[0].Entries...)
	entries[1].Undone = true
	sessions[0].Entries = entries
	ws.Sessions = sessions
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	hits := find("pressed")
	if len(hits) != 1 || hits[0].ID != "entry-1" {
		t.Fatalf("undone entry is still searchable: %#v", hits)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions[0].Entries) != 2 || !got.Sessions[0].Entries[1].Undone {
		t.Fatalf("undone entry must stay on the transcript: %#v", got.Sessions[0].Entries)
	}
	// Redo puts it back.
	entries[1].Undone = false
	sessions[0].Entries = entries
	ws.Sessions = sessions
	if err := store.Save(ws); err != nil {
		t.Fatal(err)
	}
	if len(find("pressed")) != 2 {
		t.Fatalf("redo did not restore the entry to the index")
	}
}
