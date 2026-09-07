package ingest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest/fivetools"
)

// ApplyTarget imports a local markdown file or a 5e.tools book/adventure ref.
func ApplyTarget(ws domain.Workspace, target string, opts Options) (domain.Workspace, Report, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return ws, Report{}, fmt.Errorf("import target is required")
	}
	if fileExists(target) {
		return ApplyFile(ws, target, opts)
	}
	if fivetools.LooksLikeRef(target) {
		return ApplyFiveE(ws, target, opts)
	}
	return ApplyFile(ws, target, opts)
}

// ApplyFiveE fetches the chosen 5e.tools source at import time and upserts it.
// Raw JSON is never written into the git tree; records land in the workspace.
func ApplyFiveE(ws domain.Workspace, ref string, opts Options) (domain.Workspace, Report, error) {
	fetcher := fivetools.ResolveFetcher(opts.Fetcher, opts.DataDir)
	bundle, err := fivetools.Build(fetcher, ref)
	if err != nil {
		return ws, Report{}, err
	}
	if opts.Kind != "" {
		bundle.Doc.Kind = opts.Kind
	}
	if opts.Path != "" {
		bundle.Doc.Path = opts.Path
	}
	if err := bundle.Doc.Validate(); err != nil {
		return ws, Report{}, err
	}

	ws.EnsureLibrary()
	ws.StripSourceContent(bundle.Doc.ID)
	ws.UpsertSource(bundle.Doc)

	scope := opts.Scope
	if scope.WorldID == "" {
		scope = ws.Scope
	}
	records, plans := fivetools.Materialize(bundle, scope, time.Now().UTC())
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return ws, Report{}, err
		}
	}
	ws.Records = append(ws.Records, records...)
	ws.PlannedNotes = append(ws.PlannedNotes, plans...)

	linked := 0
	if bundle.Doc.Kind == domain.SourceAdventure {
		enabled := append([]string(nil), ws.EnabledSourceIDs(scope)...)
		enabled = append(enabled, bundle.Doc.ID)
		linked = associateMentions(&ws, scope, enabled)
	}
	if scope.CampaignID != "" {
		ws.EnableSource(scope.WorldID, scope.CampaignID, bundle.Doc.ID)
	}
	return ws, Report{
		SourceID: bundle.Doc.ID,
		Kind:     bundle.Doc.Kind,
		Title:    bundle.Doc.Title,
		Records:  len(records),
		Planned:  len(plans),
		Linked:   linked,
	}, nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
