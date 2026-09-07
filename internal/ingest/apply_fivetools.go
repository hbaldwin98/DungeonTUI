package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// ApplyFiveE fetches a 5e.tools adventure, caches it, and enables it for the
// campaign. It does not write wiki rows.
func ApplyFiveE(ws domain.Workspace, ref string, opts Options) (domain.Workspace, Report, error) {
	opts.report(StageFetch, "Resolving 5e.tools adventure", 0, 0)
	fetcher := &progressFetcher{
		inner:  fivetools.ResolveFetcher(opts.Fetcher, opts.DataDir),
		report: opts.report,
	}
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
	if bundle.Doc.Kind != domain.SourceAdventure {
		bundle.Doc.Kind = domain.SourceAdventure
	}
	if err := bundle.Doc.Validate(); err != nil {
		return ws, Report{}, err
	}

	opts.report(StageWrite, "Caching "+bundle.Doc.Title, 0, 0)
	ws.EnsureLibrary()
	ws.StripSourceContent(bundle.Doc.ID)
	ws.UpsertSource(bundle.Doc)

	scope := opts.Scope
	if scope.WorldID == "" {
		scope = ws.Scope
	}
	if scope.CampaignID != "" {
		ws.EnableSource(scope.WorldID, scope.CampaignID, bundle.Doc.ID)
	}
	return ws, Report{
		SourceID:  bundle.Doc.ID,
		Kind:      bundle.Doc.Kind,
		Title:     bundle.Doc.Title,
		Reference: true,
	}, nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

type progressFetcher struct {
	inner  fivetools.Fetcher
	report func(Stage, string, int, int)
	n      int
}

func (f *progressFetcher) Get(path string) ([]byte, error) {
	f.n++
	if f.report != nil {
		f.report(StageFetch, "Fetching "+filepath.Base(path), f.n, 0)
	}
	return f.inner.Get(path)
}
