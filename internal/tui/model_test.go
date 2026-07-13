package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/incidents"
	"github.com/c1r5/easycat/internal/observer"
	"github.com/c1r5/easycat/internal/rules"
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
	m.selectedDevice = &domain.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &domain.Package{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.width = 80
	m.height = 24
	m.resize()
	m.logs.SetContent("old")

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines:    []domain.LogLine{{PID: "1234", Raw: "new"}},
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
	m := New(context.Background(), adb.NewClient()).(*model)
	stream := &adb.Stream{}
	m.stream = stream
	m.streamID = 1
	m.selectedDevice = &domain.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &domain.Package{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = false
	m.observer.Reset(observer.Context{Device: *m.selectedDevice, Package: m.selectedApp.Name, PID: m.filters.PID})
	m.observer.Start(context.Background())
	t.Cleanup(m.observer.Stop)

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines: []domain.LogLine{
			{PID: "1234", Raw: "app log"},
			{PID: "9999", Raw: "system log"},
		},
	})
	m = updated.(*model)

	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("captured lines with PID only disabled: %+v", got)
	}
	assertObserverLogsStayEmpty(t, m.observer)
}

func TestLogBatchCapturesOnlySelectedPID(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	stream := &adb.Stream{}
	m.stream = stream
	m.streamID = 1
	m.selectedDevice = &domain.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &domain.Package{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.observer.Reset(observer.Context{Device: *m.selectedDevice, Package: m.selectedApp.Name, PID: m.filters.PID})
	m.observer.Start(context.Background())
	t.Cleanup(m.observer.Stop)

	updated, _ := m.Update(logBatchMsg{
		stream:   stream,
		streamID: 1,
		lines: []domain.LogLine{
			{PID: "9999", Raw: "system log"},
			{PID: "1234", Raw: "app log"},
		},
	})
	m = updated.(*model)

	got := m.buffer.Lines()
	if len(got) != 1 || got[0].PID != "1234" {
		t.Fatalf("captured lines = %+v, want only PID 1234", got)
	}
	observerLogs := waitForObserverLogCount(t, m.observer, 1)
	if observerLogs[0].PID != "1234" {
		t.Fatalf("observer received PID %q, want 1234", observerLogs[0].PID)
	}
}

func TestDisablingPIDOnlyClearsCaptureButPreservesIncidents(t *testing.T) {
	obs, err := observer.New(observer.Options{
		Rules: []rules.Rule{{
			ID:        "fatal",
			Match:     rules.MatchConfig{Level: "E", Contains: []string{"fatal test"}},
			Threshold: rules.ThresholdConfig{Count: 1, Window: time.Minute, Cooldown: time.Minute},
			Action:    rules.ActionConfig{Type: "write_incident"},
		}},
		Store: incidents.NewStore(t.TempDir()),
	})
	if err != nil {
		t.Fatal(err)
	}

	m := New(context.Background(), adb.NewClient()).(*model)
	m.observer = obs
	m.selectedDevice = &domain.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &domain.Package{Name: "com.example"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.buffer.Add(domain.LogLine{PID: "1234", Raw: "captured"})
	obs.Reset(observer.Context{Device: *m.selectedDevice, Package: m.selectedApp.Name, PID: m.filters.PID})
	obs.Start(context.Background())
	t.Cleanup(obs.Stop)
	if !obs.Publish(domain.LogLine{Level: "E", PID: "1234", Raw: "fatal test"}) {
		t.Fatal("failed to publish test incident line")
	}
	select {
	case event := <-obs.Events():
		if event.Err != nil {
			t.Fatal(event.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for incident")
	}

	updated, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(*model)

	if m.filters.PIDOnly {
		t.Fatal("PID only remained enabled")
	}
	if got := m.buffer.Lines(); len(got) != 0 {
		t.Fatalf("buffer was not cleared: %+v", got)
	}
	snapshot := obs.Snapshot()
	if len(snapshot.Logs) != 0 {
		t.Fatalf("observer logs were not cleared: %+v", snapshot.Logs)
	}
	if len(snapshot.Incidents) != 1 {
		t.Fatalf("incidents = %d, want preserved incident", len(snapshot.Incidents))
	}
}

func TestSelectingAppClearsPreviousObserverLogsWhileResolvingPID(t *testing.T) {
	m := New(context.Background(), adb.NewClient()).(*model)
	m.selectedDevice = &domain.Device{Serial: "emulator-5554", State: "device"}
	m.selectedApp = &domain.Package{Name: "com.old"}
	m.filters.PID = "1234"
	m.filters.PIDOnly = true
	m.focus = focusApps
	m.apps.SetItems([]list.Item{domain.Package{Name: "com.new"}})
	m.observer.Reset(observer.Context{Device: *m.selectedDevice, Package: m.selectedApp.Name, PID: m.filters.PID})
	m.observer.Start(context.Background())
	if !m.observer.Publish(domain.LogLine{PID: "1234", Raw: "old app log"}) {
		t.Fatal("failed to publish old app log")
	}
	waitForObserverLogCount(t, m.observer, 1)

	_, cmd := m.selectFocused()
	if cmd == nil {
		t.Fatal("expected stream start command")
	}
	if m.filters.PIDOnly {
		t.Fatal("PID only remained enabled while resolving the new PID")
	}
	snapshot := m.observer.Snapshot()
	if snapshot.Context.Package != "com.new" || snapshot.Context.PID != "" {
		t.Fatalf("observer context = %+v, want new package without PID", snapshot.Context)
	}
	if len(snapshot.Logs) != 0 {
		t.Fatalf("observer retained previous app logs: %+v", snapshot.Logs)
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

func waitForObserverLogCount(t *testing.T, obs *observer.Observer, want int) []domain.LogLine {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		logs := obs.Snapshot().Logs
		if len(logs) == want {
			return logs
		}
		time.Sleep(time.Millisecond)
	}
	logs := obs.Snapshot().Logs
	t.Fatalf("observer log count = %d, want %d", len(logs), want)
	return nil
}

func assertObserverLogsStayEmpty(t *testing.T, obs *observer.Observer) {
	t.Helper()
	deadline := time.Now().Add(25 * time.Millisecond)
	for time.Now().Before(deadline) {
		if logs := obs.Snapshot().Logs; len(logs) != 0 {
			t.Fatalf("observer unexpectedly received logs: %+v", logs)
		}
		time.Sleep(time.Millisecond)
	}
}
