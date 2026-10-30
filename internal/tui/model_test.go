package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/core"
)

func TestRenderLogLineWrapped(t *testing.T) {
	line := core.LogLine{
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
	m := New(context.Background(), core.NewClient()).(*model)
	device := core.Device{Serial: "emulator-5554", State: "device"}
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
	m := New(context.Background(), core.NewClient()).(*model)
	currentStream := &core.Stream{}
	m.stream = currentStream
	m.streamID = 2

	updated, _ := m.Update(logBatchMsg{
		stream:   &core.Stream{},
		streamID: 1,
		lines:    []core.LogLine{{Raw: "stale"}},
	})
	m = updated.(*model)

	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("expected stale batch to be ignored, got %+v", got)
	}
}

func TestLogBatchBuffersWhilePausedWithoutRendering(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	stream := &core.Stream{}
	m.stream = stream
	m.streamID = 1
	m.paused = true
	m.selectedDevice = &core.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &core.App{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.width = 80
	m.height = 24
	m.resize()
	m.logs.SetContent("old")

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines:    []core.LogLine{{PID: "1234", Raw: "new"}},
	})
	m = updated.(*model)

	if got := m.buffer.Lines(); len(got) != 1 || got[0].Raw != "new" {
		t.Fatalf("expected paused batch to be buffered, got %+v", got)
	}
	if view := m.logs.View(); !strings.Contains(view, "old") || strings.Contains(view, "new") {
		t.Fatalf("expected paused viewport to keep previous content, got %q", view)
	}
}

func TestLogBatchDropsLinesWhenPIDOnlyDisabled(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	stream := &core.Stream{}
	m.stream = stream
	m.streamID = 1
	m.selectedDevice = &core.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &core.App{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = false

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines: []core.LogLine{
			{PID: "1234", Raw: "app log"},
			{PID: "9999", Raw: "system log"},
		},
	})
	m = updated.(*model)

	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("captured lines with PID only disabled: %+v", got)
	}
}

func TestLogBatchCapturesOnlySelectedPID(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	stream := &core.Stream{}
	m.stream = stream
	m.streamID = 1
	m.selectedDevice = &core.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &core.App{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines: []core.LogLine{
			{PID: "9999", Raw: "system log"},
			{PID: "1234", Raw: "app log"},
		},
	})
	m = updated.(*model)

	got := m.buffer.Lines()
	if len(got) != 1 || got[0].PID != "1234" {
		t.Fatalf("captured lines = %+v, want only PID 1234", got)
	}
}

func TestDisablingPIDOnlyClearsCapture(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	m.selectedDevice = &core.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &core.App{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.buffer.Add(core.LogLine{PID: "1234", Raw: "captured"})

	updated, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(*model)

	if m.filters.PIDOnly {
		t.Fatal("PID only remained enabled")
	}
	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("buffer was not cleared: %+v", got)
	}
}

func TestSelectingAppClearsBufferWhileResolvingPID(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	m.selectedDevice = &core.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &core.App{Name: "com.old"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.focus = focusApps
	m.buffer.Add(core.LogLine{PID: "1234", Raw: "old app log"})
	m.apps.SetItems([]list.Item{core.App{Name: "com.new"}})

	_, cmd := m.selectFocused()
	if cmd == nil {
		t.Fatal("expected stream start command")
	}
	if m.selectedApp == nil || m.selectedApp.Name != "com.new" {
		t.Fatalf("selected app = %+v, want com.new", m.selectedApp)
	}
	if m.filters.PIDOnly || m.filters.PID != "" {
		t.Fatalf("filters = %+v, want pid cleared while the new pid resolves", m.filters)
	}
	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("buffer retained previous app logs: %+v", got)
	}
}

func TestActionShortcutsRequireCtrl(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	m.buffer.Add(core.LogLine{Raw: "keep"})

	updated, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(*model)
	if got := m.buffer.Lines(); len(got) != 1 || got[0].Raw != "keep" {
		t.Fatalf("plain c cleared logs, got %+v", got)
	}

	updated, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(*model)
	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("ctrl+k did not clear logs, got %+v", got)
	}
}

func TestStreamDoneIgnoresStaleStream(t *testing.T) {
	m := New(context.Background(), core.NewClient()).(*model)
	currentStream := &core.Stream{}
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
	line := core.LogLine{
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

func TestLogcatLineReachesBufferThroughBus(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "adb")
	body := []byte(`#!/bin/sh
if [ "$1" = "-s" ] && [ "$3" = "shell" ] && [ "$4" = "pidof" ]; then
  echo 1234
  exit 0
fi
if [ "$1" = "-s" ] && [ "$3" = "logcat" ]; then
  printf '%s\n' '01-01 00:00:00.000 1234 1 I Tag: hello' '01-01 00:00:00.001 9999 1 I Tag: other'
  exit 0
fi
exit 1
`)
	if err := os.WriteFile(script, body, 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(context.Background(), core.Client{Path: script}).(*model)
	sub := m.bus.Subscribe()
	t.Cleanup(sub.Cancel)
	m.events = sub.Events
	m.selectedDevice = &core.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &core.App{Name: "com.example"}
	id := m.beginStream()
	started, ok := m.startStreamCmd("emulator-5554", "com.example", id)().(streamStartedMsg)
	if !ok {
		t.Fatal("expected stream start message")
	}
	if started.err != nil {
		t.Fatal(started.err)
	}
	if started.pid != "1234" {
		t.Fatalf("pid = %q, want 1234", started.pid)
	}

	updated, cmd := m.Update(started)
	m = updated.(*model)
	if cmd == nil {
		t.Fatal("expected bus wait")
	}
	updated, _ = m.Update(cmd())
	m = updated.(*model)

	got := m.buffer.Lines()
	if len(got) != 1 || got[0].Message != "hello" || got[0].PID != "1234" {
		t.Fatalf("buffer = %+v, want the selected pid line", got)
	}
	if m.stream != nil {
		t.Fatal("stream remained after logcat exited")
	}
}
