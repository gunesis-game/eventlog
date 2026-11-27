package eventlog

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

// eventTracker observes one live prepared Event instance.
type eventTracker struct {
	channel    *channelRuntime
	schema     *typeSchema
	instance   reflect.Value
	last       fieldSnapshot
	observeErr string
	bound      bool
	boundRaw   uuid
	eventID    uuid
}

// changeItem is one immutable captured snapshot queued for persistence: a
// Channel snapshot when event is nil, otherwise an Event occurrence.
type changeItem struct {
	channel  *channelRuntime
	snapshot fieldSnapshot
	event    *eventTracker
	eventID  uuid
}

// changeManager is the single managed scheduler that polls every selected
// Event and Channel field, plus the persistence worker draining its bounded
// FIFO ingress queue. One worker preserves FIFO order per logical identity.
type changeManager struct {
	rt       *logRuntime
	mu       sync.Mutex
	channels []*channelRuntime
	events   []*eventTracker
	queue    chan changeItem
}

func newChangeManager(rt *logRuntime) *changeManager {
	return &changeManager{rt: rt, queue: make(chan changeItem, rt.policy.ingressCapacity)}
}

func (cm *changeManager) trackChannel(channel *channelRuntime) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.channels = append(cm.channels, channel)
}

func (cm *changeManager) trackEvent(tracker *eventTracker) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.events = append(cm.events, tracker)
}

// withPollingPaused runs fn while no poll pass is in progress. It is a
// test-only synchronization point for mutating consumer-owned fields.
func (cm *changeManager) withPollingPaused(fn func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	fn()
}

func (cm *changeManager) runPoller() {
	ticker := time.NewTicker(cm.rt.policy.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cm.poll()
		case <-cm.rt.closed:
			return
		}
	}
}

// poll observes every Channel then every Event. Channels go first so an Event
// occurrence uses a Channel snapshot captured in the same pass.
func (cm *changeManager) poll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, channel := range cm.channels {
		cm.observeChannel(channel)
	}
	for _, tracker := range cm.events {
		cm.observeEvent(tracker)
	}
}

func (cm *changeManager) observeChannel(channel *channelRuntime) {
	snapshot, raw, err := captureSnapshot(channel.schema, channel.instance)
	if err != nil {
		cm.reportObserveError(channel, &channel.observeErr, fmt.Errorf("eventlog: channel %s: %w", channel.id, err))
		return
	}
	channel.observeErr = ""
	if snapshot.equal(channel.observed) {
		return
	}
	channel.observed = snapshot
	if raw != channel.identity {
		channel.reportError(fmt.Errorf("eventlog: channel %s identity field %q was mutated; the change is rejected", channel.id, channel.schema.identityField().name))
		return
	}
	cm.enqueue(changeItem{channel: channel, snapshot: snapshot})
}

func (cm *changeManager) observeEvent(tracker *eventTracker) {
	channel := tracker.channel
	snapshot, raw, err := captureSnapshot(tracker.schema, tracker.instance)
	if err != nil {
		cm.reportObserveError(channel, &tracker.observeErr, fmt.Errorf("eventlog: event %s: %w", tracker.schema.key(), err))
		return
	}
	tracker.observeErr = ""
	if snapshot.equal(tracker.last) {
		return
	}
	tracker.last = snapshot
	if raw.isZero() {
		channel.reportError(fmt.Errorf("eventlog: event %s changed while its identity field %q is the zero UUID; nothing was persisted", tracker.schema.key(), tracker.schema.identityField().name))
		return
	}
	if !tracker.bound {
		id, err := deriveIdentity(eventIdentityDomain, tracker.schema, raw)
		if err != nil {
			channel.reportError(fmt.Errorf("eventlog: event %s: %w", tracker.schema.key(), err))
			return
		}
		tracker.bound, tracker.boundRaw, tracker.eventID = true, raw, id
	} else if raw != tracker.boundRaw {
		channel.reportError(fmt.Errorf("eventlog: event %s identity field %q was mutated; the change is rejected", tracker.eventID, tracker.schema.identityField().name))
		return
	}
	cm.enqueue(changeItem{channel: channel, snapshot: snapshot, event: tracker, eventID: tracker.eventID})
}

// reportObserveError reports a capture error once until it changes or clears,
// rather than on every poll.
func (cm *changeManager) reportObserveError(channel *channelRuntime, last *string, err error) {
	if *last == err.Error() {
		return
	}
	*last = err.Error()
	channel.reportError(err)
}

// enqueue blocks while the bounded queue is full, which delays monitoring but
// never blocks consumer field assignments.
func (cm *changeManager) enqueue(item changeItem) {
	select {
	case cm.queue <- item:
	case <-cm.rt.closed:
	}
}

func (cm *changeManager) runPersistence() {
	for {
		select {
		case item := <-cm.queue:
			cm.persist(item)
		case <-cm.rt.closed:
			return
		}
	}
}

// persist applies one queued snapshot. A Channel snapshot only updates the
// latest Channel state; an Event occurrence is appended atomically together
// with that Channel state when it differs from the persisted Channel record.
func (cm *changeManager) persist(item changeItem) {
	channel := item.channel
	if item.event == nil {
		channel.setLatestSnapshot(item.snapshot)
		return
	}
	channelState := channel.latestSnapshot()
	write := channelWrite{id: channel.id, typ: channel.schema.key(), fields: channelState.toRecordFields(channel.schema)}
	occurrence := eventWrite{id: item.eventID, typ: item.event.schema.key(), fields: item.snapshot.toRecordFields(item.event.schema)}
	backoff := cm.rt.policy.persistBackoff
	var err error
	for attempt := 1; attempt <= cm.rt.policy.persistAttempts; attempt++ {
		if _, err = cm.rt.store.appendOccurrence(write, occurrence); err == nil {
			return
		}
		select {
		case <-time.After(backoff):
		case <-cm.rt.closed:
			return
		}
		backoff *= 2
	}
	channel.reportError(fmt.Errorf("eventlog: persist occurrence of event %s failed: %w", item.eventID, err))
}
