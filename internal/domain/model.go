package domain

import (
	"fmt"
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
	Aliases     []string
	Tags        []string
	IsAIContent bool
}

type EntityLink struct {
	Text     string
	RecordID string
}

type TranscriptEntry struct {
	ID        string
	Text      string
	CreatedAt time.Time
	Links     []EntityLink
	Revision  int
	Undone    bool
}

type SessionRecord struct {
	ID        string
	Title     string
	Scope     Scope
	StartedAt time.Time
	EndedAt   *time.Time
	Entries   []TranscriptEntry
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
	Scope    Scope
	Records  []Record
	Sessions []SessionRecord
}

func NewWorkspace(scope Scope, records []Record) (Workspace, error) {
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return Workspace{}, err
		}
	}
	return Workspace{Scope: scope, Records: records}, nil
}
