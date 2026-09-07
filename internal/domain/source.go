package domain

import (
	"fmt"
	"strings"
)

// SourceKind classifies an imported library document.
type SourceKind string

const (
	SourceBestiary  SourceKind = "bestiary"
	SourceAdventure SourceKind = "adventure"
	SourceRules     SourceKind = "rules"
)

// SourceDocument is a library-level book. Campaigns enable documents; wiki
// records cite SourceID rather than cloning the book into every campaign.
type SourceDocument struct {
	ID      string
	Title   string
	Edition string
	Kind    SourceKind
	Path    string
}

func (d SourceDocument) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("source ID is required")
	}
	if d.Title == "" {
		return fmt.Errorf("source %q title is required", d.ID)
	}
	if d.Kind == "" {
		return fmt.Errorf("source %q kind is required", d.ID)
	}
	return nil
}

// EnabledSourceIDs returns source documents turned on for the active campaign.
func (w Workspace) EnabledSourceIDs(scope Scope) []string {
	for _, world := range w.Library {
		if world.ID != scope.WorldID {
			continue
		}
		for _, campaign := range world.Campaigns {
			if campaign.ID == scope.CampaignID {
				return campaign.EnabledSourceIDs
			}
		}
	}
	return nil
}

// RecordVisibleIn reports whether a record belongs to the campaign view:
// campaign-owned, world-shared, or imported from an enabled source.
func RecordVisibleIn(record Record, scope Scope, enabled []string) bool {
	if record.Scope.CampaignID == scope.CampaignID && scope.CampaignID != "" {
		return true
	}
	if record.Scope.CampaignID == "" && record.Scope.WorldID == scope.WorldID && scope.WorldID != "" {
		return true
	}
	if record.SourceID == "" {
		return false
	}
	for _, id := range enabled {
		if id == record.SourceID {
			return true
		}
	}
	return false
}

// EnableSource adds sourceID to the named campaign if it is not already listed.
func (w *Workspace) EnableSource(worldID, campaignID, sourceID string) {
	if sourceID == "" {
		return
	}
	for i := range w.Library {
		if w.Library[i].ID != worldID {
			continue
		}
		for j := range w.Library[i].Campaigns {
			if w.Library[i].Campaigns[j].ID != campaignID {
				continue
			}
			for _, existing := range w.Library[i].Campaigns[j].EnabledSourceIDs {
				if existing == sourceID {
					return
				}
			}
			w.Library[i].Campaigns[j].EnabledSourceIDs = append(w.Library[i].Campaigns[j].EnabledSourceIDs, sourceID)
			return
		}
	}
}

// DisableSource removes sourceID from the named campaign's enabled set.
func (w *Workspace) DisableSource(worldID, campaignID, sourceID string) {
	if sourceID == "" {
		return
	}
	for i := range w.Library {
		if w.Library[i].ID != worldID {
			continue
		}
		for j := range w.Library[i].Campaigns {
			if w.Library[i].Campaigns[j].ID != campaignID {
				continue
			}
			next := make([]string, 0, len(w.Library[i].Campaigns[j].EnabledSourceIDs))
			for _, existing := range w.Library[i].Campaigns[j].EnabledSourceIDs {
				if existing != sourceID {
					next = append(next, existing)
				}
			}
			w.Library[i].Campaigns[j].EnabledSourceIDs = next
			return
		}
	}
}

// SourceEnabled reports whether the campaign currently enables sourceID.
func (w Workspace) SourceEnabled(scope Scope, sourceID string) bool {
	for _, id := range w.EnabledSourceIDs(scope) {
		if id == sourceID {
			return true
		}
	}
	return false
}

// UpsertSource replaces a source document with the same ID, or appends it.
func (w *Workspace) UpsertSource(doc SourceDocument) {
	for i := range w.Sources {
		if w.Sources[i].ID == doc.ID {
			w.Sources[i] = doc
			return
		}
	}
	w.Sources = append(w.Sources, doc)
}

// FindSource returns a library document by ID, or by unique title match.
func (w Workspace) FindSource(idOrTitle string) (SourceDocument, bool) {
	q := strings.TrimSpace(idOrTitle)
	if q == "" {
		return SourceDocument{}, false
	}
	for _, doc := range w.Sources {
		if doc.ID == q {
			return doc, true
		}
	}
	var match SourceDocument
	n := 0
	for _, doc := range w.Sources {
		if strings.EqualFold(doc.Title, q) {
			match = doc
			n++
		}
	}
	if n == 1 {
		return match, true
	}
	return SourceDocument{}, false
}

// RemoveSource deletes a library document and the records/prep ingested from
// it, and disables it in every campaign. Session Links keep their record IDs
// so a later ingest of the same book can restore CAST.
func (w *Workspace) RemoveSource(sourceID string) bool {
	if sourceID == "" {
		return false
	}
	kept := make([]SourceDocument, 0, len(w.Sources))
	found := false
	for _, doc := range w.Sources {
		if doc.ID == sourceID {
			found = true
			continue
		}
		kept = append(kept, doc)
	}
	if !found {
		return false
	}
	w.Sources = kept
	w.StripSourceContent(sourceID)
	for i := range w.Library {
		for j := range w.Library[i].Campaigns {
			w.DisableSource(w.Library[i].ID, w.Library[i].Campaigns[j].ID, sourceID)
		}
	}
	return true
}

// StripSourceContent removes records and planned notes previously ingested
// from sourceID so a re-import can replace them.
func (w *Workspace) StripSourceContent(sourceID string) {
	if sourceID == "" {
		return
	}
	records := make([]Record, 0, len(w.Records))
	for _, record := range w.Records {
		if record.SourceID != sourceID {
			records = append(records, record)
		}
	}
	w.Records = records
	plans := make([]PlannedNotes, 0, len(w.PlannedNotes))
	for _, plan := range w.PlannedNotes {
		if plan.SourceID != sourceID {
			plans = append(plans, plan)
		}
	}
	w.PlannedNotes = plans
}
