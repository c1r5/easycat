package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
)

func TestRenderLogLineWrapped(t *testing.T) {
	line := domain.LogLine{
		Level:   "E",
		Tag:     "App",
		Message: strings.Repeat("payload", 8),
		Raw:     "E/App: " + strings.Repeat("payload", 8),
	}

	wrapped := renderLogLineWrapped(line, 20)
	if len(wrapped) < 2 {
		t.Fatalf("expected wrapped line, got %#v", wrapped)
	}
	for _, line := range wrapped {
		if width := printableWidth(line); width > 20 {
			t.Fatalf("wrapped line width = %d, want <= 20: %q", width, line)
		}
	}
}

func TestSelectDeviceFocusesApps(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	device := domain.Device{Serial: "emulator-5554", State: "device"}
	m.devices.SetItems([]list.Item{device})
	m.focus = focusDevices

	_, cmd := m.selectFocused()
	if cmd == nil {
		t.Fatal("expected load apps command")
	}
	if m.focus != focusApps {
		t.Fatalf("focus = %v, want %v", m.focus, focusApps)
	}
}

func printableWidth(s string) int {
	return len([]rune(stripANSI(s)))
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r >= '@' && r <= '~' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
