package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
	"github.com/gunesis-game/eventlog/internal/storage"
)

// channelReader is a Channel's one unified Reader progression: it reconstructs
// every committed occurrence of the Channel, in global sequence order, and
// delivers it on the Channel's notification stream.
type channelReader struct {
	channel *channelRuntime
}

// run is the Channel's one continuous committed-sequence progression.
// It waits for every decoder its existing history needs, then delivers each
// committed occurrence in global sequence order, waiting on commit wake-up
// hints when caught up and rechecking the store after each wake.
func (r *channelReader) run() {
	for _, key := range r.channel.rt.store.ChannelHistoryTypes(r.channel.id) {
		if _, err := r.awaitDecoder(key); err != nil {
			return
		}
	}
	for position := 0; ; {
		if err := r.awaitOccurrence(position); err != nil {
			return
		}
		entries, frames, err := r.channel.rt.store.ReadOccurrences(r.channel.id, position, r.channel.rt.policy.readBatch)
		if err != nil {
			r.channel.terminate(r.channel.newError(ReaderFailed, err, nil))
			return
		}
		for i, entry := range entries {
			event, err := r.reconstruct(frames[i], entry)
			if errors.Is(err, errRuntimeStopped) {
				return
			}
			if err != nil {
				r.channel.terminate(r.channel.newError(ReaderFailed, err, nil))
				return
			}
			if err := r.deliver(event); err != nil {
				if !errors.Is(err, errRuntimeStopped) {
					r.channel.terminate(r.channel.newError(DeliveryFailed, err, []Event{event}))
				}
				return
			}
			position++
		}
	}
}

// awaitOccurrence waits until the position-th occurrence is committed.
func (r *channelReader) awaitOccurrence(position int) error {
	for {
		_, ok, wake := r.channel.rt.store.ChannelOccurrence(r.channel.id, position)
		if ok {
			return nil
		}
		select {
		case <-wake:
		case <-r.channel.rt.closed:
			return errRuntimeStopped
		}
	}
}

// awaitDecoder waits indefinitely, without error, for a decoder to register.
func (r *channelReader) awaitDecoder(key fieldschema.TypeKey) (*fieldschema.Schema, error) {
	for {
		schema, changed := r.channel.rt.registry.lookup(key)
		if schema != nil {
			return schema, nil
		}
		select {
		case <-changed:
		case <-r.channel.rt.closed:
			return nil, errRuntimeStopped
		}
	}
}

// reconstruct decodes a committed occurrence into a fresh concrete Event
// pointer and verifies its persisted identity.
func (r *channelReader) reconstruct(frame []byte, entry storage.Occurrence) (Event, error) {
	// The frame was schema-validated at recovery or before it was written;
	// field values and identity are re-checked below during reconstruction.
	var occurrence record.Event
	if err := json.Unmarshal(frame, &occurrence); err != nil {
		return nil, fmt.Errorf("decode occurrence %d: %w", entry.Sequence, err)
	}
	if occurrence.Kind != record.KindEvent || occurrence.Sequence != entry.Sequence {
		return nil, fmt.Errorf("occurrence %d does not match its indexed Event record", entry.Sequence)
	}
	schema, err := r.awaitDecoder(entry.Type)
	if err != nil {
		return nil, err
	}
	if len(occurrence.Fields) != len(schema.Fields) {
		return nil, fmt.Errorf("occurrence %d of %s has %d fields, registered schema has %d", entry.Sequence, entry.Type, len(occurrence.Fields), len(schema.Fields))
	}
	instance := reflect.New(schema.Type)
	for _, binding := range schema.Fields {
		raw, ok := occurrence.Fields[binding.Name]
		if !ok {
			return nil, fmt.Errorf("occurrence %d of %s lacks field %q", entry.Sequence, entry.Type, binding.Name)
		}
		value, err := fieldschema.ValueFromRecord(raw)
		if err != nil {
			return nil, err
		}
		if err := fieldschema.Decode(binding, schema.FieldValue(instance, binding), value); err != nil {
			return nil, fmt.Errorf("occurrence %d of %s: %w", entry.Sequence, entry.Type, err)
		}
	}
	_, raw, err := fieldschema.Capture(schema, instance)
	if err != nil {
		return nil, err
	}
	id, err := identity.Derive(identity.EventDomain, schema.Descriptor(), raw)
	if err != nil {
		return nil, err
	}
	if id != entry.EventID {
		return nil, fmt.Errorf("occurrence %d identity %s does not match its fields (%s)", entry.Sequence, entry.EventID, id)
	}
	event, ok := instance.Interface().(Event)
	if !ok {
		return nil, fmt.Errorf("%s does not implement Event", schema.Type)
	}
	return event, nil
}

// deliver sends one occurrence, retrying the same occurrence with bounded
// backoff. Later occurrences cannot overtake it; exhaustion is fatal.
func (r *channelReader) deliver(event Event) error {
	backoff := r.channel.rt.policy.deliveryBackoff
	var lastErr error
	for attempt := 1; attempt <= r.channel.rt.policy.deliveryAttempts; attempt++ {
		lastErr = r.sendNotification(event)
		if lastErr == nil || errors.Is(lastErr, errRuntimeStopped) {
			return lastErr
		}
		if attempt == r.channel.rt.policy.deliveryAttempts {
			break
		}
		select {
		case <-time.After(backoff):
		case <-r.channel.rt.closed:
			return errRuntimeStopped
		}
		backoff *= 2
	}
	return fmt.Errorf("notification delivery failed after %d attempts: %w", r.channel.rt.policy.deliveryAttempts, lastErr)
}

// sendNotification blocks while the bounded notification buffer is full; that
// backpressure holds only this Reader, never persistence. A send on a stream
// the consumer closed is a delivery failure.
func (r *channelReader) sendNotification(event Event) (err error) {
	if fault := r.channel.rt.policy.deliveryFault; fault != nil {
		if err := fault(r.channel.id, event); err != nil {
			return err
		}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("send notification: %v", recovered)
		}
	}()
	select {
	case r.channel.notifications <- event:
		return nil
	case <-r.channel.rt.closed:
		return errRuntimeStopped
	}
}
