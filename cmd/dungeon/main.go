package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/dungeon/internal/domain"
	"github.com/hbaldwin98/dungeon/internal/ingest"
	"github.com/hbaldwin98/dungeon/internal/storage"
	"github.com/hbaldwin98/dungeon/internal/tui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "import" {
		if err := runImport(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "dungeon import: %v\n", err)
			os.Exit(1)
		}
		return
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
	kindFlag := fs.String("kind", "auto", "bestiary, adventure, rules, or auto")
	dataDir := fs.String("data", "", "local 5e.tools directory containing data/ (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dungeon import [-kind auto|bestiary|adventure|rules] [-data DIR] <file.md|url|5e:ID>")
	}
	kind, err := ingest.ParseKind(*kindFlag)
	if err != nil {
		return err
	}
	path, err := storage.DefaultPath()
	if err != nil {
		return err
	}
	store := storage.NewJSON(path)
	ws, err := store.Load()
	if err != nil {
		ws = newImportWorkspace()
		fmt.Println("created a new workspace at", path)
	}
	ws.EnsureLibrary()
	ws, report, err := ingest.ApplyTarget(ws, fs.Arg(0), ingest.Options{
		Kind:    kind,
		Scope:   ws.Scope,
		DataDir: *dataDir,
	})
	if err != nil {
		return err
	}
	if err := store.Save(ws); err != nil {
		return err
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
