package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/observer"
)

type devicesLoadedMsg struct {
	devices []domain.Device
	err     error
}

type appsLoadedMsg struct {
	apps []domain.Package
	err  error
}

type streamStartedMsg struct {
	stream   *adb.Stream
	streamID int
	pid      string
	err      error
}

type logBatchMsg struct {
	stream   *adb.Stream
	streamID int
	lines    []domain.LogLine
}

type streamDoneMsg struct {
	streamID int
	err      error
}

type observerEventMsg struct {
	event observer.Event
}

type mcpStartedMsg struct {
	err error
}

type mcpStoppedMsg struct {
	err error
}

func (m *model) loadDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		devices, err := m.client.ListDevices(m.ctx)
		return devicesLoadedMsg{devices: devices, err: err}
	}
}

func (m *model) loadAppsCmd(serial string) tea.Cmd {
	return func() tea.Msg {
		apps, err := m.client.ListPackages(m.ctx, serial)
		return appsLoadedMsg{apps: apps, err: err}
	}
}

func (m *model) startStreamCmd(serial, packageName string, streamID int) tea.Cmd {
	return func() tea.Msg {
		pidCtx, cancel := context.WithTimeout(m.ctx, adbPIDTimeout)
		defer cancel()
		pid, _ := m.client.PIDOf(pidCtx, serial, packageName)
		stream, err := m.client.StartLogcat(m.ctx, serial)
		return streamStartedMsg{stream: stream, streamID: streamID, pid: pid, err: err}
	}
}

func waitLogBatchCmd(stream *adb.Stream, streamID int) tea.Cmd {
	return func() tea.Msg {
		if stream == nil {
			return nil
		}
		line, ok := <-stream.Lines
		if !ok {
			return nil
		}

		lines := []domain.LogLine{line}
		timer := time.NewTimer(logBatchWindow)
		defer timer.Stop()

		// Logcat can emit large bursts. Sending one Bubble Tea message per line
		// makes the whole model rebuild the viewport content for every line and
		// can starve regular key/layout updates. This mirrors lazydocker's
		// "tainted view + periodic redraw" principle while staying in Bubble Tea:
		// wait for one line, gather the short burst around it, then repaint once.
		for len(lines) < logBatchMaxLines {
			select {
			case line, ok := <-stream.Lines:
				if !ok {
					return logBatchMsg{stream: stream, streamID: streamID, lines: lines}
				}
				lines = append(lines, line)
			case <-timer.C:
				return logBatchMsg{stream: stream, streamID: streamID, lines: lines}
			}
		}

		return logBatchMsg{stream: stream, streamID: streamID, lines: lines}
	}
}

func waitStreamDoneCmd(stream *adb.Stream, streamID int) tea.Cmd {
	return func() tea.Msg {
		if stream == nil {
			return nil
		}
		err, ok := <-stream.Done
		if !ok {
			return streamDoneMsg{streamID: streamID}
		}
		return streamDoneMsg{streamID: streamID, err: err}
	}
}

func waitObserverEventCmd(ctx context.Context, obs *observer.Observer) tea.Cmd {
	return func() tea.Msg {
		if obs == nil {
			return nil
		}
		select {
		case event := <-obs.Events():
			return observerEventMsg{event: event}
		case <-ctx.Done():
			return nil
		}
	}
}
