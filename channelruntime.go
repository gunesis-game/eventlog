package eventlog

import (
	"reflect"
	"sync"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
)

// liveEvent is one adopted live Event of a Channel, keyed by logical identity.
type liveEvent struct {
	id       identity.UUID
	identity identity.UUID
	schema   *fieldschema.Schema
	instance reflect.Value
	event    Event
}

// channelRuntime is the single runtime of one logical Channel: its adopted live
// pointer and live Events, its notification and error streams, its shared
// transaction channel, and its one unified Reader progression.
type channelRuntime struct {
	rt            *logRuntime
	id            identity.UUID
	identity      identity.UUID
	schema        *fieldschema.Schema
	live          Channel
	instance      reflect.Value
	notifications chan Event
	errs          chan *ChannelError
	transactions  chan func() error
	processor     *transactionProcessor

	mu     sync.Mutex
	events []*liveEvent
	byID   map[identity.UUID]*liveEvent

	// streamMu guards closing the streams against in-flight error sends.
	streamMu      sync.RWMutex
	streamsClosed bool
}

func newChannelRuntime(rt *logRuntime, id, raw identity.UUID, schema *fieldschema.Schema, live Channel, notifications chan Event, errs chan *ChannelError) *channelRuntime {
	return &channelRuntime{
		rt:            rt,
		id:            id,
		identity:      raw,
		schema:        schema,
		live:          live,
		instance:      reflect.ValueOf(live),
		notifications: notifications,
		errs:          errs,
		transactions:  make(chan func() error),
		byID:          make(map[identity.UUID]*liveEvent),
	}
}

// adoptEvent returns the live Event for a logical identity, adopting the
// candidate when the identity is new.
func (c *channelRuntime) adoptEvent(candidate *liveEvent) *liveEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.byID[candidate.id]; ok {
		return existing
	}
	c.byID[candidate.id] = candidate
	c.events = append(c.events, candidate)
	return candidate
}

// liveEvents returns the adopted live Events in adoption order.
func (c *channelRuntime) liveEvents() []*liveEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*liveEvent(nil), c.events...)
}

func (c *channelRuntime) newError(kind ErrorKind, cause error, events []Event) *ChannelError {
	return &ChannelError{Kind: kind, Cause: cause, Events: events, channelID: c.id}
}

// deliverError blocks until the consumer receives err. It returns false
// without delivering when the streams are closed or the runtime has stopped.
func (c *channelRuntime) deliverError(err *ChannelError) (delivered bool) {
	c.streamMu.RLock()
	defer c.streamMu.RUnlock()
	if c.streamsClosed {
		return false
	}
	defer func() {
		if recover() != nil {
			delivered = false
		}
	}()
	select {
	case c.errs <- err:
		return true
	case <-c.rt.closed:
		return false
	}
}

// terminate ends this Channel's Reader after a fatal error: it delivers the
// error, then closes the error and notification streams. Persistence and other
// Channels continue.
func (c *channelRuntime) terminate(err *ChannelError) {
	c.deliverError(err)
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	if c.streamsClosed {
		return
	}
	c.streamsClosed = true
	closeQuietly(c.errs)
	closeQuietly(c.notifications)
}

func closeQuietly[T any](ch chan T) {
	defer func() {
		_ = recover()
	}()
	close(ch)
}
