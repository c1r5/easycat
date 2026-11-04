package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/c1r5/easycat/internal/core"
)

type listKind int

const (
	kindDevices listKind = iota
	kindApps
)

type compactDelegate struct {
	kind  listKind
	model *model
}

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
	style := compactIdleStyle
	if index == m.Index() {
		cursor = "> "
		known, open := d.itemState(item)
		switch {
		case known && open:
			style = compactOpenStyle
		case known:
			style = compactClosedStyle
		default:
			style = compactIdleStyle
		}
	}

	width := m.Width() - 2
	if width < 1 {
		width = 1
	}
	title := ansi.Truncate(titled.Title(), width, "...")
	fmt.Fprint(w, style.Render(cursor+title))
}

func (d compactDelegate) itemState(item list.Item) (known bool, open bool) {
	if d.model == nil {
		return false, false
	}
	switch d.kind {
	case kindDevices:
		device, ok := item.(core.Device)
		if !ok {
			return false, false
		}
		return true, device.State == "device"
	case kindApps:
		app, ok := item.(core.App)
		if !ok || d.model.selectedApp == nil || app.Name != d.model.selectedApp.Name || !d.model.appStateKnown {
			return false, false
		}
		return true, d.model.appRunning
	default:
		return false, false
	}
}

var (
	compactIdleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("4"))
	compactOpenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("2")).
				Bold(true)
	compactClosedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("1")).
				Bold(true)
	compactCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("4")).
				Bold(true)
)
