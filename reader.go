package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/gunesis-game/eventlog/internal/causal"
	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
	"github.com/gunesis-game/eventlog/internal/storage"
)

// channelReader is a Channel's one Reader progression: it reconstructs every
// committed occurrence of the Channel, from this device's log and every other
// device's, and delivers each on the Channel's notification stream in causal
// order (D-057, D-058).
type channelReader struct {
	channel *channelRuntime
	self    identity.UUID
	// delivered is the Channel's delivered version vector: per device, how
	// many of its occurrences of the Channel have been delivered.
	delivered map[string]uint64
	// pending holds, per device, occurrences read but not yet delivered, in
	// that device's order, starting just after its delivered count.
	pending map[identity.UUID][]*occurrence
	// current is, per logical Event, the occurrence holding its current state.
	current map[identity.UUID]*occurrence
	// missing is, per device, where the missing history already reported on
	// this Channel starts (D-070).
	missing map[identity.UUID]uint64
}

// occurrence is one reconstructed committed occurrence with its causal
// metadata.
type occurrence struct {
	event    Event
	schema   *fieldschema.Schema
	instance reflect.Value
	id       identity.UUID
	device   identity.UUID
	vector   map[string]uint64
	at       causal.Timestamp
}

func newChannelReader(channel *channelRuntime) *channelReader {
	return &channelReader{
		channel:   channel,
		self:      channel.rt.store.Device(),
		delivered: make(map[string]uint64),
		pending:   make(map[identity.UUID][]*occurrence),
		current:   make(map[identity.UUID]*occurrence),
		missing:   make(map[identity.UUID]uint64),
	}
}

// run is the Channel's one continuous Reader progression. It waits for every
// decoder its existing history needs, then repeatedly delivers the next ready
// occurrence, waiting for a commit or a refresh of other devices' logs when
// none is ready and rechecking after each wake.
func (r *channelReader) run() {
	store := r.channel.rt.store
	for _, key := range store.ChannelHistoryTypes(r.channel.id) {
		if _, err := r.awaitDecoder(key); err != nil {
			return
		}
	}
	for {
		changed := store.Changed()
		next, err := r.next()
		if errors.Is(err, errRuntimeStopped) {
			return
		}
		if err != nil {
			r.channel.terminate(r.channel.newError(ReaderFailed, err, nil))
			return
		}
		if next == nil {
			if !r.reportMissing() {
				return
			}
			select {
			case <-changed:
			case <-r.channel.rt.closed:
				return
			}
			continue
		}
		if err := r.deliverNext(next); err != nil {
			if !errors.Is(err, errRuntimeStopped) {
				r.channel.terminate(r.channel.newError(DeliveryFailed, err, []Event{next.event}))
			}
			return
		}
	}
}

// next returns the ready occurrence to deliver next, or nil when none is
// ready. An occurrence is ready when every occurrence its version vector
// counts has been delivered; among ready ones the earliest clock reading goes
// first, with device identity breaking ties, so every device replays the same
// history in the same order.
func (r *channelReader) next() (*occurrence, error) {
	var best *occurrence
	for _, device := range append([]identity.UUID{r.self}, r.channel.rt.store.Devices()...) {
		head, err := r.head(device)
		if err != nil {
			return nil, err
		}
		if head != nil && r.ready(head) && (best == nil || head.before(best)) {
			best = head
		}
	}
	return best, nil
}

// head returns a device's next undelivered occurrence of the Channel, reading
// a batch when none is pending, or nil when there is none yet. Another
// device's log that cannot be read now has none yet; its missing history is
// reported by reportMissing.
func (r *channelReader) head(device identity.UUID) (*occurrence, error) {
	if pending := r.pending[device]; len(pending) > 0 {
		return pending[0], nil
	}
	store := r.channel.rt.store
	position := int(r.delivered[device.String()])
	var entries []storage.Occurrence
	var frames [][]byte
	var err error
	if device == r.self {
		entries, frames, err = store.ReadOccurrences(r.channel.id, position, r.channel.rt.policy.readBatch)
		if err != nil {
			return nil, err
		}
	} else if entries, frames, err = store.DeviceOccurrences(device, r.channel.id, position, r.channel.rt.policy.readBatch); err != nil {
		return nil, nil
	}
	if len(entries) == 0 {
		return nil, nil
	}
	pending := make([]*occurrence, 0, len(entries))
	for i, entry := range entries {
		o, err := r.reconstruct(frames[i], entry, device)
		if err != nil {
			return nil, err
		}
		pending = append(pending, o)
	}
	r.pending[device] = pending
	return pending[0], nil
}

// ready reports whether every occurrence o's version vector counts, other
// than o itself, has been delivered.
func (r *channelReader) ready(o *occurrence) bool {
	writer := o.device.String()
	for device, count := range o.vector {
		if device == writer {
			if count != r.delivered[device]+1 {
				return false
			}
		} else if count > r.delivered[device] {
			return false
		}
	}
	return true
}

