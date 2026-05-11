package rules

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c1r5/easycat/internal/domain"
	"gopkg.in/yaml.v3"
)

const DefaultCooldown = 5 * time.Minute

type Config struct {
	Rules []Rule `yaml:"rules"`
}

type Rule struct {
	ID          string          `yaml:"id"`
	Description string          `yaml:"description"`
	Match       MatchConfig     `yaml:"match"`
	Threshold   ThresholdConfig `yaml:"threshold"`
	Action      ActionConfig    `yaml:"action"`
}

type MatchConfig struct {
	Level    string   `yaml:"level"`
	PID      string   `yaml:"pid"`
	Contains []string `yaml:"contains"`
}

type ThresholdConfig struct {
	Count    int           `yaml:"count"`
	Window   time.Duration `yaml:"window"`
	Cooldown time.Duration `yaml:"cooldown"`
}

func (c *ThresholdConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawThreshold struct {
		Count    int    `yaml:"count"`
		Window   string `yaml:"window"`
		Cooldown string `yaml:"cooldown"`
	}
	var raw rawThreshold
	if err := value.Decode(&raw); err != nil {
		return err
	}
	window, err := parseOptionalDuration(raw.Window)
	if err != nil {
		return fmt.Errorf("window: %w", err)
	}
	cooldown, err := parseOptionalDuration(raw.Cooldown)
	if err != nil {
		return fmt.Errorf("cooldown: %w", err)
	}
	c.Count = raw.Count
	c.Window = window
	c.Cooldown = cooldown
	return nil
}

type ActionConfig struct {
	Type string `yaml:"type"`
}

type Trigger struct {
	Rule    Rule
	Line    domain.LogLine
	Matches []domain.LogLine
}

type Engine struct {
	rules     []Rule
	windows   map[string][]timedLine
	cooldowns map[string]time.Time
}

type timedLine struct {
	at   time.Time
	line domain.LogLine
}

func Load(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultRules(), nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Rules) == 0 {
		return DefaultRules(), nil
	}
	if err := Validate(cfg.Rules); err != nil {
		return nil, err
	}
	return cfg.Rules, nil
}

func Validate(rules []Rule) error {
	seen := map[string]bool{}
	for _, rule := range rules {
		if strings.TrimSpace(rule.ID) == "" {
			return errors.New("rule id is required")
		}
		if seen[rule.ID] {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		seen[rule.ID] = true
		if rule.Threshold.Count <= 0 {
			return fmt.Errorf("rule %q threshold count must be positive", rule.ID)
		}
		if rule.Threshold.Window <= 0 {
			return fmt.Errorf("rule %q threshold window must be positive", rule.ID)
		}
		if strings.TrimSpace(rule.Action.Type) == "" {
			return fmt.Errorf("rule %q action type is required", rule.ID)
		}
		if rule.Action.Type != "write_incident" {
			return fmt.Errorf("rule %q unsupported action type %q", rule.ID, rule.Action.Type)
		}
	}
	return nil
}

func NewEngine(rules []Rule) (*Engine, error) {
	if len(rules) == 0 {
		rules = DefaultRules()
	}
	if err := Validate(rules); err != nil {
		return nil, err
	}
	return &Engine{
		rules:     rules,
		windows:   map[string][]timedLine{},
		cooldowns: map[string]time.Time{},
	}, nil
}

func (e *Engine) Reset() {
	e.windows = map[string][]timedLine{}
	e.cooldowns = map[string]time.Time{}
}

func (e *Engine) Observe(line domain.LogLine, now time.Time, scope string) []Trigger {
	triggers := []Trigger{}
	for _, rule := range e.rules {
		if !rule.Matches(line) {
			continue
		}
		key := rule.ID + "|" + scope
		window := append(e.windows[key], timedLine{at: now, line: line})
		cutoff := now.Add(-rule.Threshold.Window)
		kept := window[:0]
		for _, item := range window {
			if !item.at.Before(cutoff) {
				kept = append(kept, item)
			}
		}
		e.windows[key] = kept
		if len(kept) < rule.Threshold.Count {
			continue
		}
		cooldown := rule.Threshold.Cooldown
		if cooldown <= 0 {
			cooldown = DefaultCooldown
		}
		if last, ok := e.cooldowns[key]; ok && now.Sub(last) < cooldown {
			continue
		}
		e.cooldowns[key] = now
		matches := make([]domain.LogLine, 0, len(kept))
		for _, item := range kept {
			matches = append(matches, item.line)
		}
		triggers = append(triggers, Trigger{Rule: rule, Line: line, Matches: matches})
	}
	return triggers
}

func (r Rule) Matches(line domain.LogLine) bool {
	if r.Match.Level != "" && strings.ToUpper(line.Level) != strings.ToUpper(r.Match.Level) {
		return false
	}
	if r.Match.PID != "" && line.PID != r.Match.PID {
		return false
	}
	if len(r.Match.Contains) == 0 {
		return true
	}
	haystack := strings.ToLower(line.Raw + "\n" + line.Message)
	for _, needle := range r.Match.Contains {
		if strings.Contains(haystack, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func DefaultRules() []Rule {
	return []Rule{
		defaultRule("fatal_exception", "Android fatal exception", []string{"FATAL EXCEPTION"}),
		defaultRule("null_pointer_exception", "Null pointer exception", []string{"NullPointerException", "null pointer"}),
		defaultRule("sqlite_locked", "SQLite database lock", []string{"database is locked", "SQLiteDatabaseLockedException"}),
		defaultRule("anr_detected", "Application not responding", []string{"ANR in", "Application Not Responding"}),
	}
}

func defaultRule(id, description string, contains []string) Rule {
	return Rule{
		ID:          id,
		Description: description,
		Match:       MatchConfig{Level: "E", Contains: contains},
		Threshold:   ThresholdConfig{Count: 1, Window: 30 * time.Second, Cooldown: DefaultCooldown},
		Action:      ActionConfig{Type: "write_incident"},
	}
}

func parseOptionalDuration(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}
