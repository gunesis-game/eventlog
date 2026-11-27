package eventlog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// API-03/04/06, N-01, N-02, L-06, R-02: each transaction that changes an Event
// becomes an ordered occurrence, delivered as a fresh isolated snapshot.
func TestTransactionsBecomeOrderedNotifications(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	setAmount(event, 10)
	first := expectAmount(t, channel, 10)
	setAmount(event, 20)
	expectAmount(t, channel, 20)
	if first == event || first.Amount != 10 || first.ID != OrderID(testUUID(2)) {
		t.Fatalf("snapshot is not isolated: %+v", first)
	}
	if first.Transaction() != nil {
		t.Fatal("snapshots must have a nil transaction channel")
	}
	event.Transaction() <- func() error {
		event.Note = stringPointer("gift")
		event.Status = "paid"
		event.ignore = "local"
		return nil
	}
	got := receiveEvent(t, channel).(*OrderPlaced)
	if got.Note == nil || *got.Note != "gift" || got.Status != "paid" || got.Amount != 20 || got.ignore != "" {
		t.Fatalf("snapshot = %+v", got)
	}
}

// D-050: every changing transaction is its own occurrence, so nothing between
// sends is lost (A-B-A stays three occurrences); a no-op persists nothing.
func TestEveryTransactionIsAnOccurrence(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	for _, amount := range []int64{1, 2, 1} {
		setAmount(event, amount)
	}
	event.Transaction() <- func() error { return nil }
	for _, amount := range []int64{1, 2, 1} {
		expectAmount(t, channel, amount)
	}
	expectNoNotification(t, channel, 30*time.Millisecond)
	if committedCount(rt) != 3 {
		t.Fatalf("committed = %d, want 3", committedCount(rt))
	}
}

// D-050: queued transactions are group-committed in one persisted transaction.
func TestQueuedTransactionsAreGroupCommitted(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	release := gate(event)
	for i := int64(1); i <= 5; i++ {
		setAmount(event, i)
	}
	waitPending(t, rt, channel, 5)
	release()
	for i := int64(1); i <= 5; i++ {
		expectAmount(t, channel, i)
	}
	if commits := countCommits(t, rt); commits != 1 {
		t.Fatalf("commits = %d, want 1", commits)
	}
}

// D-050: the batch limit bounds a group commit.
func TestBatchLimitSplitsCommits(t *testing.T) {
	policy := fastPolicy()
	policy.batchLimit = 2
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	release := gate(event)
	for i := int64(1); i <= 5; i++ {
		setAmount(event, i)
	}
	waitPending(t, rt, channel, 5)
	release()
	for i := int64(1); i <= 5; i++ {
		expectAmount(t, channel, i)
	}
	// The gate and the first send form one batch, then two pairs.
	if commits := countCommits(t, rt); commits != 3 {
		t.Fatalf("commits = %d, want 3", commits)
	}
}

func countCommits(t *testing.T, rt *logRuntime) int {
	t.Helper()
	rt.store.mu.Lock()
	path := rt.store.active.path
	rt.store.mu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), `"kind":"commit"`)
}

// The car scenario (D-051): when a transaction fails, the accepted prefix is
// kept, the failed one is rolled back, and everything queued behind it is
// rejected, so an impossible sequence is never recorded.
func TestFailureKeepsPrefixAndRejectsQueued(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	car, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	errDoorJammed := errors.New("door jammed")
	release := gate(car)
	car.Transaction() <- func() error { car.Status = "door opened"; return nil }
	car.Transaction() <- func() error { car.Status = "driver in"; return nil }
	car.Transaction() <- func() error {
		car.Status = "door closed"
		return fmt.Errorf("closing door: %w", errDoorJammed)
	}
	car.Transaction() <- func() error { car.Status = "engine started"; return nil }
	car.Transaction() <- func() error { car.Status = "moving"; return nil }
	release()

	failure := receiveError(t, channel)
	if failure.Kind != TransactionRefused || !errors.Is(failure, errDoorJammed) {
		t.Fatalf("failure = %v", failure)
	}
	if len(failure.Events) != 1 || failure.Events[0].(*OrderPlaced).Status != "door closed" {
		t.Fatalf("attempted Events = %#v", failure.Events)
	}
	for _, status := range []OrderStatus{"door opened", "driver in"} {
		if got := receiveEvent(t, channel).(*OrderPlaced).Status; got != status {
			t.Fatalf("status = %q, want %q", got, status)
		}
	}
	expectNoNotification(t, channel, 30*time.Millisecond)
	read(car, func() {
		if car.Status != "driver in" {
			t.Errorf("live status = %q, want the rolled-back %q", car.Status, "driver in")
		}
	})
	car.Transaction() <- func() error { car.Status = "door closed"; return nil }
	if got := receiveEvent(t, channel).(*OrderPlaced).Status; got != "door closed" {
		t.Fatalf("resent transaction applied %q", got)
	}
	if committedCount(rt) != 3 {
		t.Fatalf("committed = %d, want 3", committedCount(rt))
	}
}

