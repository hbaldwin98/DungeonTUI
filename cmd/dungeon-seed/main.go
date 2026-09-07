package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hbaldwin98/dungeon/internal/storage"
	"github.com/hbaldwin98/dungeon/internal/tui"
)

func main() {
	path, err := storage.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dungeon-seed: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		if err := copyFile(path, backup); err != nil {
			fmt.Fprintf(os.Stderr, "dungeon-seed: backup %s: %v\n", backup, err)
			os.Exit(1)
		}
		fmt.Println("backed up", backup)
	}
	if err := storage.NewSQLite(path).Save(tui.ShowcaseWorkspace()); err != nil {
		fmt.Fprintf(os.Stderr, "dungeon-seed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("seeded", path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
