package observer

import (
	"context"
	"sync"
	"time"

	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/incidents"
	"github.com/c1r5/easycat/internal/rules"
)

const (
	DefaultQueueSize    = 2048
	DefaultContextLines = 200
)

type Context struct {
	Device  domain.Device `json:"device"`
	Package string        `json:"package"`
	PID     string        `json:"pid"`
}

type Event struct {
	Incident incidents.Incident
	Err      error
}

type Options struct {
	Rules        []rules.Rule
	Store        incidents.Store
	QueueSize    int
	ContextLines int
	Now          func() time.Time
}

type Observer struct {
	engine *rules.Engine
	store  incidents.Store
	now    func() time.Time

	input  chan domain.LogLine
	events chan Event

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu           sync.RWMutex
	meta         Context
	recent       *domain.LogBuffer
	created      []incidents.Incident
	contextLines int
}

func New(options Options) (*Observer, error) {
	engine, err := rules.NewEngine(options.Rules)
	if err != nil {
		return nil, err
	}
	queueSize := options.QueueSize
	if queueSize <= 0 {
		queueSize = DefaultQueueSize
	}
	contextLines := options.ContextLines
	if contextLines <= 0 {
		contextLines = DefaultContextLines
	}
	store := options.Store
	if store.Dir == "" {
		store = incidents.NewStore("")
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Observer{
		engine:       engine,
		store:        store,
		now:          now,
		input:        make(chan domain.LogLine, queueSize),
		events:       make(chan Event, queueSize),
		recent:       domain.NewLogBuffer(contextLines),
		contextLines: contextLines,
	}, nil
}

func (o *Observer) Start(ctx context.Context) {
	o.Stop()
	o.ctx, o.cancel = context.WithCancel(ctx)
	o.wg.Add(1)
	go o.run()
}

func (o *Observer) Stop() {
	if o.cancel != nil {
		o.cancel()
		o.wg.Wait()
		o.cancel = nil
	}
}

func (o *Observer) Events() <-chan Event {
	return o.events
}

func (o *Observer) Publish(line domain.LogLine) bool {
	select {
	case o.input <- line:
		return true
	default:
		return false
	}
}

func (o *Observer) Reset(meta Context) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.meta = meta
	o.recent.Clear()
	o.engine.Reset()
	o.drainInput()
}

func (o *Observer) Snapshot() Snapshot {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return Snapshot{
		Context:   o.meta,
		Logs:      o.recent.Lines(),
		Incidents: append([]incidents.Incident(nil), o.created...),
	}
}

func (o *Observer) GetIncident(id string) (incidents.Incident, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, incident := range o.created {
		if incident.ID == id {
			return incident, true
		}
	}
	return incidents.Incident{}, false
}

type Snapshot struct {
	Context   Context              `json:"context"`
	Logs      []domain.LogLine     `json:"logs"`
	Incidents []incidents.Incident `json:"incidents"`
}

func (o *Observer) run() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			return
		case line := <-o.input:
			o.process(line)
		}
	}
}

func (o *Observer) process(line domain.LogLine) {
	now := o.now()
	o.mu.Lock()
	o.recent.Add(line)
	meta := o.meta
	scope := meta.Device.Serial + "|" + meta.Package + "|" + meta.PID
	triggers := o.engine.Observe(line, now, scope)
	contextLines := o.recent.Lines()
	o.mu.Unlock()

	for _, trigger := range triggers {
		incident := incidents.NewIncident(trigger, incidents.Metadata{
			Device:  meta.Device,
			Package: meta.Package,
			PID:     meta.PID,
		}, contextLines, now)
		written, err := o.store.Write(incident)
		o.mu.Lock()
		if err == nil {
			o.created = append(o.created, written)
		}
		o.mu.Unlock()
		o.emit(Event{Incident: written, Err: err})
	}
}

func (o *Observer) emit(event Event) {
	select {
	case o.events <- event:
	default:
	}
}

func (o *Observer) drainInput() {
	for {
		select {
		case <-o.input:
		default:
			return
		}
	}
}
