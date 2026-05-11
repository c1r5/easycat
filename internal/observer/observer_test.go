package observer

import (
	"context"
	"testing"
	"time"

	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/incidents"
	"github.com/c1r5/easycat/internal/rules"
)

func TestPublishDropsWhenQueueFull(t *testing.T) {
	obs, err := New(Options{QueueSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !obs.Publish(domain.LogLine{Raw: "one"}) {
		t.Fatal("first publish should fit")
	}
	if obs.Publish(domain.LogLine{Raw: "two"}) {
		t.Fatal("second publish should drop without blocking")
	}
}

func TestObserverCreatesIncident(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	obs, err := New(Options{
		Rules: []rules.Rule{{
			ID:        "sqlite_locked",
			Match:     rules.MatchConfig{Level: "E", Contains: []string{"database is locked"}},
			Threshold: rules.ThresholdConfig{Count: 1, Window: time.Minute, Cooldown: time.Minute},
			Action:    rules.ActionConfig{Type: "write_incident"},
		}},
		Store: incidents.NewStore(t.TempDir()),
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	obs.Reset(Context{Device: domain.Device{Serial: "device"}, Package: "com.example", PID: "1234"})
	obs.Start(context.Background())
	defer obs.Stop()

	if !obs.Publish(domain.LogLine{Level: "E", Raw: "database is locked"}) {
		t.Fatal("publish dropped")
	}

	select {
	case event := <-obs.Events():
		if event.Err != nil {
			t.Fatal(event.Err)
		}
		if event.Incident.RuleID != "sqlite_locked" {
			t.Fatalf("unexpected incident: %+v", event.Incident)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for incident")
	}
}