// A first failing transaction rejects the whole queued sequence.
func TestFailureOfFirstTransactionRejectsSequence(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	car, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	release := gate(car)
	car.Transaction() <- func() error { return errors.New("door locked") }
	car.Transaction() <- func() error { car.Status = "driver in"; return nil }
	release()
	if failure := receiveError(t, channel); failure.Kind != TransactionRefused || len(failure.Events) != 0 {
		t.Fatalf("failure = %v", failure)
	}
	expectNoNotification(t, channel, 30*time.Millisecond)
	if committedCount(rt) != 0 {
		t.Fatal("nothing may be committed")
	}
}

// D-051: panics and invalid state are rejected and rolled back.
func TestInvalidTransactionsRollBack(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	cases := []struct {
		name   string
		kind   ErrorKind
		update func() error
	}{
		{"panic", TransactionPanicked, func() error { event.Amount = 9; panic("boom") }},
		{"Event identity changed", InvalidState, func() error { event.ID = testUUID(9); return nil }},
		{"Channel identity changed", InvalidState, func() error { channel.ID = testUUID(9); event.Amount = 9; return nil }},
		{"invalid UTF-8", InvalidState, func() error { event.Status = "\xff"; return nil }},
		{"Channel change without Event change", InvalidState, func() error { channel.Region = "us"; return nil }},
		{"nil function", TransactionRefused, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			event.Transaction() <- c.update
			failure := receiveError(t, channel)
			if failure.Kind != c.kind {
				t.Fatalf("kind = %v, want %v (%v)", failure.Kind, c.kind, failure)
			}
			read(event, func() {
				if event.Amount != 0 || event.ID != OrderID(testUUID(2)) || event.Status != "" || channel.Region != "eu" || channel.ID != CustomerID(testUUID(1)) {
					t.Errorf("not rolled back: event %+v channel %+v", event, channel)
				}
			})
		})
	}
	if committedCount(rt) != 0 {
		t.Fatal("rejected transactions must not persist")
	}
}

func TestFloatValidation(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	reading, channel, err := prepareInRuntime[*Reading, *Orders](context.Background(), rt, func(tx chan<- func() error) *Reading {
		return &Reading{ID: testUUID(3), tx: tx}
	}, ordersFactory(testUUID(1)))
	if err != nil {
		t.Fatal(err)
	}
	reading.Transaction() <- func() error { reading.Value = math.NaN(); return nil }
	if failure := receiveError(t, channel); failure.Kind != InvalidState || len(failure.Events) != 1 || !math.IsNaN(failure.Events[0].(*Reading).Value) {
		t.Fatalf("failure = %v", failure)
	}
	read(reading, func() {
		if reading.Value != 0 {
			t.Errorf("value not rolled back: %v", reading.Value)
		}
	})
}

type Reading struct {
	ID    OrderID
	Value float64
	tx    chan<- func() error
}

func (r *Reading) Fields() Fields {
	return Fields{
		IdentityOf(func(r *Reading) *OrderID { return &r.ID }),
		FieldOf(func(r *Reading) *float64 { return &r.Value }),
	}
}

func (r *Reading) Transaction() chan<- func() error { return r.tx }

// D-052: a rejection pauses the Channel until its error is received; anything
// sent before receipt is rejected, later sends proceed.
func TestRejectionPausesUntilReceived(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	event.Transaction() <- func() error { return errors.New("no") }
	setAmount(event, 1)
	time.Sleep(50 * time.Millisecond)
	if committedCount(rt) != 0 {
		t.Fatal("the Channel must pause until the error is received")
	}
	receiveError(t, channel)
	setAmount(event, 2)
	expectAmount(t, channel, 2)
	if committedCount(rt) != 1 {
		t.Fatalf("committed = %d: the transaction sent before receipt must be rejected", committedCount(rt))
	}
}

