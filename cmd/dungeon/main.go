package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/dungeon/internal/classify"
	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest"
	"github.com/hbaldwin98/dungeon/internal/storage"
	"github.com/hbaldwin98/dungeon/internal/tui"
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
		case "classify":
			if err := runClassify(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "dungeon classify: %v\n", err)
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
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	kindFlag := fs.String("kind", "auto", "bestiary, adventure, rules, or auto (markdown files)")
	dataDir := fs.String("data", "", "local 5e.tools directory containing data/ (optional)")
	remove := fs.Bool("remove", false, "delete an ingested source by id or title instead of importing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dungeon import [-kind auto|bestiary|adventure|rules] [-data DIR] <file.md|url|5e:ID>\n       dungeon import -remove <source-id-or-title>\n\n5e.tools adventures are cached as read-only reference (no wiki rows). Markdown files still become owner wiki records. Mechanical books (MM/PHB) are not imported.")
	}
	path, err := storage.DefaultPath()
	if err != nil {
		return err
	}
	store := storage.NewJSON(path)
	ws, err := store.Load()
	if err != nil {
		if *remove {
			return fmt.Errorf("no workspace at %s", path)
		}
		ws = newImportWorkspace()
		fmt.Println("created a new workspace at", path)
	}
	ws.EnsureLibrary()
	if *remove {
		doc, ok := ws.FindSource(fs.Arg(0))
		if !ok {
			return fmt.Errorf("no ingested source matching %q", fs.Arg(0))
		}
		if !ws.RemoveSource(doc.ID) {
			return fmt.Errorf("could not remove %s", doc.ID)
		}
		if err := store.Save(ws); err != nil {
			return err
		}
		fmt.Printf("Removed %s (%s)\n", doc.Title, doc.ID)
		return nil
	}
	kind, err := ingest.ParseKind(*kindFlag)
	if err != nil {
		return err
	}
	var lastProgress string
	ws, report, err := ingest.ApplyTarget(ws, fs.Arg(0), ingest.Options{
		Kind:    kind,
		Scope:   ws.Scope,
		DataDir: *dataDir,
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
	if err := store.Save(ws); err != nil {
		return err
	}
	if report.Reference {
		fmt.Printf("Cached %s as reference · enable it for Sources to read and @ peek\n", report.Title)
		fmt.Printf("source id %s\n", report.SourceID)
		return nil
	}
	fmt.Printf("Imported %s (%s) into %s\n", report.Title, report.Kind, ws.Scope.Campaign)
	fmt.Printf("%d records, %d prep notes, %d associations\n", report.Records, report.Planned, report.Linked)
	fmt.Printf("source id %s\n", report.SourceID)
	return nil
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
	path, err := storage.DefaultPath()
	if err != nil {
		return domain.Workspace{}, nil, err
	}
	store := storage.NewJSON(path)
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

func runClassify(args []string) error {
	fs := flag.NewFlagSet("classify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("source", "", "limit to a source id or title")
	apply := fs.Bool("apply", false, "retype records that fail the structural sanity pass and save")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ws, store, err := loadWorkspace()
	if err != nil {
		return err
	}
	findings := classify.Diagnose(ws.Records, *source)
	if *apply {
		classify.OrganizeRecords(ws.Records)
		if err := store.Save(ws); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "applied structural classify; %d findings before apply\n", len(findings))
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(findings)
}
