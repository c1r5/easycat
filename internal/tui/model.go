package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/c1r5/easycat/internal/adb"
	"github.com/c1r5/easycat/internal/domain"
	localmcp "github.com/c1r5/easycat/internal/mcp"
	"github.com/c1r5/easycat/internal/observer"
	"github.com/c1r5/easycat/internal/rules"
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
	streamID int
	observer *observer.Observer
	mcp      *localmcp.Server
	allApps  []domain.Package
	appQuery string

	selectedDevice *domain.Device
	selectedApp    *domain.Package

	paused        bool
	loadingDevice bool
	loadingApps   bool
	status        string
	mcpEnabled    bool
	mcpStatus     string
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
	loadedRules, rulesErr := rules.Load(".easycat/rules.yaml")
	obs, obsErr := observer.New(observer.Options{Rules: loadedRules})
	status := "loading devices..."
	if rulesErr != nil {
		status = "rules failed: " + rulesErr.Error()
	}
	if obsErr != nil {
		status = "observer failed: " + obsErr.Error()
	}
	var mcpServer *localmcp.Server
	if obs != nil {
		mcpServer = localmcp.NewServer(localmcp.DefaultAddr, obs)
	}

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
		observer:    obs,
		mcp:         mcpServer,
		mcpEnabled:  true,
		mcpStatus:   "starting",
		status:      status,
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.loadDevicesCmd(), m.startMCPCmd(), waitObserverEventCmd(m.ctx, m.observer))
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
		if msg.streamID != m.streamID {
			if msg.stream != nil {
				msg.stream.Stop()
			}
			return m, nil
		}
		if msg.err != nil {
			m.status = "logcat failed: " + msg.err.Error()
			return m, nil
		}
		m.stream = msg.stream
		m.filters.PID = msg.pid
		m.filters.PIDOnly = msg.pid != ""
		if m.observer != nil {
			m.observer.Reset(observer.Context{
				Device:  selectedDeviceValue(m.selectedDevice),
				Package: selectedPackageName(m.selectedApp),
				PID:     msg.pid,
			})
			m.observer.Start(m.ctx)
		}
		m.status = "streaming from " + m.client.LogcatWindow()
		if msg.pid == "" {
			m.status = "streaming from " + m.client.LogcatWindow() + " without pid"
		}
		return m, tea.Batch(waitLogBatchCmd(msg.stream, msg.streamID), waitStreamDoneCmd(msg.stream, msg.streamID))
	case logBatchMsg:
		if msg.streamID != m.streamID || msg.stream != m.stream {
			return m, nil
		}
		for _, line := range msg.lines {
			m.buffer.Add(line)
			if m.observer != nil {
				m.observer.Publish(line)
			}
		}
		if !m.paused {
			m.refreshLogContent(true)
		}
		return m, waitLogBatchCmd(m.stream, m.streamID)
	case streamDoneMsg:
		if msg.streamID != m.streamID {
			return m, nil
		}
		m.stream = nil
		if msg.err != nil {
			m.status = "stream stopped: " + msg.err.Error()
		}
		return m, nil
	case observerEventMsg:
		if msg.event.Err != nil {
			m.status = "incident failed: " + msg.event.Err.Error()
		} else if msg.event.Incident.RuleID != "" {
			m.status = "[incident] " + msg.event.Incident.RuleID + " created"
		}
		return m, waitObserverEventCmd(m.ctx, m.observer)
	case mcpStartedMsg:
		if msg.err != nil {
			m.mcpStatus = "error"
			m.status = "mcp failed: " + msg.err.Error()
			return m, nil
		}
		m.mcpStatus = "on " + localmcp.DefaultAddr
		return m, nil
	case mcpStoppedMsg:
		if msg.err != nil {
			m.status = "mcp stop failed: " + msg.err.Error()
		}
		m.mcpStatus = "off"
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
		return m, tea.Batch(m.stopMCPCmd(), tea.Quit)
	case "m":
		m.mcpEnabled = !m.mcpEnabled
		if m.mcpEnabled {
			m.mcpStatus = "starting"
			return m, m.startMCPCmd()
		}
		m.mcpStatus = "stopping"
		return m, m.stopMCPCmd()
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
		m.focus = focusApps
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
		streamID := m.beginStream()
		return m, m.startStreamCmd(m.selectedDevice.Serial, item.Name, streamID)
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
	// ADB cancellation is asynchronous: a command that was already waiting on a
	// line or on process completion can still deliver one final Bubble Tea
	// message after Stop returns. Bumping the stream ID makes those late
	// messages harmless instead of letting an old logcat session repaint the
	// active UI or overwrite the footer status.
	m.streamID++
	if m.stream != nil {
		m.stream.Stop()
		m.stream = nil
	}
	if m.observer != nil {
		m.observer.Stop()
	}
}

func (m *model) beginStream() int {
	// Each start request gets its own identity before the adb command runs.
	// This also protects against a slow PID/logcat startup from an old app
	// selection racing with a newer selection.
	m.streamID++
	return m.streamID
}

func (m *model) refreshLogContent(follow bool) {
	lines := m.buffer.Filtered(m.filters)
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, renderLogLineWrapped(line, m.logs.Width)...)
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

func (m *model) startMCPCmd() tea.Cmd {
	return func() tea.Msg {
		if !m.mcpEnabled || m.mcp == nil {
			return nil
		}
		return mcpStartedMsg{err: m.mcp.Start()}
	}
}

func (m *model) stopMCPCmd() tea.Cmd {
	return func() tea.Msg {
		if m.mcp == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return mcpStoppedMsg{err: m.mcp.Stop(ctx)}
	}
}

func selectedDeviceValue(device *domain.Device) domain.Device {
	if device == nil {
		return domain.Device{}
	}
	return *device
}

func selectedPackageName(pkg *domain.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Name
}

func renderLogLine(line domain.LogLine) string {
	if line.Raw == "" {
		return ""
	}
	level := line.Level
	if level == "" {
		return mutedStyle.Render(sanitizeTerminalText(line.Raw))
	}
	prefix := strings.TrimSpace(strings.Join([]string{
		sanitizeTerminalText(line.Timestamp),
		sanitizeTerminalText(level),
		sanitizeTerminalText(line.Tag) + ":",
		sanitizeTerminalText(line.Message),
	}, " "))
	if prefix == "" {
		prefix = sanitizeTerminalText(line.Raw)
	}
	return levelStyle(level).Render(prefix)
}

func sanitizeTerminalText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r':
			b.WriteRune(' ')
		case r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// Logcat is external input. Control bytes such as ESC, BEL and BS
			// are data here, not terminal commands; dropping them prevents a log
			// line from moving the cursor, clearing cells, or visually escaping
			// the viewport/footer containers.
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func renderLogLineWrapped(line domain.LogLine, width int) []string {
	rendered := renderLogLine(line)
	if width <= 0 {
		return []string{rendered}
	}
	wrapped := ansi.Wrap(rendered, width, "")
	if wrapped == "" {
		return []string{""}
	}
	return strings.Split(wrapped, "\n")
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
