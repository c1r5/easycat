package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
)

type focus int

const (
	focusFilters focus = iota
	focusDevices
	focusApps
	focusLogcat
)

type filterField int

const (
	filterFieldPackage filterField = iota
	filterFieldText
	filterFieldLevel
	filterFieldPIDOnly
	filterFieldCount
)

type model struct {
	ctx    context.Context
	client adb.Client

	width       int
	height      int
	focus       focus
	filterField filterField

	devices list.Model
	apps    list.Model
	logs    viewport.Model
	filter  textinput.Model

	filters  domain.Filters
	buffer   *domain.LogBuffer
	stream   *adb.Stream
	allApps  []domain.Package
	appQuery string

	selectedDevice *domain.Device
	selectedApp    *domain.Package

	paused        bool
	loadingDevice bool
	loadingApps   bool
	status        string
}

func New(ctx context.Context, client adb.Client) tea.Model {
	devices := list.New([]list.Item{}, compactDelegate{}, 0, 0)
	devices.SetShowHelp(false)
	devices.SetShowTitle(false)
	devices.SetShowPagination(false)
	devices.SetFilteringEnabled(false)
	devices.SetShowStatusBar(false)

	apps := list.New([]list.Item{}, compactDelegate{}, 0, 0)
	apps.SetShowHelp(false)
	apps.SetShowTitle(false)
	apps.SetShowPagination(false)
	apps.SetFilteringEnabled(false)
	apps.SetShowStatusBar(false)

	filter := textinput.New()
	filter.Placeholder = "contains text"
	filter.Prompt = ""

	logs := viewport.New(0, 0)

	return &model{
		ctx:         ctx,
		client:      client,
		focus:       focusDevices,
		filterField: filterFieldText,
		devices:     devices,
		apps:        apps,
		logs:        logs,
		filter:      filter,
		buffer:      domain.NewLogBuffer(domain.MaxLogLines),
		status:      "loading devices...",
	}
}

func (m *model) Init() tea.Cmd {
	return m.loadDevicesCmd()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refreshLogContent(true)
		return m, nil
	case tea.KeyMsg:
		if m.filter.Focused() {
			return m.updateFilterInput(msg)
		}
		return m.updateKey(msg)
	case devicesLoadedMsg:
		m.loadingDevice = false
		if msg.err != nil {
			m.status = "adb devices failed: " + msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("%d device(s)", len(msg.devices))
		m.setDeviceItems(msg.devices)
		return m, nil
	case appsLoadedMsg:
		m.loadingApps = false
		if msg.err != nil {
			m.status = "pm list packages failed: " + msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("%d app(s)", len(msg.apps))
		m.setAppItems(msg.apps)
		return m, nil
	case streamStartedMsg:
		if msg.err != nil {
			m.status = "logcat failed: " + msg.err.Error()
			return m, nil
		}
		m.stream = msg.stream
		m.filters.PID = msg.pid
		m.filters.PIDOnly = msg.pid != ""
		m.status = "streaming from " + m.client.LogcatWindow()
		if msg.pid == "" {
			m.status = "streaming from " + m.client.LogcatWindow() + " without pid"
		}
		return m, tea.Batch(waitLogLineCmd(msg.stream), waitStreamDoneCmd(msg.stream))
	case logLineMsg:
		m.buffer.Add(msg.line)
		if !m.paused {
			m.refreshLogContent(true)
		}
		return m, waitLogLineCmd(m.stream)
	case streamDoneMsg:
		if msg.err != nil {
			m.status = "stream stopped: " + msg.err.Error()
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.focus {
	case focusDevices:
		m.devices, cmd = m.devices.Update(msg)
	case focusApps:
		m.apps, cmd = m.apps.Update(msg)
	case focusLogcat:
		m.logs, cmd = m.logs.Update(msg)
	}
	return m, cmd
}

func (m *model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.filter.Blur()
		m.filters.Text = m.filter.Value()
		m.refreshLogContent(false)
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filters.Text = m.filter.Value()
	m.refreshLogContent(false)
	return m, cmd
}

func (m *model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.stopStream()
		return m, tea.Quit
	case "tab":
		m.focus = (m.focus + 1) % 4
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + 3) % 4
		return m, nil
	case "/":
		m.focus = focusFilters
		m.filterField = filterFieldText
		m.filter.Focus()
		return m, textinput.Blink
	}

	if m.focus == focusFilters {
		return m.updateFiltersKey(msg)
	}
	if m.focus == focusApps {
		if handled, cmd := m.updateAppFilterKey(msg); handled {
			return m, cmd
		}
	}

	switch msg.String() {
	case "r":
		m.status = "refreshing devices..."
		m.loadingDevice = true
		return m, m.loadDevicesCmd()
	case "c":
		m.buffer.Clear()
		m.refreshLogContent(false)
		return m, nil
	case "p":
		m.paused = !m.paused
		if m.paused {
			m.status = "paused"
		} else {
			m.status = "streaming"
			m.refreshLogContent(true)
		}
		return m, nil
	case "l":
		m.filters.Level = nextLevel(m.filters.Level)
		m.refreshLogContent(false)
		return m, nil
	case "o":
		m.filters.PIDOnly = !m.filters.PIDOnly
		m.refreshLogContent(false)
		return m, nil
	case "enter":
		return m.selectFocused()
	case "g":
		if m.focus == focusLogcat {
			m.logs.GotoTop()
		}
		return m, nil
	case "G":
		if m.focus == focusLogcat {
			m.logs.GotoBottom()
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.focus {
	case focusDevices:
		m.devices, cmd = m.devices.Update(msg)
	case focusApps:
		m.apps, cmd = m.apps.Update(msg)
	case focusLogcat:
		m.logs, cmd = m.logs.Update(msg)
	}
	return m, cmd
}

func (m *model) updateFiltersKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.filterField = (m.filterField + filterFieldCount - 1) % filterFieldCount
	case "down", "j":
		m.filterField = (m.filterField + 1) % filterFieldCount
	case "enter":
		if m.filterField == filterFieldText {
			m.filter.Focus()
			return m, textinput.Blink
		}
		m.toggleSelectedFilter()
	case "left", "right", " ":
		m.toggleSelectedFilter()
	}
	return m, nil
}

func (m *model) toggleSelectedFilter() {
	switch m.filterField {
	case filterFieldLevel:
		m.filters.Level = nextLevel(m.filters.Level)
		m.refreshLogContent(false)
	case filterFieldPIDOnly:
		m.filters.PIDOnly = !m.filters.PIDOnly
		m.refreshLogContent(false)
	}
}

func (m *model) selectFocused() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusDevices:
		item, ok := m.devices.SelectedItem().(domain.Device)
		if !ok {
			return m, nil
		}
		m.selectedDevice = &item
		m.selectedApp = nil
		m.filters.Package = ""
		m.filters.PID = ""
		m.allApps = nil
		m.appQuery = ""
		m.apps.SetItems(nil)
		m.stopStream()
		m.buffer.Clear()
		m.refreshLogContent(false)
		if item.State != "device" {
			m.status = item.Serial + " is " + item.State
			return m, nil
		}
		m.status = "loading apps..."
		m.loadingApps = true
		return m, m.loadAppsCmd(item.Serial)
	case focusApps:
		item, ok := m.apps.SelectedItem().(domain.Package)
		if !ok || m.selectedDevice == nil {
			return m, nil
		}
		m.selectedApp = &item
		m.filters.Package = item.Name
		m.filters.PID = ""
		m.stopStream()
		m.buffer.Clear()
		m.refreshLogContent(false)
		m.status = "starting logcat..."
		return m, m.startStreamCmd(m.selectedDevice.Serial, item.Name)
	}
	return m, nil
}

