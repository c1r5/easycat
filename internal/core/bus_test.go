package core

import (
	"testing"
	"time"
)

func TestBusDeliversToSubscribersAndDropsWhenFull(t *testing.T) {
	bus := NewBus(2)
	first := bus.Subscribe()
	second := bus.Subscribe()
	defer first.Cancel()
	defer second.Cancel()

	bus.Publish(Event{Kind: KindLog, Line: LogLine{Raw: "a"}})
	bus.Publish(Event{Kind: KindStreamStopped, Err: nil})

	for _, sub := range []Subscription{first, second} {
		got := <-sub.Events
		if got.Kind != KindLog || got.Line.Raw != "a" {
			t.Fatalf("event = %+v, want log a", got)
		}
		got = <-sub.Events
		if got.Kind != KindStreamStopped || got.Err != nil {
			t.Fatalf("event = %+v, want stream stopped", got)
		}
	}

	tight := NewBus(1)
	sub := tight.Subscribe()
	tight.Publish(Event{Kind: KindLog, Line: LogLine{Raw: "kept"}})
	tight.Publish(Event{Kind: KindLog, Line: LogLine{Raw: "dropped"}})
	got := <-sub.Events
	if got.Line.Raw != "kept" {
		t.Fatalf("event = %+v, want kept", got)
	}
	select {
	case extra := <-sub.Events:
		t.Fatalf("full queue delivered %+v", extra)
	case <-time.After(20 * time.Millisecond):
	}

	sub.Cancel()
	_, ok := <-sub.Events
	if ok {
		t.Fatal("cancel left the subscription open")
	}
}
