package core

import (
	"sync"

	"github.com/c1r5/easycat/internal/domain"
)

const DefaultQueueSize = 2048

type Kind int

const (
	KindLog Kind = iota
	KindStreamStarted
	KindStreamStopped
)

type Event struct {
	Kind     Kind
	StreamID int
	Line     domain.LogLine
	Err      error
	Serial   string
	Package  string
	PID      string
}

type Bus struct {
	queueSize int

	mu   sync.Mutex
	next int
	subs map[int]chan Event
}

func NewBus(queueSize int) *Bus {
	if queueSize <= 0 {
		queueSize = DefaultQueueSize
	}
	return &Bus{
		queueSize: queueSize,
		subs:      map[int]chan Event{},
	}
}

type Subscription struct {
	Events <-chan Event
	cancel func()
}

func (s Subscription) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (b *Bus) Subscribe() Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	ch := make(chan Event, b.queueSize)
	b.subs[id] = ch
	return Subscription{
		Events: ch,
		cancel: func() { b.remove(id) },
	}
}

func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Bus) remove(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.subs[id]
	if !ok {
		return
	}
	delete(b.subs, id)
	close(ch)
}
