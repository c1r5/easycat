package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
)

func TestViewFitsTerminal(t *testing.T) {
	sizes := []struct {
		width  int
		height int
	}{
		{120, 40},
		{80, 24},
		{60, 18},
		{42, 12},
	}

	for _, size := range sizes {
		m := New(context.Background(), adb.NewClient()).(*model)
		m.width = size.width
		m.height = size.height
		m.status = "this is a very long status that should never force the footer outside the terminal width"
		m.resize()

		view := m.View()
		if got := lipgloss.Width(view); got > size.width {
			t.Fatalf("view width = %d, want <= %d for size %+v", got, size.width, size)
		}
		if got := lipgloss.Height(view); got > size.height {
			t.Fatalf("view height = %d, want <= %d for size %+v", got, size.height, size)
		}
		if !strings.Contains(view, "tab: focus") {
			t.Logf("view for size %+v:\n%s", size, view)
			t.Fatalf("footer shortcuts were clipped for size %+v", size)
		}
	}
}

func TestViewContainsOneFooterWithHostileLogText(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	m.width = 80
	m.height = 24
	m.resize()
	m.buffer.Add(domainLogLine("before\x1b[2J\x1b[H\rafter"))
	m.refreshLogContent(true)

	view := m.View()
	if got := strings.Count(stripANSI(view), "tab: focus"); got != 1 {
		t.Fatalf("footer shortcut count = %d, want 1\n%s", got, view)
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("view width = %d, want <= %d", got, m.width)
	}
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("view height = %d, want <= %d", got, m.height)
	}
}

func domainLogLine(raw string) domain.LogLine {
	return domain.LogLine{Raw: raw}
}
