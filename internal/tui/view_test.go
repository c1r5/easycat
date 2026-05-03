package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/c1r5/easycat/internal/adb"
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
