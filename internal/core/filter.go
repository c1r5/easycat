package core

import (
	"container/ring"
	"strings"
)

const MaxLogLines = 10000

type Filters struct {
	Package string
	Text    string
	Level   string
	PIDOnly bool
	PID     string
}

func (f Filters) Match(line LogLine) bool {
	if f.Level != "" && !strings.EqualFold(line.Level, f.Level) {
		return false
	}
	if f.PIDOnly && f.PID != "" && line.PID != f.PID {
		return false
	}
	if f.Text != "" && !strings.Contains(strings.ToLower(line.Raw), strings.ToLower(f.Text)) {
		return false
	}
	return true
}

type LogBuffer struct {
	r     *ring.Ring
	count int
	limit int
}

func NewLogBuffer(limit int) *LogBuffer {
	if limit <= 0 {
		limit = MaxLogLines
	}
	return &LogBuffer{r: ring.New(limit), limit: limit}
}

func (b *LogBuffer) Add(line LogLine) {
	b.r.Value = line
	b.r = b.r.Next()
	if b.count < b.limit {
		b.count++
	}
}

func (b *LogBuffer) Clear() {
	b.r = ring.New(b.limit)
	b.count = 0
}

func (b *LogBuffer) Lines() []LogLine {
	lines := make([]LogLine, 0, b.count)
	if b.count == 0 {
		return lines
	}

	start := b.r
	if b.count < b.limit {
		start = b.r.Move(-b.count)
	}
	for i := 0; i < b.count; i++ {
		if line, ok := start.Value.(LogLine); ok {
			lines = append(lines, line)
		}
		start = start.Next()
	}
	return lines
}

func (b *LogBuffer) Filtered(filters Filters) []LogLine {
	lines := b.Lines()
	filtered := make([]LogLine, 0, len(lines))
	for _, line := range lines {
		if filters.Match(line) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}
