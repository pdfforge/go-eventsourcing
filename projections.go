package eventsourcing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hallgren/eventsourcing/core"
)

type fetchFunc func() (core.Iterator, error)
type callbackFunc func(e Event) error

type ProjectionHandler struct {
	register *Register
	Encoder  encoder
	count    int
}

func NewProjectionHandler(register *Register, encoder encoder) *ProjectionHandler {
	return &ProjectionHandler{
		register: register,
		Encoder:  encoder,
	}
}

type Projection struct {
	fetchF    fetchFunc
	callbackF callbackFunc
	handler   *ProjectionHandler
	trigger   chan func()
	Strict    bool // Strict indicate if the projection should return error if the event it fetches is not found in the register
	Name      string
}

// Group runs projections concurrently
type Group struct {
	Pace        time.Duration // Pace is used when a projection is running and it reaches the end of the event stream
	handler     *ProjectionHandler
	projections []*Projection
	cancelF     context.CancelFunc
	wg          sync.WaitGroup
	ErrChan     chan error
	started     bool
}

// ProjectionResult is the return type for a Group and Race
type ProjectionResult struct {
	Error            error
	Name             string
	LastHandledEvent Event
}

// Projection creates a projection that will run down an event stream
func (ph *ProjectionHandler) Projection(fetchF fetchFunc, callbackF callbackFunc) *Projection {
	projection := Projection{
		fetchF:    fetchF,
		callbackF: callbackF,
		handler:   ph,
		trigger:   make(chan func()),
		Strict:    true,                        // Default strict is active
		Name:      fmt.Sprintf("%d", ph.count), // Default the name to it's creation index
	}
	ph.count++
	return &projection
}

// triggerAsync force a running projection to run immediately independent on the pace
// It will return immediately after triggering the prjection to run.
// TriggerAsync will deadlock if the projection is not running.
func (p *Projection) triggerAsync() {
	p.trigger <- func() {}
}

// triggerSync force a running projection to run immediately independent on the pace
// It will wait for the projection to finish running to its current end before returning.
// TriggerAsync will deadlock if the projection is not running.
func (p *Projection) triggerSync() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	f := func() {
		wg.Done()
	}
	p.trigger <- f
	wg.Wait()
}

// Run runs the projection forever until the context is cancelled. When there are no more events to consume it
// waits for a trigger or context cancel.
func (p *Projection) run(ctx context.Context, pace time.Duration) error {
	var noopFunc = func() {}
	var f = noopFunc
	triggerFunc := func() {
		f()
		// reset the f to the noop func
		f = noopFunc
	}
	for {
		result := p.RunToEnd(ctx)
		triggerFunc()
		if result.Error != nil {
			return result.Error
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pace):
		case f = <-p.trigger:
		}
	}
}

// RunToEnd runs until the projection reaches the end of the event stream
func (p *Projection) RunToEnd(ctx context.Context) ProjectionResult {
	var result ProjectionResult
	var lastHandledEvent Event

	for {
		select {
		case <-ctx.Done():
			return ProjectionResult{Error: ctx.Err(), Name: result.Name, LastHandledEvent: result.LastHandledEvent}
		default:
			ran, result := p.RunOnce()
			// if the first event returned error or if it did not run at all
			if result.LastHandledEvent.GlobalVersion() == 0 {
				result.LastHandledEvent = lastHandledEvent
			}
			if result.Error != nil {
				return result
			}
			// hit the end of the event stream
			if !ran {
				return result
			}
			lastHandledEvent = result.LastHandledEvent
		}
	}
}

