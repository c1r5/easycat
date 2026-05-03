package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type compactDelegate struct{}

func (d compactDelegate) Height() int  { return 1 }
func (d compactDelegate) Spacing() int { return 0 }

func (d compactDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	titled, ok := item.(interface{ Title() string })
	if !ok {
		return
	}

	cursor := "  "
	style := compactItemStyle
	if index == m.Index() {
		cursor = "> "
		style = compactSelectedItemStyle
	}

	width := m.Width() - 2
	if width < 1 {
		width = 1
	}
	title := ansi.Truncate(titled.Title(), width, "...")
	fmt.Fprint(w, style.Render(cursor+title))
}

var (
	compactItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	compactSelectedItemStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("39")).
					Bold(true)
)
