package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Authority string

const (
	Canon      Authority = "canon"
	Secret     Authority = "secret"
	Draft      Authority = "draft"
	Proposal   Authority = "proposal"
	Unknown    Authority = "unknown"
	Superseded Authority = "superseded"
	// Reference marks external material Dungeon does not own, such as 5e-cli
	// rules text. It is never campaign canon and is never written to the wiki.
	Reference Authority = "reference"
)

func (a Authority) Marker() string {
	switch a {
	case Canon:
		return "●"
	case Secret:
		return "◆"
	case Draft:
		return "◇"
	case Proposal:
		return "△"
	case Unknown:
		return "?"
	case Superseded:
		return "×"
	case Reference:
		return "§"
	default:
		return "?"
	}
}

func (a Authority) Label() string {
	switch a {
	case Canon:
		return "CANON"
	case Secret:
		return "SECRET"
	case Draft:
		return "DRAFT"
	case Proposal:
		return "AI PROPOSAL"
	case Unknown:
		return "UNKNOWN"
	case Superseded:
		return "SUPERSEDED"
	case Reference:
		return "REFERENCE"
	default:
		return "UNCLASSIFIED"
	}
}

type EntityType string

const (
	NPC       EntityType = "NPC"
	Character EntityType = "CHARACTER"
	Location  EntityType = "LOCATION"
	Faction   EntityType = "FACTION"
	Item      EntityType = "ITEM"
	Creature  EntityType = "CREATURE"
	Thread    EntityType = "THREAD"
	Session   EntityType = "SESSION"
	Scene     EntityType = "SCENE"
	Event     EntityType = "EVENT"
	Rule      EntityType = "RULE"
	Note      EntityType = "NOTE"
)

type Scope struct {
	WorldID    string
	WorldName  string
	CampaignID string
	Campaign   string
}

// CampaignRef names one campaign under a world in the library catalog.
type CampaignRef struct {
	ID               string
	Name             string
	EnabledSourceIDs []string // library source documents visible in this campaign
}

// WorldRef is a world entry in the library catalog (Obsidian-vault style).
type WorldRef struct {
	ID        string
	Name      string
	Campaigns []CampaignRef
}

func (s Scope) Label() string {
	if s.Campaign != "" {
		return s.Campaign
	}
	if s.WorldName != "" {
		return s.WorldName
	}
	return "Library"
}

type Record struct {
	ID          string
	Type        EntityType
	Title       string
	Summary     string
	Body        string
	Authority   Authority
	Scope       Scope
	Source      string
	SourceID    string // library SourceDocument ID when imported; empty if DM-authored
	Folder      string // optional slash path; imported books use Source/Type
	Aliases     []string
	Tags        []string
	IsAIContent bool
}

type EntityLink struct {
	Text     string
	RecordID string
}

// RollResult records a local # dice or arithmetic evaluation tied to a
// transcript entry. Results are session evidence and never become canon alone.
type RollResult struct {
	ID         string
	Label      string
	Expression string
	Total      int
	Detail     string
	Rolls      []int
}

type TranscriptEntry struct {
	ID        string
	Text      string
	CreatedAt time.Time
	Links     []EntityLink
	Rolls     []RollResult
	Revision  int
	Undone    bool
}

type SessionRecord struct {
	ID             string
	Title          string
	Scope          Scope
	StartedAt      time.Time
	EndedAt        *time.Time
	LocationID     string
	LocationName   string
	PlannedNotesID string       // optional prep notes that seeded this live sit
	Folder         string       // optional slash path; empty sits group by month
	Links          []EntityLink // durable present/cast; wiki records stay global
	Entries        []TranscriptEntry
}

func (r Record) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("record ID is required")
	}
	if r.Title == "" {
		return fmt.Errorf("record %q title is required", r.ID)
	}
	if r.Type == "" {
		return fmt.Errorf("record %q type is required", r.ID)
	}
	if r.IsAIContent && r.Authority != Proposal {
		return fmt.Errorf("AI record %q must have proposal authority", r.ID)
	}
	return nil
}

