package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// liveEvent is one adopted live Event of a Channel, keyed by logical identity.
type liveEvent struct {
	id       uuid
	identity uuid
	schema   *typeSchema
	instance reflect.Value
	event    Event
}

// channelRuntime is the single runtime of one logical Channel: its adopted live
// pointer and live Events, its notification and error streams, its shared
// transaction channel, and its one unified Reader progression.
type channelRuntime struct {
	rt            *logRuntime
	id            uuid
	identity      uuid
	schema        *typeSchema
	live          Channel
	instance      reflect.Value
	notifications chan Event
	errs          chan *ChannelError
	transactions  chan func() error

	mu     sync.Mutex
	events []*liveEvent
	byID   map[uuid]*liveEvent

	// streamMu guards closing the streams against in-flight error sends.
	streamMu      sync.RWMutex
	streamsClosed bool
}

func newChannelRuntime(rt *logRuntime, id, identity uuid, schema *typeSchema, live Channel, notifications chan Event, errs chan *ChannelError) *channelRuntime {
	return &channelRuntime{
		rt:            rt,
		id:            id,
		identity:      identity,
		schema:        schema,
		live:          live,
		instance:      reflect.ValueOf(live),
		notifications: notifications,
		errs:          errs,
		transactions:  make(chan func() error),
		byID:          make(map[uuid]*liveEvent),
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

// runReader is the Channel's one continuous committed-sequence progression.
// It waits for every decoder its existing history needs, then delivers each
// committed occurrence in global sequence order, waiting on commit wake-up
// hints when caught up and rechecking the store after each wake.
func (c *channelRuntime) runReader() {
	for _, key := range c.rt.store.channelHistoryTypes(c.id) {
		if _, err := c.awaitDecoder(key); err != nil {
			return
		}
	}
	for position := 0; ; position++ {
		entry, err := c.awaitOccurrence(position)
		if err != nil {
			return
		}
		event, err := c.reconstruct(position, entry)
		if errors.Is(err, errRuntimeStopped) {
			return
		}
		if err != nil {
			c.terminate(c.newError(ReaderFailed, err, nil))
			return
		}
		if err := c.deliver(event); err != nil {
			if !errors.Is(err, errRuntimeStopped) {
				c.terminate(c.newError(DeliveryFailed, err, []Event{event}))
			}
			return
		}
	}
}

func (c *channelRuntime) awaitOccurrence(position int) (indexEntry, error) {
	for {
		entry, ok, wake := c.rt.store.channelOccurrence(c.id, position)
		if ok {
			return entry, nil
		}
		select {
		case <-wake:
		case <-c.rt.closed:
			return indexEntry{}, errRuntimeStopped
		}
	}
}

// awaitDecoder waits indefinitely, without error, for a decoder to register.
func (c *channelRuntime) awaitDecoder(key typeKey) (*typeSchema, error) {
	for {
		schema, changed := c.rt.registry.lookup(key)
		if schema != nil {
			return schema, nil
		}
		select {
		case <-changed:
		case <-c.rt.closed:
			return nil, errRuntimeStopped
		}
	}
}

// reconstruct decodes a committed occurrence into a fresh concrete Event
// pointer and verifies its persisted identity.
func (c *channelRuntime) reconstruct(position int, entry indexEntry) (Event, error) {
	frame, err := c.rt.store.readOccurrenceFrame(c.id, position)
	if err != nil {
		return nil, err
	}
	// The frame was schema-validated at recovery or before it was written;
	// field values and identity are re-checked below during reconstruction.
	var record eventRecord
	if err := json.Unmarshal(frame, &record); err != nil {
		return nil, fmt.Errorf("decode occurrence %d: %w", entry.sequence, err)
	}
	if record.Kind != recordKindEvent || record.Sequence != entry.sequence {
		return nil, fmt.Errorf("occurrence %d does not match its indexed Event record", entry.sequence)
	}
	schema, err := c.awaitDecoder(entry.typ)
	if err != nil {
		return nil, err
	}
	if len(record.Fields) != len(schema.fields) {
		return nil, fmt.Errorf("occurrence %d of %s has %d fields, registered schema has %d", entry.sequence, entry.typ, len(record.Fields), len(schema.fields))
	}
	instance := reflect.New(schema.typ)
	for _, binding := range schema.fields {
		raw, ok := record.Fields[binding.name]
		if !ok {
			return nil, fmt.Errorf("occurrence %d of %s lacks field %q", entry.sequence, entry.typ, binding.name)
		}
		value, err := fieldValueFromRecord(raw)
		if err != nil {
			return nil, err
		}
		if err := decodeFieldInto(binding, schema.fieldValueOf(instance, binding), value); err != nil {
			return nil, fmt.Errorf("occurrence %d of %s: %w", entry.sequence, entry.typ, err)
		}
	}
	_, raw, err := captureSnapshot(schema, instance)
	if err != nil {
		return nil, err
	}
	id, err := deriveIdentity(eventIdentityDomain, schema, raw)
	if err != nil {
		return nil, err
	}
	if id != entry.eventID {
		return nil, fmt.Errorf("occurrence %d identity %s does not match its fields (%s)", entry.sequence, entry.eventID, id)
	}
	event, ok := instance.Interface().(Event)
	if !ok {
		return nil, fmt.Errorf("%s does not implement Event", schema.typ)
	}
	return event, nil
}

// deliver sends one occurrence, retrying the same occurrence with bounded
// backoff. Later occurrences cannot overtake it; exhaustion is fatal.
func (c *channelRuntime) deliver(event Event) error {
	backoff := c.rt.policy.deliveryBackoff
	var lastErr error
	for attempt := 1; attempt <= c.rt.policy.deliveryAttempts; attempt++ {
		lastErr = c.sendNotification(event)
		if lastErr == nil || errors.Is(lastErr, errRuntimeStopped) {
			return lastErr
		}
		if attempt == c.rt.policy.deliveryAttempts {
			break
		}
		select {
		case <-time.After(backoff):
		case <-c.rt.closed:
			return errRuntimeStopped
		}
		backoff *= 2
	}
	return fmt.Errorf("notification delivery failed after %d attempts: %w", c.rt.policy.deliveryAttempts, lastErr)
}

// sendNotification blocks while the bounded notification buffer is full; that
// backpressure holds only this Reader, never persistence. A send on a stream
// the consumer closed is a delivery failure.
func (c *channelRuntime) sendNotification(event Event) (err error) {
	if fault := c.rt.policy.deliveryFault; fault != nil {
		if err := fault(c.id, event); err != nil {
			return err
		}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("send notification: %v", recovered)
		}
	}()
	select {
	case c.notifications <- event:
		return nil
	case <-c.rt.closed:
		return errRuntimeStopped
	}
}
