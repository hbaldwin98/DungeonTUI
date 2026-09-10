package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hbaldwin98/DungeonTUI/internal/gitsync"
	"github.com/hbaldwin98/DungeonTUI/internal/storage"
)

const syncUsage = "usage: dungeon sync init [-dir PATH] [-b BRANCH] <git-url>\n" +
	"       dungeon sync push\n" +
	"       dungeon sync pull [-force]\n" +
	"       dungeon sync status"

func runSync(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", syncUsage)
	}
	switch args[0] {
	case "init":
		return runSyncInit(args[1:])
	case "push":
		return runSyncPush()
	case "pull":
		return runSyncPull(args[1:])
	case "status":
		return runSyncStatus()
	default:
		return fmt.Errorf("%s", syncUsage)
	}
}

func runSyncInit(args []string) error {
	fs := flag.NewFlagSet("sync init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dirFlag := fs.String("dir", "", "local clone directory (default ~/.config/dungeon/sync-repo)")
	branchFlag := fs.String("b", "main", "branch to push and pull")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("%s", syncUsage)
	}
	cfg, err := newSyncConfig(fs.Arg(0), *dirFlag, *branchFlag)
	if err != nil {
		return err
	}
	client, err := cfg.Client()
	if err != nil {
		return err
	}
	if err := client.Init(); err != nil {
		return err
	}
	path, err := gitsync.DefaultConfigPath()
	if err != nil {
		return err
	}
	if err := gitsync.SaveConfig(path, cfg); err != nil {
		return err
	}
	fmt.Println("configured git sync")
	fmt.Println("remote", cfg.Remote)
	fmt.Println("clone", client.Dir)
	fmt.Println("run dungeon sync push to upload this machine")
	return nil
}

func newSyncConfig(remote, dir, branch string) (gitsync.Config, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return gitsync.Config{}, fmt.Errorf("git remote is required")
	}
	if dir == "" {
		var err error
		dir, err = gitsync.DefaultRepoDir()
		if err != nil {
			return gitsync.Config{}, err
		}
	}
	if branch == "" {
		branch = "main"
	}
	return gitsync.Config{Remote: remote, Dir: dir, Branch: branch}, nil
}

func runSyncPush() error {
	client, err := configuredClient()
	if err != nil {
		return err
	}
	store, err := storage.OpenDefault()
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err := store.Load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no local workspace to push; create a campaign first")
		}
		return err
	}
	snap, err := gitsync.SnapshotFrom(store)
	if err != nil {
		return err
	}
	result, err := client.Push(snap)
	if err != nil {
		return err
	}
	printSyncResult(result)
	return nil
}

func runSyncPull(args []string) error {
	fs := flag.NewFlagSet("sync pull", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "discard unpushed local changes and take the remote")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := configuredClient()
	if err != nil {
		return err
	}
	store, err := storage.OpenDefault()
	if err != nil {
		return err
	}
	defer store.Close()
	local, err := gitsync.SnapshotFrom(store)
	if err != nil {
		return err
	}
	snap, result, err := client.Pull(local, *force)
	if err != nil {
		return err
	}
	if err := gitsync.Apply(store, snap); err != nil {
		return err
	}
	printSyncResult(result)
	fmt.Printf("%d records, %d sessions\n", len(snap.Workspace.Records), len(snap.Workspace.Sessions))
	return nil
}

func runSyncStatus() error {
	cfg, err := loadSyncConfig()
	if err != nil {
		return err
	}
	client, err := cfg.Client()
	if err != nil {
		return err
	}
	fmt.Println("remote", cfg.Remote)
	branch := strings.TrimSpace(cfg.Branch)
	if branch == "" {
		branch = "main"
	}
	fmt.Println("branch", branch)
	fmt.Println("clone", client.Dir)
	if _, err := os.Stat(client.Dir); err != nil {
		fmt.Println("clone does not exist yet")
		return nil
	}
	out, err := client.Git.Run(client.Dir, "log", "-1", "--oneline")
	if err != nil {
		fmt.Println("clone has no commits yet")
		return nil
	}
	fmt.Println("head", out)
	return nil
}

func configuredClient() (gitsync.Client, error) {
	cfg, err := loadSyncConfig()
	if err != nil {
		return gitsync.Client{}, err
	}
	return cfg.Client()
}

func loadSyncConfig() (gitsync.Config, error) {
	path, err := gitsync.DefaultConfigPath()
	if err != nil {
		return gitsync.Config{}, err
	}
	cfg, err := gitsync.LoadConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return gitsync.Config{}, fmt.Errorf("sync is not configured; run dungeon sync init <git-url>")
		}
		return gitsync.Config{}, err
	}
	return cfg, nil
}

func printSyncResult(result gitsync.Result) {
	if result.Commit != "" {
		fmt.Println(result.Message, result.Commit)
		return
	}
	fmt.Println(result.Message)
}
