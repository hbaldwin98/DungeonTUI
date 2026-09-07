package classify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hbaldwin98/dungeon/internal/domain"
)

type Kind string

const (
	KindRetype      Kind = "retype"
	KindDuplicate   Kind = "duplicate-title"
	KindFrontMatter Kind = "front-matter"
	KindRoom        Kind = "numbered-room"
)

type Finding struct {
	Kind    Kind   `json:"kind"`
	ID      string `json:"record_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Detail  string `json:"detail"`
	Suggest string `json:"suggest_type,omitempty"`
}

type RecordView struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Folder   string   `json:"folder,omitempty"`
	Source   string   `json:"source,omitempty"`
	SourceID string   `json:"source_id,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Body     string   `json:"body,omitempty"`
}

type Snapshot struct {
	Campaign string                  `json:"campaign"`
	World    string                  `json:"world"`
	Sources  []domain.SourceDocument `json:"sources"`
	Records  []RecordView            `json:"records"`
	Findings []Finding               `json:"findings"`
}

// Dump is the headless view of ingested data for an agent harness.
func Dump(ws domain.Workspace, sourceFilter string, includeBody bool) Snapshot {
	filter := strings.TrimSpace(sourceFilter)
	out := Snapshot{
		Campaign: ws.Scope.Campaign,
		World:    ws.Scope.WorldName,
		Sources:  append([]domain.SourceDocument(nil), ws.Sources...),
		Records:  make([]RecordView, 0, len(ws.Records)),
	}
	for _, rec := range ws.Records {
		if filter != "" && rec.SourceID != filter && !strings.EqualFold(rec.Source, filter) && rec.ID != filter {
			continue
		}
		view := RecordView{
			ID: rec.ID, Type: string(rec.Type), Title: rec.Title, Folder: rec.Folder,
			Source: rec.Source, SourceID: rec.SourceID, Summary: rec.Summary, Tags: rec.Tags,
		}
		if includeBody {
			view.Body = rec.Body
		}
		out.Records = append(out.Records, view)
	}
	out.Findings = Diagnose(ws.Records, filter)
	return out
}

func WriteDump(path string, ws domain.Workspace, sourceFilter string, includeBody bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("dump path is required")
	}
	data, err := json.MarshalIndent(Dump(ws, sourceFilter, includeBody), "", "  ")
	if err != nil {
		return fmt.Errorf("encode ingest dump: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create ingest dump directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write ingest dump: %w", err)
	}
	return nil
}

func Diagnose(records []domain.Record, sourceFilter string) []Finding {
	filter := strings.TrimSpace(sourceFilter)
	seen := map[string][]string{}
	var findings []Finding
	for _, rec := range records {
		if filter != "" && rec.SourceID != filter && !strings.EqualFold(rec.Source, filter) {
			continue
		}
		key := strings.ToLower(rec.SourceID + "\x00" + rec.Title)
		seen[key] = append(seen[key], rec.ID)
		want := rec.Type
		if rec.Type != domain.NPC && rec.Type != domain.Item && rec.Type != domain.Creature && rec.Type != domain.Rule {
			group := groupFromFolder(rec.Folder)
			want = Assign(rec.Title, group, rec.Tags)
		}
		if want != rec.Type {
			findings = append(findings, Finding{
				Kind: KindRetype, ID: rec.ID, Title: rec.Title,
				Detail:  "structure suggests " + string(want) + ", stored as " + string(rec.Type),
				Suggest: string(want),
			})
		}
		if FrontMatter(rec.Title) && rec.Type != domain.Note {
			findings = append(findings, Finding{
				Kind: KindFrontMatter, ID: rec.ID, Title: rec.Title,
				Detail:  "front-matter heading should be a note, not a site",
				Suggest: string(domain.Note),
			})
		}
		if NumberedRoom(rec.Title) && rec.Type != domain.Location {
			findings = append(findings, Finding{
				Kind: KindRoom, ID: rec.ID, Title: rec.Title,
				Detail:  "numbered heading is a room/site in published adventures",
				Suggest: string(domain.Location),
			})
		}
	}
	for _, ids := range seen {
		if len(ids) < 2 {
			continue
		}
		findings = append(findings, Finding{
			Kind:   KindDuplicate,
			ID:     ids[0],
			Detail: "duplicate title in the same source: " + strings.Join(ids, ", "),
		})
	}
	return findings
}

func groupFromFolder(folder string) string {
	folder = strings.Trim(folder, "/")
	if folder == "" {
		return ""
	}
	parts := strings.Split(folder, "/")
	if len(parts) <= 2 {
		return ""
	}
	return strings.Join(parts[2:], "/")
}
