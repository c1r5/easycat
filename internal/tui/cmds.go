package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
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
	stream *adb.Stream
	pid    string
	err    error
}

type logLineMsg struct {
	line domain.LogLine
}

type streamDoneMsg struct {
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

func (m *model) startStreamCmd(serial, packageName string) tea.Cmd {
	return func() tea.Msg {
		pidCtx, cancel := context.WithTimeout(m.ctx, adbPIDTimeout)
		defer cancel()
		pid, _ := m.client.PIDOf(pidCtx, serial, packageName)
		stream, err := m.client.StartLogcat(m.ctx, serial)
		return streamStartedMsg{stream: stream, pid: pid, err: err}
	}
}

func waitLogLineCmd(stream *adb.Stream) tea.Cmd {
	return func() tea.Msg {
		if stream == nil {
			return nil
		}
		line, ok := <-stream.Lines
		if !ok {
			return nil
		}
		return logLineMsg{line: line}
	}
}

func waitStreamDoneCmd(stream *adb.Stream) tea.Cmd {
	return func() tea.Msg {
		if stream == nil {
			return nil
		}
		err, ok := <-stream.Done
		if !ok {
			return streamDoneMsg{}
		}
		return streamDoneMsg{err: err}
	}
}
