package domain

import (
	"fmt"
	"strings"
)

// Collection is a named set of wiki records. It is a secondary organization
// aid on top of tags; records stay in the campaign tree.
type Collection struct {
	ID        string
	Title     string
	Scope     Scope
	RecordIDs []string
}

func (c Collection) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("collection ID is required")
	}
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("collection %q title is required", c.ID)
	}
	return nil
}

func (c Collection) Has(recordID string) bool {
	if recordID == "" {
		return false
	}
	for _, id := range c.RecordIDs {
		if id == recordID {
			return true
		}
	}
	return false
}

func (c Collection) Add(recordID string) Collection {
	if recordID == "" || c.Has(recordID) {
		return c
	}
	c.RecordIDs = append(append([]string(nil), c.RecordIDs...), recordID)
	return c
}

func (c Collection) Remove(recordID string) Collection {
	if recordID == "" {
		return c
	}
	out := make([]string, 0, len(c.RecordIDs))
	for _, id := range c.RecordIDs {
		if id == recordID {
			continue
		}
		out = append(out, id)
	}
	c.RecordIDs = out
	return c
}

func (c Collection) Toggle(recordID string) Collection {
	if c.Has(recordID) {
		return c.Remove(recordID)
	}
	return c.Add(recordID)
}

// ScopedCollections returns collections in the given world/campaign.
func ScopedCollections(collections []Collection, scope Scope) []Collection {
	out := make([]Collection, 0)
	for _, col := range collections {
		if col.Scope.WorldID != scope.WorldID || col.Scope.CampaignID != scope.CampaignID {
			continue
		}
		out = append(out, col)
	}
	return out
}

// CollectionsContaining lists collections in scope that include recordID.
func CollectionsContaining(collections []Collection, scope Scope, recordID string) []Collection {
	out := make([]Collection, 0)
	for _, col := range ScopedCollections(collections, scope) {
		if col.Has(recordID) {
			out = append(out, col)
		}
	}
	return out
}

func FindCollection(collections []Collection, id string) (Collection, bool) {
	for _, col := range collections {
		if col.ID == id {
			return col, true
		}
	}
	return Collection{}, false
}