// RunOnce runs the fetch method one time
func (p *Projection) RunOnce() (bool, ProjectionResult) {
	// ran indicate if there were events to fetch
	var ran bool
	var lastHandledEvent Event

	iterator, err := p.fetchF()
	if err != nil {
		return false, ProjectionResult{Error: err, Name: p.Name, LastHandledEvent: lastHandledEvent}
	}
	defer iterator.Close()

	for iterator.Next() {
		ran = true
		event, err := iterator.Value()
		if err != nil {
			return false, ProjectionResult{Error: err, Name: p.Name, LastHandledEvent: lastHandledEvent}
		}

		// TODO: is only registered events of interest?
		f, found := p.handler.register.EventRegistered(event)
		if !found {
			if p.Strict {
				err = fmt.Errorf("event not registered aggregate type: %s, reason: %s, global version: %d, %w", event.AggregateType, event.Reason, event.GlobalVersion, ErrEventNotRegistered)
				return false, ProjectionResult{Error: err, Name: p.Name, LastHandledEvent: lastHandledEvent}
			}
			continue
		}

		data := f()
		err = p.handler.Encoder.Deserialize(event.Data, &data)
		if err != nil {
			return false, ProjectionResult{Error: err, Name: p.Name, LastHandledEvent: lastHandledEvent}
		}

		metadata := make(map[string]interface{})
		if event.Metadata != nil {
			err = p.handler.Encoder.Deserialize(event.Metadata, &metadata)
			if err != nil {
				return false, ProjectionResult{Error: err, Name: p.Name, LastHandledEvent: lastHandledEvent}
			}
		}
		e := NewEvent(event, data, metadata)

		err = p.callbackF(e)
		if err != nil {
			return false, ProjectionResult{Error: err, Name: p.Name, LastHandledEvent: lastHandledEvent}
		}
		// keep a reference to the last successfully handled event
		lastHandledEvent = e
	}
	return ran, ProjectionResult{Error: nil, Name: p.Name, LastHandledEvent: lastHandledEvent}
}

// Group runs a group of projections concurrently
func (ph *ProjectionHandler) Group(projections ...*Projection) *Group {
	return &Group{
		handler:     ph,
		projections: projections,
		cancelF:     func() {},
		ErrChan:     make(chan error),
		Pace:        time.Second * 10, // Default pace 10 seconds
	}
}

// Start starts all projectinos in the group, an error channel i created on the group to notify
// if a result containing an error is returned from a projection
func (g *Group) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelF = cancel

	g.wg.Add(len(g.projections))
	for _, projection := range g.projections {
		go func(p *Projection) {
			defer g.wg.Done()
			err := p.run(ctx, g.Pace)
			if !errors.Is(err, context.Canceled) {
				g.ErrChan <- err
			}
		}(projection)
	}
	g.started = true
}

// TriggerAsync force all projections to run not waiting for them to finish
func (g *Group) TriggerAsync() {
	if !g.started {
		return
	}

	for _, projection := range g.projections {
		projection.triggerAsync()
	}
}

// TriggerSync force all projections to run and wait for them to finish
func (g *Group) TriggerSync() {
	if !g.started {
		return
	}

	wg := sync.WaitGroup{}
	for _, projection := range g.projections {
		wg.Add(1)
		go func(p *Projection) {
			p.triggerSync()
			wg.Done()
		}(projection)
	}
	wg.Wait()
}

// Stop terminate all projections in the group
func (g *Group) Stop() {
	if !g.started {
		return
	}
	g.cancelF()

	// return when all projections has stopped
	g.wg.Wait()

	// close the error channel
	close(g.ErrChan)

	g.started = false
}

// Race runs the projections to the end of the events streams.
// Can be used on a stale event stream with no more events coming in or when you want to know when all projections are done.
func (p *ProjectionHandler) Race(cancelOnError bool, projections ...*Projection) ([]ProjectionResult, error) {
	var lock sync.Mutex
	var causingErr error

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(len(projections))

	results := make([]ProjectionResult, len(projections))
	for i, projection := range projections {
		go func(pr *Projection, index int) {
			defer wg.Done()
			result := pr.RunToEnd(ctx)
			if result.Error != nil {
				if !errors.Is(result.Error, context.Canceled) && cancelOnError {
					cancel()

					lock.Lock()
					causingErr = result.Error
					lock.Unlock()
				}
			}
			lock.Lock()
			results[index] = result
			lock.Unlock()
		}(projection, i)
	}
	wg.Wait()
	return results, causingErr
}
