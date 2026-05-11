package incidents

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/rules"
)

const DefaultDir = ".easycat/incidents"

type Incident struct {
	ID          string           `json:"id"`
	RuleID      string           `json:"rule_id"`
	Description string           `json:"description"`
	Device      domain.Device    `json:"device"`
	Package     string           `json:"package"`
	PID         string           `json:"pid"`
	Trigger     domain.LogLine   `json:"trigger"`
	Context     []domain.LogLine `json:"context"`
	Matches     []domain.LogLine `json:"matches"`
	CreatedAt   time.Time        `json:"created_at"`
	Path        string           `json:"path"`
}

type Metadata struct {
	Device  domain.Device
	Package string
	PID     string
}

type Store struct {
	Dir string
}

func NewStore(dir string) Store {
	if dir == "" {
		dir = DefaultDir
	}
	return Store{Dir: dir}
}

func NewIncident(trigger rules.Trigger, meta Metadata, context []domain.LogLine, createdAt time.Time) Incident {
	id := fmt.Sprintf("%s_%s", sanitizeFilePart(trigger.Rule.ID), createdAt.UTC().Format("20060102T150405Z"))
	return Incident{
		ID:          id,
		RuleID:      trigger.Rule.ID,
		Description: trigger.Rule.Description,
		Device:      meta.Device,
		Package:     meta.Package,
		PID:         meta.PID,
		Trigger:     trigger.Line,
		Context:     append([]domain.LogLine(nil), context...),
		Matches:     append([]domain.LogLine(nil), trigger.Matches...),
		CreatedAt:   createdAt.UTC(),
	}
}

func (s Store) Write(incident Incident) (Incident, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return incident, err
	}
	path := filepath.Join(s.Dir, incident.ID+".md")
	incident.Path = path
	return incident, os.WriteFile(path, []byte(RenderMarkdown(incident)), 0o644)
}

func RenderMarkdown(i Incident) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Incident: %s\n\n", i.RuleID)
	fmt.Fprintf(&b, "Created: %s\n\n", i.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "## Rule\n\n%s", i.RuleID)
	if i.Description != "" {
		fmt.Fprintf(&b, " - %s", i.Description)
	}
	b.WriteString("\n\n## Device\n\n")
	fmt.Fprintf(&b, "- Serial: %s\n", i.Device.Serial)
	fmt.Fprintf(&b, "- Model: %s\n", i.Device.Model)
	fmt.Fprintf(&b, "- Package: %s\n", i.Package)
	fmt.Fprintf(&b, "- PID: %s\n", i.PID)
	b.WriteString("\n## Trigger\n\n```text\n")
	b.WriteString(logText(i.Trigger))
	b.WriteString("\n```\n\n## Matching Logs\n\n```text\n")
	for _, line := range i.Matches {
		b.WriteString(logText(line))
		b.WriteByte('\n')
	}
	b.WriteString("```\n\n## Context Logs\n\n```text\n")
	for _, line := range i.Context {
		b.WriteString(logText(line))
		b.WriteByte('\n')
	}
	b.WriteString("```\n\n## Suggested Prompt\n\n")
	b.WriteString("Analyze the probable root cause of this Android logcat incident. Identify the failing component, likely code paths, and a safe correction plan. Do not modify code automatically.\n")
	return b.String()
}

func logText(line domain.LogLine) string {
	if strings.TrimSpace(line.Raw) != "" {
		return line.Raw
	}
	return strings.TrimSpace(strings.Join([]string{line.Timestamp, line.PID, line.Level, line.Tag + ":", line.Message}, " "))
}

func sanitizeFilePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	value = re.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "incident"
	}
	return value
}