// D-051: a failed commit rolls back every transaction in the batch.
func TestCommitFailureRollsBackBatch(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	rt.store.mu.Lock()
	rt.store.writeFailure = errors.New("disk on fire")
	rt.store.mu.Unlock()
	release := gate(event)
	setAmount(event, 1)
	event.Transaction() <- func() error { event.Amount = 2; channel.Region = "us"; return nil }
	release()
	failure := receiveError(t, channel)
	if failure.Kind != CommitFailed || !strings.Contains(failure.Error(), "disk on fire") || len(failure.Events) != 1 {
		t.Fatalf("failure = %v (%d events)", failure, len(failure.Events))
	}
	read(event, func() {
		if event.Amount != 0 || channel.Region != "eu" {
			t.Errorf("batch not rolled back: amount %d region %q", event.Amount, channel.Region)
		}
	})
	rt.store.mu.Lock()
	rt.store.writeFailure = nil
	rt.store.mu.Unlock()
	setAmount(event, 3)
	expectAmount(t, channel, 3)
}

// D-050: a Channel change travels with the Event change it was made with, and
// a Channel record is written only when the Channel snapshot changed.
func TestChannelChangesTravelWithEvents(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	setAmount(event, 1)
	expectAmount(t, channel, 1)
	event.Transaction() <- func() error { channel.Region = "us"; event.Amount = 2; return nil }
	expectAmount(t, channel, 2)
	setAmount(event, 3)
	expectAmount(t, channel, 3)
	fields, _ := persistedChannelFields(rt.store, channel.id())
	if !strings.Contains(fields, `"value":"us"`) {
		t.Fatalf("persisted Channel snapshot = %s", fields)
	}
	rt.store.mu.Lock()
	next := rt.store.nextSequence
	rt.store.mu.Unlock()
	// (Channel, Event, Commit) + (Channel, Event, Commit) + (Event, Commit)
	if next != 9 {
		t.Fatalf("next sequence = %d, want 9", next)
	}
}

// Concurrent senders on many goroutines are serialized race-free.
func TestConcurrentTransactions(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	const senders, each = 8, 25
	var wg sync.WaitGroup
	for s := 0; s < senders; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				event.Transaction() <- func() error { event.Amount++; return nil }
			}
		}()
	}
	for want := int64(1); want <= senders*each; want++ {
		expectAmount(t, channel, want)
	}
	wg.Wait()
}

