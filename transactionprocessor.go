package eventlog

import (
	"errors"
	"fmt"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/storage"
)

var errChannelChangeWithoutEvent = errors.New("a transaction that changes the Channel must also change an Event")

// transactionProcessor applies one Channel's transactions in send order,
// rolls back rejected ones, and group-commits accepted ones. It is the only
// writer of the Channel's live instances while a transaction runs.
//
// The consumer-facing transaction channel is unbuffered. An intake goroutine
// receives every send into the bounded pending queue, so a send completes only
// when intake has taken it. That makes "sent before the error was received"
// exact: while a failure awaits receipt, intake discards each send it takes.
type transactionProcessor struct {
	channel  *channelRuntime
	pending  chan func() error
	failures chan *ChannelError
	resumed  chan struct{}
}

func newTransactionProcessor(channel *channelRuntime) *transactionProcessor {
	return &transactionProcessor{
		channel:  channel,
		pending:  make(chan func() error, channel.rt.policy.transactionBuffer),
		failures: make(chan *ChannelError),
		resumed:  make(chan struct{}),
	}
}

// runIntake moves sends into the pending queue. On a failure it discards the
// queue and every further send until the consumer receives the error.
func (p *transactionProcessor) runIntake() {
	c := p.channel
	var held func() error
	holding := false
	for {
		var pending chan func() error
		var incoming chan func() error
		if holding {
			pending = p.pending
		} else {
			incoming = c.transactions
		}
		select {
		case fn := <-incoming:
			held, holding = fn, true
		case pending <- held:
			held, holding = nil, false
		case failure := <-p.failures:
			held, holding = nil, false
			p.discardPending()
			p.pauseUntilReceived(failure)
			p.resumed <- struct{}{}
		case <-c.rt.closed:
			return
		}
	}
}

func (p *transactionProcessor) discardPending() {
	for {
		select {
		case <-p.pending:
		default:
			return
		}
	}
}

// pauseUntilReceived delivers failure, discarding every send taken before the
// consumer receives it. Once the streams are closed nothing can be received,
// so it returns at once.
func (p *transactionProcessor) pauseUntilReceived(failure *ChannelError) {
	c := p.channel
	c.streamMu.RLock()
	defer c.streamMu.RUnlock()
	if c.streamsClosed {
		return
	}
	defer func() {
		_ = recover()
	}()
	for {
		select {
		case <-c.transactions:
		case c.errs <- failure:
			return
		case <-c.rt.closed:
			return
		}
	}
}

// liveState is a captured snapshot of a Channel and a fixed list of its live
// Events, with their raw identity values.
type liveState struct {
	channel    fieldschema.Snapshot
	channelRaw identity.UUID
	events     []*liveEvent
	snapshots  []fieldschema.Snapshot
	raws       []identity.UUID
}

type pendingOccurrence struct {
	event    *liveEvent
	snapshot fieldschema.Snapshot
	channel  fieldschema.Snapshot
}

// transactionBatch collects accepted, uncommitted occurrences. original keeps
// each touched instance's state before the batch, for rollback on a failed
// commit.
type transactionBatch struct {
	occurrences     []pendingOccurrence
	channelOriginal fieldschema.Snapshot
	originals       map[*liveEvent]fieldschema.Snapshot
	order           []*liveEvent
}

func (p *transactionProcessor) run() {
	c := p.channel
	for {
		select {
		case first := <-p.pending:
			p.processBatch(first)
		case <-c.rt.closed:
			return
		}
	}
}

// processBatch applies transactions until the queue is empty or the batch
// limit is reached, then commits. On a rejection, the accepted prefix is
// committed and the failure is reported.
func (p *transactionProcessor) processBatch(first func() error) {
	c := p.channel
	batch := &transactionBatch{originals: make(map[*liveEvent]fieldschema.Snapshot)}
	next := first
	for applied := 1; ; applied++ {
		if failure := p.apply(batch, next); failure != nil {
			if commitFailure := p.commit(batch); commitFailure != nil {
				commitFailure.Cause = errors.Join(commitFailure.Cause, failure)
				failure = commitFailure
			}
			p.reject(failure)
			return
		}
		if applied >= c.rt.policy.batchLimit {
			break
		}
		select {
		case next = <-p.pending:
			continue
		default:
		}
		break
	}
	if failure := p.commit(batch); failure != nil {
		p.reject(failure)
	}
}