func (m *model) setDeviceItems(devices []domain.Device) {
	items := make([]list.Item, len(devices))
	for i, device := range devices {
		items[i] = device
	}
	m.devices.SetItems(items)
}

func (m *model) setAppItems(packages []domain.Package) {
	m.allApps = packages
	m.appQuery = ""
	m.applyAppFilter()
}

func (m *model) updateAppFilterKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		m.appQuery += string(msg.Runes)
		m.applyAppFilter()
		return true, nil
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.appQuery == "" {
			return false, nil
		}
		runes := []rune(m.appQuery)
		m.appQuery = string(runes[:len(runes)-1])
		m.applyAppFilter()
		return true, nil
	case tea.KeyEsc:
		if m.appQuery == "" {
			return false, nil
		}
		m.appQuery = ""
		m.applyAppFilter()
		return true, nil
	}
	return false, nil
}

func (m *model) applyAppFilter() {
	query := strings.ToLower(strings.TrimSpace(m.appQuery))
	filtered := make([]domain.Package, 0, len(m.allApps))
	for _, pkg := range m.allApps {
		if query == "" || strings.Contains(strings.ToLower(pkg.Name), query) {
			filtered = append(filtered, pkg)
		}
	}

	items := make([]list.Item, len(filtered))
	for i, pkg := range filtered {
		items[i] = pkg
	}
	m.apps.SetItems(items)
	m.apps.Select(0)
	if query == "" {
		m.status = fmt.Sprintf("%d app(s)", len(m.allApps))
		return
	}
	m.status = fmt.Sprintf("%d/%d app(s) match %q", len(filtered), len(m.allApps), m.appQuery)
}

func (m *model) stopStream() {
	if m.stream != nil {
		m.stream.Stop()
		m.stream = nil
	}
}

func (m *model) refreshLogContent(follow bool) {
	lines := m.buffer.Filtered(m.filters)
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, renderLogLine(line))
	}
	if len(rendered) == 0 {
		rendered = append(rendered, mutedStyle.Render("no logs"))
	}
	m.logs.SetContent(strings.Join(rendered, "\n"))
	if follow {
		m.logs.GotoBottom()
	}
}

func nextLevel(level string) string {
	switch level {
	case "":
		return "E"
	case "E":
		return "W"
	case "W":
		return "I"
	case "I":
		return "D"
	default:
		return ""
	}
}

func renderLogLine(line domain.LogLine) string {
	if line.Raw == "" {
		return ""
	}
	level := line.Level
	if level == "" {
		return mutedStyle.Render(line.Raw)
	}
	prefix := strings.TrimSpace(strings.Join([]string{line.Timestamp, level, line.Tag + ":", line.Message}, " "))
	if prefix == "" {
		prefix = line.Raw
	}
	return levelStyle(level).Render(prefix)
}

func levelStyle(level string) lipgloss.Style {
	switch strings.ToUpper(level) {
	case "E":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case "W":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case "I":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	case "D":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	default:
		return lipgloss.NewStyle()
	}
}