// before reports whether o goes before p among ready occurrences, and which
// of two concurrent occurrences of one Event holds its current state: the
// earlier clock reading, then the lower device identity.
func (o *occurrence) before(p *occurrence) bool {
	if c := o.at.Compare(p.at); c != 0 {
		return c < 0
	}
	return slices.Compare(o.device[:], p.device[:]) < 0
}

// deliverNext advances the delivered vector past o, so transactions that
// follow its notification count it, delivers o, and then reports a conflict
// when o is concurrent with its Event's current state.
func (r *channelReader) deliverNext(o *occurrence) error {
	r.pending[o.device] = r.pending[o.device][1:]
	writer := o.device.String()
	r.delivered[writer]++
	if o.device != r.self {
		r.channel.rt.store.Delivered(r.channel.id, o.device, r.delivered[writer], o.at)
	}
	previous := r.current[o.id]
	concurrent := previous != nil && !descends(o.vector, previous.vector)
	// Of concurrent occurrences the greater clock reading holds the current
	// state, so every device converges on the same one whatever the delivery
	// order; an occurrence that knows of the current state replaces it.
	winner, other := o, previous
	if concurrent && o.before(previous) {
		winner, other = previous, o
	}
	r.current[o.id] = winner
	if err := r.deliver(o.event); err != nil {
		return err
	}
	if !concurrent {
		return nil
	}
	conflict := r.channel.newError(ConcurrentChange,
		fmt.Errorf("occurrences from devices %s and %s were written without either knowing of the other; the one from %s holds the current state", winner.device, other.device, winner.device),
		[]Event{winner.copy(), other.copy()})
	if !r.channel.deliverError(conflict) {
		return errRuntimeStopped
	}
	return nil
}

// descends reports whether vector v counts every occurrence u counts, so the
// occurrence of v was written knowing of the occurrence of u.
func descends(v, u map[string]uint64) bool {
	for device, count := range u {
		if v[device] < count {
			return false
		}
	}
	return true
}

// reportMissing reports, once per device and missing range, another device's
// missing history that this Channel may be affected by: the Channel has
// delivered occurrences of that device, or an occurrence waits on that
// device's occurrences past its readable history (D-070). It returns false
// when the runtime has stopped.
func (r *channelReader) reportMissing() bool {
	store := r.channel.rt.store
	for _, device := range store.Devices() {
		from, cause := store.DeviceHistory(device)
		if cause == nil {
			delete(r.missing, device)
			continue
		}
		if reported, ok := r.missing[device]; ok && reported == from {
			continue
		}
		if len(r.pending[device]) > 0 || !r.affectedBy(device) {
			continue
		}
		missing := r.channel.newError(HistoryMissing, fmt.Errorf("history of device %s is missing from sequence %d: %w", device, from, cause), nil)
		if !r.channel.deliverError(missing) {
			return false
		}
		r.missing[device] = from
	}
	return true
}

// affectedBy reports whether the Channel has delivered occurrences of device
// or holds an occurrence waiting on more of them.
func (r *channelReader) affectedBy(device identity.UUID) bool {
	key := device.String()
	if r.delivered[key] > 0 {
		return true
	}
	for _, pending := range r.pending {
		if len(pending) > 0 && pending[0].vector[key] > r.delivered[key] {
			return true
		}
	}
	return false
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

// reconstruct decodes a committed occurrence of device's log into a fresh
// concrete Event pointer, verifies its persisted identity, and keeps its
// causal metadata.
func (r *channelReader) reconstruct(frame []byte, entry storage.Occurrence, device identity.UUID) (*occurrence, error) {
	// The frame was schema-validated at recovery or before it was written;
	// field values and identity are re-checked below during reconstruction.
	var persisted record.Event
	if err := json.Unmarshal(frame, &persisted); err != nil {
		return nil, fmt.Errorf("decode occurrence %d of device %s: %w", entry.Sequence, device, err)
	}
	if persisted.Kind != record.KindEvent || persisted.Sequence != entry.Sequence || persisted.DeviceID != device.String() {
		return nil, fmt.Errorf("occurrence %d of device %s does not match its indexed Event record", entry.Sequence, device)
	}
	schema, err := r.awaitDecoder(entry.Type)
	if err != nil {
		return nil, err
	}
	if len(persisted.Fields) != len(schema.Fields) {
		return nil, fmt.Errorf("occurrence %d of %s has %d fields, registered schema has %d", entry.Sequence, entry.Type, len(persisted.Fields), len(schema.Fields))
	}
	instance := reflect.New(schema.Type)
	for _, binding := range schema.Fields {
		raw, ok := persisted.Fields[binding.Name]
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
	return &occurrence{event: event, schema: schema, instance: instance, id: id, device: device, vector: persisted.Vector, at: persisted.HLC}, nil
}

// copy returns an isolated copy of the occurrence's Event, apart from the one
// delivered as a notification.
func (o *occurrence) copy() Event {
	return fieldschema.CopySelected(o.schema, o.instance).Interface().(Event)
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
