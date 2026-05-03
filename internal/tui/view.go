package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	filterHeight = 6
	deviceHeight = 8
	footerHeight = 3
)

var (
	basePanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	activeStyle    = basePanelStyle.BorderForeground(lipgloss.Color("39"))
	inactiveStyle  = basePanelStyle.BorderForeground(lipgloss.Color("240"))
	titleStyle     = lipgloss.NewStyle().Bold(true)
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func (m *model) View() string {
	if m.width == 0 || m.height == 0 {
		return "easycat"
	}

	layout := m.layout()

	leftPanels := []string{}
	if layout.filterHeight > 0 {
		leftPanels = append(leftPanels, m.panel("Filters", m.filtersView(), layout.leftWidth, layout.filterHeight, m.focus == focusFilters))
	}
	if layout.deviceHeight > 0 {
		leftPanels = append(leftPanels, m.panel("Devices", m.devices.View(), layout.leftWidth, layout.deviceHeight, m.focus == focusDevices))
	}
	if layout.appHeight > 0 {
		leftPanels = append(leftPanels, m.panel("Apps", m.apps.View(), layout.leftWidth, layout.appHeight, m.focus == focusApps))
	}
	left := lipgloss.JoinVertical(lipgloss.Left, leftPanels...)
	right := m.panel("Logcat", m.logs.View(), layout.rightWidth, layout.bodyHeight, m.focus == focusLogcat)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	footer := m.footerView(layout.totalWidth, layout.footerHeight)
	view := lipgloss.JoinVertical(lipgloss.Left, body, footer)
	return fitBlock(view, layout.totalWidth, layout.totalHeight)
}

func (m *model) resize() {
	layout := m.layout()
	contentWidth, _ := panelContentSize(basePanelStyle, layout.leftWidth, layout.bodyHeight)

	innerLeft := max(1, contentWidth)
	m.devices.SetSize(innerLeft, listContentHeight(layout.deviceHeight))
	m.apps.SetSize(innerLeft, listContentHeight(layout.appHeight))
	m.logs.Width, m.logs.Height = panelContentSize(basePanelStyle, layout.rightWidth, layout.bodyHeight)
	m.logs.Height = max(1, m.logs.Height-1)
	m.filter.Width = max(8, innerLeft-8)
}

func (m *model) panel(title, content string, width, height int, active bool) string {
	style := inactiveStyle
	if active {
		style = activeStyle
	}
	styleWidth, styleHeight := panelStyleSize(style, width, height)
	contentWidth, contentHeight := panelContentSize(style, width, height)
	titleLine := ansi.Truncate(titleStyle.Render(" "+title+" "), contentWidth, "...")
	body := fitBlock(content, contentWidth, max(0, contentHeight-1))
	return style.Width(styleWidth).Height(styleHeight).Render(titleLine + "\n" + body)
}

func (m *model) filtersView() string {
	level := m.filters.Level
	if level == "" {
		level = "E/W/I/D"
	}
	pidOnly := "off"
	if m.filters.PIDOnly {
		pidOnly = "on"
	}
	pkg := m.filters.Package
	if pkg == "" {
		pkg = mutedStyle.Render("(none)")
	}
	pid := ""
	if m.filters.PID != "" {
		pid = " " + mutedStyle.Render("pid "+m.filters.PID)
	}
	return strings.Join([]string{
		"Package: " + pkg,
		"Text: " + m.filter.View(),
		"Level: " + level,
		"PID only: " + pidOnly + pid,
	}, "\n")
}

func (m *model) footerView(width, height int) string {
	state := "idle"
	if m.stream != nil {
		state = "streaming"
	}
	if m.paused {
		state = "paused"
	}
	help := "tab: focus | enter: select | /: filter | l: level | o: pid | r: refresh | c: clear | p: pause | q: quit"
	status := fmt.Sprintf("%s | %s | %s", time.Now().Format("15:04:05"), state, m.status)
	content := lipgloss.JoinVertical(lipgloss.Left, help, statusStyle.Render(status))
	styleWidth, styleHeight := panelStyleSize(inactiveStyle, width, height)
	contentWidth, contentHeight := panelContentSize(inactiveStyle, width, height)
	return inactiveStyle.Width(styleWidth).Height(styleHeight).Render(
		fitBlock(content, contentWidth, contentHeight),
	)
}

type layoutSize struct {
	totalWidth   int
	totalHeight  int
	leftWidth    int
	rightWidth   int
	bodyHeight   int
	filterHeight int
	deviceHeight int
	appHeight    int
	footerHeight int
}

func (m *model) layout() layoutSize {
	totalWidth := max(1, m.width)
	totalHeight := max(1, m.height)

	footer := footerHeight
	if totalHeight < 12 {
		footer = 2
	}
	if footer >= totalHeight {
		footer = 1
	}
	body := max(1, totalHeight-footer-1)

	left := int(float64(totalWidth) * 0.30)
	if totalWidth >= 90 {
		left = clamp(left, 28, 42)
	} else if totalWidth >= 50 {
		left = clamp(left, 18, totalWidth-24)
	} else {
		left = clamp(left, max(8, totalWidth/3), max(8, totalWidth-12))
	}
	right := max(1, totalWidth-left)

	filters, devices, apps := splitLeftHeights(body)

	return layoutSize{
		totalWidth:   totalWidth,
		totalHeight:  totalHeight,
		leftWidth:    left,
		rightWidth:   right,
		bodyHeight:   body,
		filterHeight: filters,
		deviceHeight: devices,
		appHeight:    apps,
		footerHeight: footer,
	}
}

func splitLeftHeights(body int) (filters, devices, apps int) {
	if body <= 1 {
		return body, 0, 0
	}
	if body <= 6 {
		filters = min(3, body)
		devices = min(2, max(0, body-filters))
		apps = max(0, body-filters-devices)
		return filters, devices, apps
	}

	filters = min(filterHeight, body)
	remaining := body - filters
	devices = min(deviceHeight, remaining)
	apps = remaining - devices

	if apps < 4 && body > 10 {
		need := 4 - apps
		take := min(need, max(0, devices-4))
		devices -= take
		apps += take
		need -= take
		take = min(need, max(0, filters-5))
		filters -= take
		apps += take
	}

	return filters, devices, apps
}

func listContentHeight(panelHeight int) int {
	_, contentHeight := panelContentSize(basePanelStyle, 1, panelHeight)
	return max(1, contentHeight-1)
}

func panelStyleSize(style lipgloss.Style, width, height int) (int, int) {
	frameX, frameY := style.GetFrameSize()
	paddingX := style.GetHorizontalPadding()
	paddingY := style.GetVerticalPadding()
	borderX := frameX - paddingX
	borderY := frameY - paddingY
	return max(1, width-borderX), max(1, height-borderY)
}

func panelContentSize(style lipgloss.Style, width, height int) (int, int) {
	styleWidth, styleHeight := panelStyleSize(style, width, height)
	return max(1, styleWidth-style.GetHorizontalPadding()), max(1, styleHeight-style.GetVerticalPadding())
}

func fitBlock(content string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	fitted := make([]string, 0, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		fitted = append(fitted, ansi.Truncate(line, width, "..."))
	}
	return strings.Join(fitted, "\n")
}

func clamp(v, low, high int) int {
	if high < low {
		return low
	}
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
