package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/tui"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := tea.NewProgram(
		tui.New(ctx, adb.NewClient()),
		tea.WithAltScreen(),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
