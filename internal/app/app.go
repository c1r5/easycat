package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/tui"
)

func Run(ctx context.Context) error {
	program := tea.NewProgram(
		tui.New(ctx, adb.NewClient()),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	return err
}
