package gitsync

import (
	"errors"
	"fmt"
	"os"

	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

// SnapshotFrom reads the operational sqlite store as a git snapshot.
func SnapshotFrom(store *storage.SQLiteStore) (Snapshot, error) {
	if store == nil {
		return Snapshot{}, fmt.Errorf("workspace store is required")
	}
	ws, err := store.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}
	books, err := store.LoadAdventures()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Workspace: ws, Adventures: books}, nil
}

// Apply writes a pulled snapshot into the operational sqlite store.
func Apply(store *storage.SQLiteStore, snap Snapshot) error {
	if store == nil {
		return fmt.Errorf("workspace store is required")
	}
	if err := store.Save(snap.Workspace); err != nil {
		return err
	}
	return store.SaveAdventures(snap.Adventures)
}
