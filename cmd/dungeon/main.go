package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/DungeonTUI/internal/classify"
	"github.com/hbaldwin98/DungeonTUI/internal/domain"
	"github.com/hbaldwin98/DungeonTUI/internal/ingest"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
	"github.com/hbaldwin98/DungeonTUI/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "import":
			if err := runImport(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "dungeon import: %v\n", err)
				os.Exit(1)
			}
			return
		case "dump":
			if err := runDump(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "dungeon dump: %v\n", err)
				os.Exit(1)
			}
			return
		case "export":
			if err := runExport(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "dungeon export: %v\n", err)
				os.Exit(1)
			}
			return
		case "restore":
			if err := runRestore(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "dungeon restore: %v\n", err)
				os.Exit(1)
			}
			return
		case "sync":
			if err := runSync(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "dungeon sync: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
	program := tea.NewProgram(tui.NewPersistent())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "dungeon: %v\n", err)
		os.Exit(1)
	}
}

func runImport(args []string) error {
	fs, kindFlag, dataDir, remove, err := parseImportArgs(args)
	if err != nil {
		return err
	}
	return runParsedImport(fs.Arg(0), *kindFlag, *dataDir, *remove)
}

func runParsedImport(target, kindFlag, dataDir string, remove bool) error {
	store, err := storage.OpenDefault()
	if err != nil {
		return err
	}
	defer store.Close()
	return runStoredImport(store, store.Path, target, kindFlag, dataDir, remove)
}

func runStoredImport(store storage.Store, path, target, kindFlag, dataDir string, remove bool) error {
	ws, err := loadImportWorkspace(store, path, remove)
	if err != nil {
		return err
	}
	ws.EnsureLibrary()
	return routeStoredImport(store, ws, target, kindFlag, dataDir, remove)
}

func routeStoredImport(store storage.Store, ws domain.Workspace, target, kindFlag, dataDir string, remove bool) error {
	if remove {
		return removeImportSource(store, ws, target)
	}
	return applyImportTarget(store, ws, target, kindFlag, dataDir)
}

func parseImportArgs(args []string) (*flag.FlagSet, *string, *string, *bool, error) {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	kindFlag := fs.String("kind", "auto", "bestiary, adventure, rules, or auto (markdown files)")
	dataDir := fs.String("data", "", "local 5e.tools directory containing data/ (optional)")
	remove := fs.Bool("remove", false, "delete an ingested source by id or title instead of importing")
	if err := fs.Parse(args); err != nil {
		return nil, nil, nil, nil, err
	}
	if fs.NArg() != 1 {
		return nil, nil, nil, nil, fmt.Errorf("usage: dungeon import [-kind auto|bestiary|adventure|rules] [-data DIR] <file.md|url|5e:ID>\n       dungeon import -remove <source-id-or-title>\n\n5e.tools adventures are cached as read-only reference (no wiki rows). Markdown files still become owner wiki records. Mechanical books (MM/PHB) are not imported.")
	}
	return fs, kindFlag, dataDir, remove, nil
}

func loadImportWorkspace(store storage.Store, path string, removing bool) (domain.Workspace, error) {
	ws, err := store.Load()
	if err == nil {
		return ws, nil
	}
	return newOrFailedImportWorkspace(path, removing, err)
}

func newOrFailedImportWorkspace(path string, removing bool, loadErr error) (domain.Workspace, error) {
	if !errors.Is(loadErr, os.ErrNotExist) {
		return domain.Workspace{}, fmt.Errorf("load workspace at %s: %w", path, loadErr)
	}
	if removing {
		return domain.Workspace{}, fmt.Errorf("no workspace at %s", path)
	}
	fmt.Println("created a new workspace at", path)
	return newImportWorkspace(), nil
}

func removeImportSource(store storage.Store, ws domain.Workspace, query string) error {
	doc, ok := ws.FindSource(query)
	if !ok {
		return fmt.Errorf("no ingested source matching %q", query)
	}
	if !ws.RemoveSource(doc.ID) {
		return fmt.Errorf("could not remove %s", doc.ID)
	}
	return saveRemovedImport(store, ws, doc)
}