type Workspace struct {
	SchemaVersion   int
	Scope           Scope
	Library         []WorldRef
	Sources         []SourceDocument
	Records         []Record
	Sessions        []SessionRecord
	Reconciliations []ReconciliationRecord
	PlannedNotes    []PlannedNotes
	Collections     []Collection
}

// Clone returns a deep copy suitable for background work or rollback.
func (w Workspace) Clone() Workspace {
	data, _ := json.Marshal(w)
	var clone Workspace
	_ = json.Unmarshal(data, &clone)
	return clone
}

// Validate checks aggregate invariants required by every storage adapter.
// An empty active scope is valid while the library picker is open.
func (w Workspace) Validate() error {
	if err := validateLibrary(w.Library); err != nil {
		return err
	}
	if err := validateActiveScope(w); err != nil {
		return err
	}
	validators := []func() error{
		func() error { return validateUniqueRecords(w.Records) },
		func() error { return validateUniqueSources(w.Sources) },
		func() error { return validateUniqueSessions(w.Sessions) },
		func() error { return validateUniquePlans(w.PlannedNotes) },
		func() error { return validateUniqueCollections(w.Collections) },
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateLibrary(library []WorldRef) error {
	worldIDs := map[string]bool{}
	campaignIDs := map[string]bool{}
	for _, world := range library {
		if world.ID == "" || strings.TrimSpace(world.Name) == "" {
			return fmt.Errorf("world ID and name are required")
		}
		if worldIDs[world.ID] {
			return fmt.Errorf("duplicate world ID %q", world.ID)
		}
		worldIDs[world.ID] = true
		for _, campaign := range world.Campaigns {
			if campaign.ID == "" || strings.TrimSpace(campaign.Name) == "" {
				return fmt.Errorf("campaign ID and name are required in world %q", world.ID)
			}
			if campaignIDs[campaign.ID] {
				return fmt.Errorf("duplicate campaign ID %q", campaign.ID)
			}
			campaignIDs[campaign.ID] = true
		}
	}
	return nil
}

func validateActiveScope(w Workspace) error {
	if w.Scope.WorldID != "" || w.Scope.CampaignID != "" {
		if _, ok := w.ScopeFor(w.Scope.WorldID, w.Scope.CampaignID); !ok {
			return fmt.Errorf("active scope does not exist in library")
		}
	}
	return nil
}

func validateUniqueRecords(records []Record) error {
	ids := map[string]bool{}
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return err
		}
		if ids[record.ID] {
			return fmt.Errorf("duplicate record ID %q", record.ID)
		}
		ids[record.ID] = true
	}
	return nil
}

func validateUniqueSources(sources []SourceDocument) error {
	ids := map[string]bool{}
	for _, source := range sources {
		if err := source.Validate(); err != nil {
			return err
		}
		if ids[source.ID] {
			return fmt.Errorf("duplicate source ID %q", source.ID)
		}
		ids[source.ID] = true
	}
	return nil
}

func validateUniqueSessions(sessions []SessionRecord) error {
	ids := map[string]bool{}
	for _, session := range sessions {
		if session.ID == "" || strings.TrimSpace(session.Title) == "" || session.StartedAt.IsZero() {
			return fmt.Errorf("session ID, title, and start time are required")
		}
		if ids[session.ID] {
			return fmt.Errorf("duplicate session ID %q", session.ID)
		}
		ids[session.ID] = true
	}
	return nil
}

func validateUniquePlans(plans []PlannedNotes) error {
	ids := map[string]bool{}
	for _, plan := range plans {
		if err := plan.Validate(); err != nil {
			return err
		}
		if ids[plan.ID] {
			return fmt.Errorf("duplicate planned notes ID %q", plan.ID)
		}
		ids[plan.ID] = true
	}
	return nil
}

func validateUniqueCollections(collections []Collection) error {
	ids := map[string]bool{}
	for _, collection := range collections {
		if err := collection.Validate(); err != nil {
			return err
		}
		if ids[collection.ID] {
			return fmt.Errorf("duplicate collection ID %q", collection.ID)
		}
		ids[collection.ID] = true
	}
	return nil
}

func NewWorkspace(scope Scope, records []Record) (Workspace, error) {
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return Workspace{}, err
		}
	}
	return Workspace{Scope: scope, Records: records}, nil
}