// D-009/D-011/D-013/D-014, D-049, RT-04: preparation reuses live instances by
// logical identity; one Channel can hold many Events of one type.
func TestPreparationReusesLiveInstances(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	var wg sync.WaitGroup
	events := make([]*OrderPlaced, 16)
	channels := make([]*Orders, 16)
	for i := range events {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			events[i], channels[i], err = prepareInRuntime[*OrderPlaced, *Orders](context.Background(), rt, placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	for i := range events {
		if events[i] != events[0] || channels[i] != channels[0] {
			t.Fatal("same logical identities must return the same live pointers")
		}
	}
	second, sameChannel := prepareOrder(t, rt, testUUID(1), testUUID(3))
	if second == events[0] || sameChannel != channels[0] {
		t.Fatal("a different Event identity gets its own live Event on the same Channel")
	}
	if second.Transaction() != events[0].Transaction() {
		t.Fatal("Events of one Channel share its transaction channel")
	}
	_, otherChannel := prepareOrder(t, rt, testUUID(4), testUUID(2))
	if otherChannel == channels[0] || len(rt.channels) != 2 {
		t.Fatal("distinct logical Channels of one type must have separate runtimes")
	}
}

type conflictingOrder struct {
	ID   OrderID
	A, B int64
	tx   chan<- func() error
}

func (c *conflictingOrder) Fields() Fields {
	fields := Fields{IdentityOf(func(c *conflictingOrder) *OrderID { return &c.ID })}
	if conflictSchemaB.Load() {
		return append(fields, FieldOf(func(c *conflictingOrder) *int64 { return &c.B }))
	}
	return append(fields, FieldOf(func(c *conflictingOrder) *int64 { return &c.A }))
}

func (c *conflictingOrder) Transaction() chan<- func() error { return c.tx }

var conflictSchemaB atomic.Bool

// D-016: the same type with a different schema fails with ErrSchemaConflict.
func TestSchemaConflict(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	factory := func(tx chan<- func() error) *conflictingOrder { return &conflictingOrder{ID: testUUID(2), tx: tx} }
	if _, _, err := prepareInRuntime[*conflictingOrder, *Orders](context.Background(), rt, factory, ordersFactory(testUUID(1))); err != nil {
		t.Fatal(err)
	}
	conflictSchemaB.Store(true)
	defer conflictSchemaB.Store(false)
	if _, _, err := prepareInRuntime[*conflictingOrder, *Orders](context.Background(), rt, factory, ordersFactory(testUUID(1))); !errors.Is(err, ErrSchemaConflict) {
		t.Fatalf("err = %v, want ErrSchemaConflict", err)
	}
}

// D-007/D-008, D-045/D-049: invalid preparations fail before exposing values.
func TestPreparationRejectsInvalidArguments(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	ctx := context.Background()
	orders := ordersFactory(testUUID(1))
	placed := placedFactory(testUUID(2))
	cases := map[string]func() error{
		"interface TEvent": func() error {
			_, _, err := prepareInRuntime[Event, *Orders](ctx, rt, func(chan<- func() error) Event { return nil }, orders)
			return err
		},
		"nil Event factory": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, nil, orders)
			return err
		},
		"nil Channel factory": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, placed, nil)
			return err
		},
		"nil Channel": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, placed, func(chan Event, chan *ChannelError) *Orders { return nil })
			return err
		},
		"panicking Event factory": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, func(chan<- func() error) *OrderPlaced { panic("boom") }, orders)
			return err
		},
		"zero Channel identity": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, placed, ordersFactory(CustomerID{}))
			return err
		},
		"zero Event identity": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, placedFactory(OrderID{}), orders)
			return err
		},
		"wrong transaction channel": func() error {
			_, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, func(chan<- func() error) *OrderPlaced {
				return &OrderPlaced{ID: testUUID(2), tx: make(chan func() error)}
			}, orders)
			return err
		},
	}
	for name, prepare := range cases {
		if err := prepare(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := prepareInRuntime[*OrderPlaced, *Orders](cancelled, rt, placed, orders); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
}

// R-05, R-06, N-12, API-07: Channels are isolated; a slow one does not stall
// another or persistence.
func TestChannelsAreIndependent(t *testing.T) {
	policy := fastPolicy()
	policy.notificationBuffer = 1
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	slowEvent, slow := prepareOrder(t, rt, testUUID(1), testUUID(3))
	fastEvent, fast := prepareOrder(t, rt, testUUID(2), testUUID(4))
	for i := int64(1); i <= 5; i++ {
		setAmount(slowEvent, i)
	}
	waitCommitted(t, rt, 5)
	setAmount(fastEvent, 100)
	expectAmount(t, fast, 100)
	for i := int64(1); i <= 5; i++ {
		expectAmount(t, slow, i)
	}
}

// N-04, N-05, N-06: a consumer that never reads holds only a bounded buffer;
// persistence continues and the Reader resumes from its held position.
func TestBackpressureBoundsBufferWithoutStallingPersistence(t *testing.T) {
	policy := fastPolicy()
	policy.notificationBuffer = 2
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	const total = 20
	for i := int64(1); i <= total; i++ {
		setAmount(event, i)
	}
	waitCommitted(t, rt, total)
	if got := len(channel.notifications); got > policy.notificationBuffer {
		t.Fatalf("buffered %d, capacity %d", got, policy.notificationBuffer)
	}
	for i := int64(1); i <= total; i++ {
		expectAmount(t, channel, i)
	}
}

// R-08, N-03: different concrete Event types share one Channel stream, in
// send order.
func TestHeterogeneousEventsOnOneChannel(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	placed, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	shipped, _, err := prepareInRuntime[*OrderShipped, *Orders](context.Background(), rt, shippedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	if err != nil {
		t.Fatal(err)
	}
	setAmount(placed, 1)
	shipped.Transaction() <- func() error { shipped.Carrier = "dhl"; return nil }
	if _, ok := receiveEvent(t, channel).(*OrderPlaced); !ok {
		t.Fatal("expected OrderPlaced first")
	}
	if got, ok := receiveEvent(t, channel).(*OrderShipped); !ok || got.Carrier != "dhl" {
		t.Fatalf("expected OrderShipped, got %#v", got)
	}
}

// One transaction may change several Events; each becomes an occurrence.
func TestTransactionChangingSeveralEvents(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	placed, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	shipped, _, _ := prepareInRuntime[*OrderShipped, *Orders](context.Background(), rt, shippedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	placed.Transaction() <- func() error {
		placed.Status = "shipped"
		shipped.Carrier = "ups"
		return nil
	}
	receiveEvent(t, channel)
	receiveEvent(t, channel)
	if committedCount(rt) != 2 || countCommits(t, rt) != 1 {
		t.Fatalf("occurrences = %d commits = %d", committedCount(rt), countCommits(t, rt))
	}
}

// R-01, R-03, R-04, R-07, L-12, API-08/09: restart replays committed history
// in order, then continues live on the same progression with no gaps.
func TestReplayThenLiveAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	rt := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	event, _ := prepareOrder(t, rt, testUUID(1), testUUID(2))
	for i := int64(1); i <= 4; i++ {
		setAmount(event, i)
	}
	waitCommitted(t, rt, 4)
	if err := rt.close(); err != nil {
		t.Fatal(err)
	}

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	live, channel := prepareOrder(t, restarted, testUUID(1), testUUID(2))
	setAmount(live, 100)
	for i := int64(1); i <= 4; i++ {
		expectAmount(t, channel, i)
	}
	expectAmount(t, channel, 100)
	expectNoNotification(t, channel, 30*time.Millisecond)
}

// R-10, R-11, D-005/D-006: replay waits, without error, until every decoder
// the Channel's history needs is registered, then delivers in order.
func TestReplayWaitsForRequiredDecoders(t *testing.T) {
	dir := t.TempDir()
	rt := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	placed, _ := prepareOrder(t, rt, testUUID(1), testUUID(2))
	shipped, _, _ := prepareInRuntime[*OrderShipped, *Orders](context.Background(), rt, shippedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	setAmount(placed, 1)
	shipped.Transaction() <- func() error { shipped.Carrier = "ups"; return nil }
	waitCommitted(t, rt, 2)
	_ = rt.close()

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	_, channel := prepareOrder(t, restarted, testUUID(1), testUUID(2))
	expectNoNotification(t, channel, 100*time.Millisecond)
	if _, _, err := prepareInRuntime[*OrderShipped, *Orders](context.Background(), restarted, shippedFactory(testUUID(2)), ordersFactory(testUUID(1))); err != nil {
		t.Fatal(err)
	}
	if _, ok := receiveEvent(t, channel).(*OrderPlaced); !ok {
		t.Fatal("expected OrderPlaced first")
	}
	if _, ok := receiveEvent(t, channel).(*OrderShipped); !ok {
		t.Fatal("expected OrderShipped second")
	}
}

// N-07, N-08, N-10: a temporary failure retries the same occurrence and later
// occurrences cannot overtake it.
func TestDeliveryRetriesSameOccurrence(t *testing.T) {
	policy := fastPolicy()
	var failures atomic.Int32
	failures.Store(3)
	policy.deliveryFault = func(_ uuid, event Event) error {
		if event.(*OrderPlaced).Amount == 1 && failures.Add(-1) >= 0 {
			return errors.New("temporary")
		}
		return nil
	}
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	setAmount(event, 1)
	setAmount(event, 2)
	expectAmount(t, channel, 1)
	expectAmount(t, channel, 2)
	if failures.Load() >= 0 {
		t.Fatal("the fault was not exercised")
	}
}

// N-09, N-14, R-13, RT-12: retry exhaustion terminates only that Channel's
// Reader with DeliveryFailed and closes its streams; persistence and other
// Channels continue, and history stays recoverable.
func TestDeliveryRetryExhaustionTerminatesOnlyThatChannel(t *testing.T) {
	policy := fastPolicy()
	doomed := (&Orders{ID: testUUID(1)}).id()
	policy.deliveryFault = func(channelID uuid, _ Event) error {
		if channelID == doomed {
			return errors.New("permanent")
		}
		return nil
	}
	dir := t.TempDir()
	rt := openTestRuntime(t, dir, fastStoreOptions(), policy)
	failing, failingChannel := prepareOrder(t, rt, testUUID(1), testUUID(3))
	healthy, healthyChannel := prepareOrder(t, rt, testUUID(2), testUUID(4))
	setAmount(failing, 1)
	if failure := receiveError(t, failingChannel); failure.Kind != DeliveryFailed {
		t.Fatalf("fatal error = %v", failure)
	}
	if _, open := <-failingChannel.Errors(); open {
		t.Fatal("error stream must close after the fatal error")
	}
	if _, open := <-failingChannel.Notifications(); open {
		t.Fatal("notification stream must close")
	}
	setAmount(healthy, 7)
	expectAmount(t, healthyChannel, 7)
	ran := make(chan struct{})
	failing.Transaction() <- func() error {
		close(ran)
		return errors.New("rejected without pausing")
	}
	<-ran
	// Anything queued behind the failure is rejected even though no one can
	// receive the error; give the rejection time to finish before resending.
	time.Sleep(50 * time.Millisecond)
	setAmount(failing, 2)
	waitCommitted(t, rt, 3)
	_ = rt.close()

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	_, channel := prepareOrder(t, restarted, testUUID(1), testUUID(3))
	expectAmount(t, channel, 1)
	expectAmount(t, channel, 2)
}

// A consumer closing its own notification stream is a delivery failure.
func TestConsumerClosedStreamIsDeliveryFailure(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	close(channel.notifications)
	setAmount(event, 1)
	if failure := receiveError(t, channel); failure.Kind != DeliveryFailed || !strings.Contains(failure.Error(), "closed channel") {
		t.Fatalf("error = %v", failure)
	}
}

// R-12: a persisted record that does not match its registered type is never
// silently skipped.
func TestUndecodableOccurrenceIsFatal(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	_, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	schema, _ := resolveTypeSchema(&OrderPlaced{})
	err := rt.store.appendTransaction(channelWrite{id: channel.id(), typ: typeKey{pkgPath: "github.com/gunesis-game/eventlog", name: "Orders"}}, []occurrenceWrite{{
		channelFields: map[string]interface{}{"ID": fieldValue{Type: "uuid", Value: uuid(testUUID(1)).String()}, "Region": fieldValue{Type: "string", Value: "eu"}},
		eventID:       uuid(testUUID(5)),
		eventType:     schema.key(),
		eventFields:   map[string]interface{}{"Unexpected": fieldValue{Type: "bool", Value: true}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if failure := receiveError(t, channel); failure.Kind != ReaderFailed || !strings.Contains(failure.Error(), "fields") {
		t.Fatalf("error = %v", failure)
	}
}

// RT-05/06/07: cancelling a preparer's Context does not stop shared workers.
func TestCallerContextDoesNotOwnRuntime(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	ctx, cancel := context.WithCancel(context.Background())
	event, channel, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	setAmount(event, 1)
	expectAmount(t, channel, 1)
}

// R-14: coalesced or missed wake-ups cannot lose commits because the Reader
// rechecks the committed log after each wake.
func TestBurstOfCommitsDeliversEveryOccurrence(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	_, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	schema, _ := resolveTypeSchema(&OrderPlaced{})
	orderID := uuid(testUUID(2))
	eventID, _ := deriveIdentity(eventIdentityDomain, schema, orderID)
	const burst = 200
	for i := 0; i < burst; i++ {
		err := rt.store.appendTransaction(channelWrite{id: channel.id(), typ: typeKey{pkgPath: "github.com/gunesis-game/eventlog", name: "Orders"}}, []occurrenceWrite{{
			channelFields: map[string]interface{}{"ID": fieldValue{Type: "uuid", Value: uuid(testUUID(1)).String()}, "Region": fieldValue{Type: "string", Value: "eu"}},
			eventID:       eventID,
			eventType:     schema.key(),
			eventFields: map[string]interface{}{
				"ID": fieldValue{Type: "uuid", Value: orderID.String()}, "Amount": fieldValue{Type: "int64", Value: fmt.Sprint(i)},
				"Status": fieldValue{Type: "string", Value: ""}, "Note": fieldValue{Type: "null"},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := int64(0); i < burst; i++ {
		expectAmount(t, channel, i)
	}
}

// ChannelError messages name the Channel, kind, Event, and cause.
func TestChannelErrorMessage(t *testing.T) {
	attempted := &OrderPlaced{ID: testUUID(2)}
	failure := &ChannelError{Kind: TransactionRefused, Cause: errors.New("door jammed"), Events: []Event{attempted}, channelID: uuid(testUUID(1))}
	message := failure.Error()
	for _, part := range []string{"channel 01010101-", "transaction refused", "OrderPlaced ", "door jammed", "queued behind it are rejected"} {
		if !strings.Contains(message, part) {
			t.Fatalf("message %q lacks %q", message, part)
		}
	}
	fatal := &ChannelError{Kind: ReaderFailed, Cause: errors.New("bad"), channelID: uuid(testUUID(1))}
	if strings.Contains(fatal.Error(), "queued") {
		t.Fatalf("fatal message mentions queued transactions: %q", fatal.Error())
	}
	if ErrorKind(99).String() != "ErrorKind(99)" {
		t.Fatal(ErrorKind(99).String())
	}
}
