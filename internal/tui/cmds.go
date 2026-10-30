package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/core"
)

type devicesLoadedMsg struct {
	devices []core.Device
	err     error
}

type appsLoadedMsg struct {
	apps []core.App
	err  error
}

type streamStartedMsg struct {
	stream   *core.Stream
	streamID int
	pid      string
	err      error
}

type logBatchMsg struct {
	stream   *core.Stream
	streamID int
	lines    []core.LogLine
}

type streamDoneMsg struct {
	streamID int
	err      error
}

type busReadMsg struct {
	lines     []core.LogLine
	streamID  int
	hasLines  bool
	hasFollow bool
	follow    core.Event
}

func (m *model) loadDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		devices, err := m.client.ListDevices(m.ctx)
		return devicesLoadedMsg{devices: devices, err: err}
	}
}

func (m *model) loadAppsCmd(serial string) tea.Cmd {
	return func() tea.Msg {
		apps, err := m.client.ListApps(m.ctx, serial)
		return appsLoadedMsg{apps: apps, err: err}
	}
}

func (m *model) startStreamCmd(serial, packageName string, streamID int) tea.Cmd {
	return func() tea.Msg {
		pidCtx, cancel := context.WithTimeout(m.ctx, adbPIDTimeout)
		defer cancel()
		pid, _ := m.client.PID(pidCtx, serial, packageName)
		stream, err := m.client.StartLogcat(m.ctx, serial, m.bus, streamID)
		return streamStartedMsg{stream: stream, streamID: streamID, pid: pid, err: err}
	}
}

func (m *model) waitBusCmd() tea.Cmd {
	events := m.events
	return func() tea.Msg {
		if events == nil {
			return nil
		}
		ev, ok := <-events
		if !ok {
			return nil
		}
		if ev.Kind != core.KindLog {
			return busReadMsg{hasFollow: true, follow: ev}
		}

		lines := []core.LogLine{ev.Line}
		timer := time.NewTimer(logBatchWindow)
		defer timer.Stop()

		// Logcat can emit large bursts. Sending one Bubble Tea message per line
		// makes the whole model rebuild the viewport content for every line and
		// can starve regular key/layout updates. Wait for one line, gather the
		// short burst around it, then repaint once.
		for len(lines) < logBatchMaxLines {
			select {
			case next, ok := <-events:
				if !ok {
					return busReadMsg{lines: lines, streamID: ev.StreamID, hasLines: true}
				}
				if next.Kind == core.KindLog && next.StreamID == ev.StreamID {
					lines = append(lines, next.Line)
					continue
				}
				return busReadMsg{
					lines:     lines,
					streamID:  ev.StreamID,
					hasLines:  true,
					hasFollow: true,
					follow:    next,
				}
			case <-timer.C:
				return busReadMsg{lines: lines, streamID: ev.StreamID, hasLines: true}
			}
		}
		return busReadMsg{lines: lines, streamID: ev.StreamID, hasLines: true}
	}
}