func saveRemovedImport(store storage.Store, ws domain.Workspace, doc domain.SourceDocument) error {
	if err := store.Save(ws); err != nil {
		return err
	}
	fmt.Printf("Removed %s (%s)\n", doc.Title, doc.ID)
	return nil
}

func applyImportTarget(store storage.Store, ws domain.Workspace, target, kindFlag, dataDir string) error {
	kind, err := ingest.ParseKind(kindFlag)
	if err != nil {
		return err
	}
	return applyParsedImportTarget(store, ws, target, dataDir, kind)
}

func applyParsedImportTarget(store storage.Store, ws domain.Workspace, target, dataDir string, kind domain.SourceKind) error {
	var lastProgress string
	ws, report, err := ingest.ApplyTarget(ws, target, ingest.Options{
		Kind:    kind,
		Scope:   ws.Scope,
		DataDir: dataDir,
		Progress: func(p ingest.Progress) {
			line := p.String()
			if line == lastProgress {
				return
			}
			lastProgress = line
			fmt.Fprintln(os.Stderr, line)
		},
	})
	if err != nil {
		return err
	}
	return saveAppliedImport(store, ws, report)
}

func saveAppliedImport(store storage.Store, ws domain.Workspace, report ingest.Report) error {
	if err := store.Save(ws); err != nil {
		return err
	}
	printImportReport(ws, report)
	return nil
}

func printImportReport(ws domain.Workspace, report ingest.Report) {
	if report.Reference {
		fmt.Printf("Cached %s as reference · enable it for Sources to read and @ peek\n", report.Title)
		fmt.Printf("source id %s\n", report.SourceID)
		return
	}
	fmt.Printf("Imported %s (%s) into %s\n", report.Title, report.Kind, ws.Scope.Campaign)
	fmt.Printf("%d records, %d prep notes, %d associations\n", report.Records, report.Planned, report.Linked)
	fmt.Printf("source id %s\n", report.SourceID)
}

func newImportWorkspace() domain.Workspace {
	scope := domain.Scope{
		WorldID: "forgotten-realms", WorldName: "Forgotten Realms",
		CampaignID: "imported", Campaign: "Imported",
	}
	ws := domain.Workspace{
		Scope: scope,
		Library: []domain.WorldRef{{
			ID:   scope.WorldID,
			Name: scope.WorldName,
			Campaigns: []domain.CampaignRef{
				{ID: scope.CampaignID, Name: scope.Campaign},
			},
		}},
	}
	return ws
}

func loadWorkspace() (domain.Workspace, storage.Store, error) {
	store, err := storage.OpenDefault()
	if err != nil {
		return domain.Workspace{}, store, err
	}
	ws, err := store.Load()
	if err != nil {
		return domain.Workspace{}, store, err
	}
	ws.EnsureLibrary()
	return ws, store, nil
}

func runDump(args []string) error {
	fs := flag.NewFlagSet("dump", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("source", "", "limit to a source id or title")
	body := fs.Bool("body", true, "include record bodies")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ws, _, err := loadWorkspace()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(classify.Dump(ws, *source, *body))
}

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("o", "", "write to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := storage.OpenDefault()
	if err != nil {
		return err
	}
	defer store.Close()
	if strings.TrimSpace(*out) != "" {
		if err := store.ExportTo(*out); err != nil {
			return err
		}
		fmt.Println("exported workspace to", *out)
		return nil
	}
	ws, err := store.Load()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(ws)
}

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := storage.OpenDefault()
	if err != nil {
		return err
	}
	defer store.Close()
	if fs.NArg() == 0 {
		if err := store.RestoreBackup(); err != nil {
			return err
		}
		fmt.Println("restored workspace from", store.BackupPath())
		return nil
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dungeon restore [export.json]")
	}
	if err := store.RestoreFrom(fs.Arg(0)); err != nil {
		return err
	}
	fmt.Println("restored workspace from", fs.Arg(0))
	return nil
}