// apply runs one transaction and validates the result. An accepted change adds
// one occurrence per changed Event; a rejected one is rolled back.
func (p *transactionProcessor) apply(batch *transactionBatch, fn func() error) *ChannelError {
	c := p.channel
	before, err := p.capture(c.liveEvents())
	if err != nil {
		return c.newError(InvalidState, fmt.Errorf("live state is invalid before the transaction: %w", err), nil)
	}
	if kind, cause := runTransaction(fn); cause != nil {
		return p.rollback(before, kind, cause)
	}
	after, err := p.capture(before.events)
	if err != nil {
		return p.rollback(before, InvalidState, err)
	}
	if after.channelRaw != c.identity {
		return p.rollback(before, InvalidState, fmt.Errorf("channel identity field %q was changed", c.schema.IdentityField().Name))
	}
	var changed []int
	for i, event := range before.events {
		if after.raws[i] != event.identity {
			return p.rollback(before, InvalidState, fmt.Errorf("%s identity field %q was changed", event.schema.Key(), event.schema.IdentityField().Name))
		}
		if !after.snapshots[i].Equal(before.snapshots[i]) {
			changed = append(changed, i)
		}
	}
	channelChanged := !after.channel.Equal(before.channel)
	if channelChanged && len(changed) == 0 {
		return p.rollback(before, InvalidState, errChannelChangeWithoutEvent)
	}
	if len(changed) > 0 {
		// Storage that refuses this device's writes still runs transactions,
		// so live fields can be read safely; only a change is refused.
		if refusal := c.rt.store.WriteRefusal(); refusal != nil {
			return p.rollback(before, WriteNotPermitted, refusal)
		}
	}
	if len(changed) > 0 && batch.channelOriginal == nil {
		batch.channelOriginal = before.channel
	}
	for _, i := range changed {
		event := before.events[i]
		if _, seen := batch.originals[event]; !seen {
			batch.originals[event] = before.snapshots[i]
			batch.order = append(batch.order, event)
		}
		batch.occurrences = append(batch.occurrences, pendingOccurrence{event: event, snapshot: after.snapshots[i], channel: after.channel})
	}
	return nil
}

// runTransaction runs fn, converting a returned error or panic into a cause.
func runTransaction(fn func() error) (kind ErrorKind, cause error) {
	if fn == nil {
		return TransactionRefused, errors.New("transaction function is nil")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			kind, cause = TransactionPanicked, fmt.Errorf("panic: %v", recovered)
		}
	}()
	if err := fn(); err != nil {
		return TransactionRefused, err
	}
	return 0, nil
}

func (p *transactionProcessor) capture(events []*liveEvent) (liveState, error) {
	c := p.channel
	state := liveState{events: events, snapshots: make([]fieldschema.Snapshot, len(events)), raws: make([]identity.UUID, len(events))}
	var err error
	if state.channel, state.channelRaw, err = fieldschema.Capture(c.schema, c.instance); err != nil {
		return state, fmt.Errorf("channel: %w", err)
	}
	for i, event := range events {
		if state.snapshots[i], state.raws[i], err = fieldschema.Capture(event.schema, event.instance); err != nil {
			return state, fmt.Errorf("%s: %w", event.schema.Key(), err)
		}
	}
	return state, nil
}

// rollback restores the state before a rejected transaction and returns its
// error, carrying copies of the attempted values of each Event it changed.
func (p *transactionProcessor) rollback(before liveState, kind ErrorKind, cause error) *ChannelError {
	c := p.channel
	var attempted []Event
	for i, event := range before.events {
		current, _, err := fieldschema.Capture(event.schema, event.instance)
		if err != nil || !current.Equal(before.snapshots[i]) {
			attempted = append(attempted, copyEvent(event))
		}
	}
	p.restore(before.channel, before.events, before.snapshots)
	return c.newError(kind, cause, attempted)
}

func (p *transactionProcessor) restore(channel fieldschema.Snapshot, events []*liveEvent, snapshots []fieldschema.Snapshot) {
	c := p.channel
	_ = fieldschema.Restore(c.schema, c.instance, channel)
	for i, event := range events {
		_ = fieldschema.Restore(event.schema, event.instance, snapshots[i])
	}
}

func copyEvent(event *liveEvent) Event {
	return fieldschema.CopySelected(event.schema, event.instance).Interface().(Event)
}

// commit persists the batch atomically. On failure every transaction in the
// batch is rolled back.
func (p *transactionProcessor) commit(batch *transactionBatch) *ChannelError {
	c := p.channel
	if len(batch.occurrences) == 0 {
		return nil
	}
	writes := make([]storage.OccurrenceWrite, len(batch.occurrences))
	for i, occurrence := range batch.occurrences {
		writes[i] = storage.OccurrenceWrite{
			ChannelFields: occurrence.channel.RecordFields(c.schema),
			EventID:       occurrence.event.id,
			EventType:     occurrence.event.schema.Key(),
			EventFields:   occurrence.snapshot.RecordFields(occurrence.event.schema),
		}
	}
	err := c.rt.store.AppendTransaction(storage.ChannelWrite{ID: c.id, Type: c.schema.Key()}, writes)
	if err == nil {
		return nil
	}
	attempted := make([]Event, 0, len(batch.order))
	originals := make([]fieldschema.Snapshot, 0, len(batch.order))
	for _, event := range batch.order {
		attempted = append(attempted, copyEvent(event))
		originals = append(originals, batch.originals[event])
	}
	p.restore(batch.channelOriginal, batch.order, originals)
	return c.newError(CommitFailed, err, attempted)
}

// reject hands a failure to intake and waits until the consumer has received
// it. Every transaction sent before then is rejected without being run.
func (p *transactionProcessor) reject(failure *ChannelError) {
	c := p.channel
	select {
	case p.failures <- failure:
	case <-c.rt.closed:
		return
	}
	select {
	case <-p.resumed:
	case <-c.rt.closed:
	}
}
