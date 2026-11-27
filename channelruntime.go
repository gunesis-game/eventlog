package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// channelRuntime is the single runtime of one logical Channel: its adopted live
// pointer, notification and error streams, live Event instances, and its one
// unified Reader progression.
type channelRuntime struct {
	rt            *logRuntime
	id            uuid
	identity      uuid
	schema        *typeSchema
	live          Channel
	instance      reflect.Value
	notifications chan Event
	errs          chan error

	mu         sync.Mutex
	events     map[typeKey]*eventTracker
	latest     fieldSnapshot
	terminated bool
	fatal      error

	// observed and observeErr are owned by the change manager's poll pass.
	observed   fieldSnapshot
	observeErr string
}

func (c *channelRuntime) latestSnapshot() fieldSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest
}

func (c *channelRuntime) setLatestSnapshot(snapshot fieldSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latest = snapshot
}

// reportError publishes a non-fatal error best-effort without blocking. The
// error stream has capacity one, so an error is dropped if one is pending.
func (c *channelRuntime) reportError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.terminated {
		return
	}
	c.offerErrorLocked(err)
}

func (c *channelRuntime) offerErrorLocked(err error) {
	defer func() {
		_ = recover()
	}()
	select {
	case c.errs <- err:
	default:
	}
}

// terminate ends this Channel's Reader after a fatal error: it retains the
// first fatal error, reports it best-effort, then closes the error and
// notification streams. Other Channels and persistence continue.
func (c *channelRuntime) terminate(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.terminated {
		return
	}
	c.terminated = true
	c.fatal = err
	c.offerErrorLocked(err)
	closeQuietly(c.errs)
	closeQuietly(c.notifications)
}

func closeQuietly[T any](ch chan T) {
	defer func() {
		_ = recover()
	}()
	close(ch)
}

func (c *channelRuntime) fatalError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fatal
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
			c.terminate(fmt.Errorf("eventlog: channel %s reader: %w", c.id, err))
			return
		}
		if err := c.deliver(event); err != nil {
			if !errors.Is(err, errRuntimeStopped) {
				c.terminate(err)
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
	return fmt.Errorf("eventlog: channel %s notification delivery failed after %d attempts: %w", c.id, c.rt.policy.deliveryAttempts, lastErr)
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
