package incidents

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/rules"
)

func TestRenderMarkdownIncludesIncidentSections(t *testing.T) {
	incident := NewIncident(rules.Trigger{
		Rule:    rules.Rule{ID: "sqlite_locked", Description: "SQLite lock"},
		Line:    domain.LogLine{Raw: "database is locked"},
		Matches: []domain.LogLine{{Raw: "database is locked"}},
	}, Metadata{
		Device:  domain.Device{Serial: "R9XX", Model: "Pixel"},
		Package: "com.example",
		PID:     "1234",
	}, []domain.LogLine{{Raw: "before"}}, time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC))

	rendered := RenderMarkdown(incident)
	for _, want := range []string{"# Incident: sqlite_locked", "## Trigger", "database is locked", "## Suggested Prompt"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in markdown:\n%s", want, rendered)
		}
	}
}

func TestStoreWriteCreatesMarkdownFile(t *testing.T) {
	store := NewStore(t.TempDir())
	incident := Incident{
		ID:        "sqlite_locked_20260509T120000Z",
		RuleID:    "sqlite_locked",
		Trigger:   domain.LogLine{Raw: "database is locked"},
		CreatedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	}
	written, err := store.Write(incident)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(written.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sqlite_locked") {
		t.Fatalf("unexpected file content: %s", data)
	}
}
