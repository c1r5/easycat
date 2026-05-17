package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

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

func TestLogBatchIgnoresStaleStream(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	currentStream := &adb.Stream{}
	m.stream = currentStream
	m.streamID = 2

	updated, _ := m.Update(logBatchMsg{
		stream:   &adb.Stream{},
		streamID: 1,
		lines:    []domain.LogLine{{Raw: "stale"}},
	})
	m = updated.(*model)

	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("expected stale batch to be ignored, got %+v", got)
	}
}

func TestLogBatchBuffersWhilePausedWithoutRendering(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	stream := &adb.Stream{}
	m.stream = stream
	m.streamID = 1
	m.paused = true
	m.width = 80
	m.height = 24
	m.resize()
	m.logs.SetContent("old")

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines:    []domain.LogLine{{Raw: "new"}},
	})
	m = updated.(*model)

	if got := m.buffer.Lines(); len(got) != 1 || got[0].Raw != "new" {
		t.Fatalf("expected paused batch to be buffered, got %+v", got)
	}
	if view := m.logs.View(); !strings.Contains(view, "old") || strings.Contains(view, "new") {
		t.Fatalf("expected paused viewport to keep previous content, got %q", view)
	}
}

func TestActionShortcutsRequireCtrl(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	m.buffer.Add(domain.LogLine{Raw: "keep"})
	m.mcpEnabled = true

	updated, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(*model)
	if got := m.buffer.Lines(); len(got) != 1 || got[0].Raw != "keep" {
		t.Fatalf("plain c cleared logs, got %+v", got)
	}

	updated, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = updated.(*model)
	if !m.mcpEnabled {
		t.Fatal("plain m toggled MCP")
	}

	updated, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(*model)
	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("ctrl+k did not clear logs, got %+v", got)
	}
}

func TestStreamDoneIgnoresStaleStream(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	currentStream := &adb.Stream{}
	m.stream = currentStream
	m.streamID = 2
	m.status = "streaming"

	updated, _ := m.Update(streamDoneMsg{streamID: 1, err: context.Canceled})
	m = updated.(*model)

	if m.stream != currentStream {
		t.Fatal("expected current stream to remain active")
	}
	if m.status != "streaming" {
		t.Fatalf("status = %q, want streaming", m.status)
	}
}

func TestRenderLogLineSanitizesTerminalControls(t *testing.T) {
	line := domain.LogLine{
		Level:   "E",
		Tag:     "App\x1b[31m",
		Message: "bad\x07\r\ntext\x7f",
		Raw:     "raw",
	}

	rendered := stripANSI(renderLogLine(line))
	if strings.ContainsAny(rendered, "\x1b\a\r\n\x7f") {
		t.Fatalf("rendered log still contains terminal controls: %q", rendered)
	}
	if !strings.Contains(rendered, "bad  text") {
		t.Fatalf("expected newlines to be normalized to spaces, got %q", rendered)
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
