package rules

import (
	"testing"
	"time"

	"github.com/c1r5/easycat/internal/domain"
)

func TestRuleMatchesContainsCaseInsensitiveAndLevelExact(t *testing.T) {
	rule := defaultRule("fatal_exception", "Fatal", []string{"fatal exception"})
	line := domain.LogLine{Level: "E", Raw: "FATAL EXCEPTION: main"}
	if !rule.Matches(line) {
		t.Fatal("expected rule to match")
	}
	line.Level = "W"
	if rule.Matches(line) {
		t.Fatal("expected level to reject line")
	}
}

func TestEngineThresholdAndCooldown(t *testing.T) {
	engine, err := NewEngine([]Rule{{
		ID: "burst",
		Match: MatchConfig{
			Level:    "E",
			Contains: []string{"boom"},
		},
		Threshold: ThresholdConfig{Count: 2, Window: time.Minute, Cooldown: 5 * time.Minute},
		Action:    ActionConfig{Type: "write_incident"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	line := domain.LogLine{Level: "E", Raw: "boom"}
	if got := engine.Observe(line, now, "scope"); len(got) != 0 {
		t.Fatalf("got trigger before threshold: %+v", got)
	}
	if got := engine.Observe(line, now.Add(time.Second), "scope"); len(got) != 1 {
		t.Fatalf("got %d triggers, want 1", len(got))
	}
	if got := engine.Observe(line, now.Add(2*time.Second), "scope"); len(got) != 0 {
		t.Fatalf("cooldown should suppress trigger: %+v", got)
	}
	if got := engine.Observe(line, now.Add(6*time.Minute), "scope"); len(got) != 0 {
		t.Fatalf("first post-cooldown line should rebuild threshold, got %d triggers", len(got))
	}
	if got := engine.Observe(line, now.Add(6*time.Minute+time.Second), "scope"); len(got) != 1 {
		t.Fatalf("cooldown should expire, got %d triggers", len(got))
	}
}

func TestLoadMissingFileUsesDefaultRules(t *testing.T) {
	loaded, err := Load(t.TempDir() + "/missing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) == 0 {
		t.Fatal("expected default rules")
	}
}
