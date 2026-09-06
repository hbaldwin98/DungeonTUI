package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/hbaldwin98/dungeon/internal/tui"
)

func main() {
	program := tea.NewProgram(tui.New())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "dungeon: %v\n", err)
		os.Exit(1)
	}
}
