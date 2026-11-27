package eventlog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func placeOrder(rt *logRuntime, event *OrderPlaced, id OrderID, amount int64) {
	mutate(rt, func() {
		event.ID = id
		event.Amount = amount
	})
}

func setAmount(rt *logRuntime, event *OrderPlaced, amount int64) {
	mutate(rt, func() {
		event.Amount = amount
	})
}

func expectAmount(t *testing.T, channel Channel, amount int64) *OrderPlaced {
	t.Helper()
	got, ok := receiveEvent(t, channel).(*OrderPlaced)
	if !ok {
		t.Fatal("expected *OrderPlaced")
	}
	if got.Amount != amount {
		t.Fatalf("amount = %d, want %d", got.Amount, amount)
	}
	return got
}

// API-03/04/06, N-01, N-02, L-06, R-02: each observed change becomes an
// ordered occurrence delivered as a fresh concrete snapshot.
func TestChangesBecomeOrderedNotifications(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 10)
	first := expectAmount(t, channel, 10)
	setAmount(rt, event, 20)
	expectAmount(t, channel, 20)
	if first == event {
		t.Fatal("notification must be a fresh snapshot, not the live pointer")
	}
	if first.Amount != 10 || first.ID != OrderID(testUUID(2)) {
		t.Fatalf("earlier snapshot changed: %+v", first)
	}
	mutate(rt, func() {
		event.Note = stringPointer("gift")
		event.Status = "paid"
	})
	got := receiveEvent(t, channel).(*OrderPlaced)
	if got.Note == nil || *got.Note != "gift" || got.Status != "paid" || got.Amount != 20 {
		t.Fatalf("snapshot = %+v", got)
	}
	if got.ignore != "" {
		t.Fatal("unselected fields must not be persisted")
	}
}

// D-021: unchanged observations are suppressed; A-B-A transitions persist.
func TestUnchangedSuppressedAndRevertIsNewOccurrence(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 1)
	expectAmount(t, channel, 1)
	expectNoNotification(t, channel, 50*time.Millisecond)
	setAmount(rt, event, 2)
	expectAmount(t, channel, 2)
	setAmount(rt, event, 1)
	expectAmount(t, channel, 1)
	if committedCount(rt) != 3 {
		t.Fatalf("committed = %d, want 3", committedCount(rt))
	}
}

// D-045: an untouched instance is ignored; a change with a zero identity is
// rejected and reported; a later non-zero identity binds.
func TestZeroEventIdentity(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	expectNoNotification(t, channel, 30*time.Millisecond)
	setAmount(rt, event, 5)
	if err := receiveError(t, channel); !strings.Contains(err.Error(), "zero UUID") {
		t.Fatalf("error = %v", err)
	}
	if committedCount(rt) != 0 {
		t.Fatal("zero identity change must not persist")
	}
	mutate(rt, func() { event.ID = testUUID(3) })
	expectAmount(t, channel, 5)
}

// D-019: a bound identity is immutable; mutation is reported and rejected.
func TestEventIdentityMutationRejected(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 1)
	expectAmount(t, channel, 1)
	mutate(rt, func() {
		event.ID = testUUID(9)
		event.Amount = 2
	})
	if err := receiveError(t, channel); !strings.Contains(err.Error(), "mutated") {
		t.Fatalf("error = %v", err)
	}
	expectNoNotification(t, channel, 30*time.Millisecond)
	mutate(rt, func() {
		event.ID = testUUID(2)
	})
	expectAmount(t, channel, 2)
	if committedCount(rt) != 2 {
		t.Fatalf("committed = %d", committedCount(rt))
	}
}

// Invalid field values are reported and never persisted.
func TestInvalidFieldValueNotPersisted(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 1)
	expectAmount(t, channel, 1)
	mutate(rt, func() { event.Status = "\xff" })
	if err := receiveError(t, channel); !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("error = %v", err)
	}
	if committedCount(rt) != 1 {
		t.Fatal("invalid value was persisted")
	}
}

// D-009/D-011/D-013/D-014, RT-04: repeated and concurrent preparation
// converges on one Channel runtime and one live Event per type.
func TestPreparationReusesLiveInstances(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	var wg sync.WaitGroup
	events := make([]*OrderPlaced, 16)
	channels := make([]*Orders, 16)
	shipped := make([]*OrderShipped, 16)
	for i := range events {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			events[i], channels[i], err = prepareInRuntime[*OrderPlaced, *Orders](context.Background(), rt, ordersFactory(testUUID(1), "eu"))
			if err != nil {
				t.Error(err)
			}
			shipped[i], _, err = prepareInRuntime[*OrderShipped, *Orders](context.Background(), rt, ordersFactory(testUUID(1), "eu"))
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	for i := range events {
		if events[i] != events[0] || channels[i] != channels[0] || shipped[i] != shipped[0] {
			t.Fatal("compatible preparations must return the same live pointers")
		}
	}
	other, otherChannel := prepareOrders(t, rt, testUUID(2))
	if other == events[0] || otherChannel == channels[0] {
		t.Fatal("distinct logical Channels of one type must have separate runtimes")
	}
	if len(rt.channels) != 2 {
		t.Fatalf("channel runtimes = %d", len(rt.channels))
	}
}

type conflictingOrder struct {
	ID    OrderID
	A, B  int64
	which bool
}

func (c *conflictingOrder) Fields() Fields {
	fields := Fields{IdentityOf(func(c *conflictingOrder) *OrderID { return &c.ID })}
	if conflictSchemaB.Load() {
		return append(fields, FieldOf(func(c *conflictingOrder) *int64 { return &c.B }))
	}
	return append(fields, FieldOf(func(c *conflictingOrder) *int64 { return &c.A }))
}

var conflictSchemaB atomic.Bool

// D-016: the same type with a different schema fails with ErrSchemaConflict.
func TestSchemaConflict(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	if _, _, err := prepareInRuntime[*conflictingOrder, *Orders](context.Background(), rt, ordersFactory(testUUID(1), "eu")); err != nil {
		t.Fatal(err)
	}
	conflictSchemaB.Store(true)
	defer conflictSchemaB.Store(false)
	if _, _, err := prepareInRuntime[*conflictingOrder, *Orders](context.Background(), rt, ordersFactory(testUUID(1), "eu")); !errors.Is(err, ErrSchemaConflict) {
		t.Fatalf("err = %v, want ErrSchemaConflict", err)
	}
}

type valueChannel struct{ Orders }

// D-007/D-008, D-045: invalid preparations fail before exposing values.
func TestPreparationRejectsInvalidArguments(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	ctx := context.Background()
	if _, _, err := prepareInRuntime[Event, *Orders](ctx, rt, ordersFactory(testUUID(1), "eu")); err == nil {
		t.Fatal("interface TEvent must be rejected")
	}
	if _, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, nil); err == nil {
		t.Fatal("nil factory must be rejected")
	}
	if _, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, func(chan Event, chan error) *Orders { return nil }); err == nil {
		t.Fatal("nil Channel must be rejected")
	}
	if _, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, func(chan Event, chan error) *Orders { panic("boom") }); err == nil {
		t.Fatal("panicking factory must be rejected")
	}
	if _, _, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, ordersFactory(CustomerID{}, "eu")); err == nil || !strings.Contains(err.Error(), "zero UUID") {
		t.Fatalf("zero Channel identity: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := prepareInRuntime[*OrderPlaced, *Orders](cancelled, rt, ordersFactory(testUUID(1), "eu")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context: %v", err)
	}
	if len(rt.channels) != 0 {
		t.Fatal("failed preparations must not leave Channel runtimes")
	}
}

// R-05, R-06, N-12, API-07: Channels are isolated; a slow one does not
// stall another or persistence.
func TestChannelsAreIndependent(t *testing.T) {
	policy := fastPolicy()
	policy.notificationBuffer = 1
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	slowEvent, slow := prepareOrders(t, rt, testUUID(1))
	fastEvent, fast := prepareOrders(t, rt, testUUID(2))
	placeOrder(rt, slowEvent, testUUID(3), 0)
	waitCommitted(t, rt, 1)
	for i := int64(1); i <= 5; i++ {
		setAmount(rt, slowEvent, i)
		waitCommitted(t, rt, int(i)+1)
	}
	placeOrder(rt, fastEvent, testUUID(4), 100)
	expectAmount(t, fast, 100)
	waitCommitted(t, rt, 7)
	for i := int64(0); i <= 5; i++ {
		expectAmount(t, slow, i)
	}
}

// N-04, N-05, N-06: a consumer that never reads holds only a bounded buffer;
// persistence continues and the Reader resumes from its held position.
func TestBackpressureBoundsBufferWithoutStallingPersistence(t *testing.T) {
	policy := fastPolicy()
	policy.notificationBuffer = 2
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 0)
	waitCommitted(t, rt, 1)
	const total = 20
	for i := int64(1); i < total; i++ {
		setAmount(rt, event, i)
		waitCommitted(t, rt, int(i)+1)
	}
	if got := len(channel.notifications); got > policy.notificationBuffer {
		t.Fatalf("buffered %d, capacity %d", got, policy.notificationBuffer)
	}
	for i := int64(0); i < total; i++ {
		expectAmount(t, channel, i)
	}
}

// R-08, N-03: different concrete Event types share one Channel stream.
func TestHeterogeneousEventsOnOneChannel(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	placed, channel := prepareOrders(t, rt, testUUID(1))
	shipped, _, err := prepareInRuntime[*OrderShipped, *Orders](context.Background(), rt, ordersFactory(testUUID(1), "eu"))
	if err != nil {
		t.Fatal(err)
	}
	placeOrder(rt, placed, testUUID(2), 1)
	if _, ok := receiveEvent(t, channel).(*OrderPlaced); !ok {
		t.Fatal("expected OrderPlaced")
	}
	mutate(rt, func() {
		shipped.ID = testUUID(2)
		shipped.Carrier = "dhl"
	})
	if got, ok := receiveEvent(t, channel).(*OrderShipped); !ok || got.Carrier != "dhl" {
		t.Fatalf("expected OrderShipped, got %#v", got)
	}
}

// Channel-only changes create no occurrence; the next occurrence carries the
// latest Channel snapshot and earlier associations stay unchanged.
func TestChannelSnapshotUpdates(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 1)
	expectAmount(t, channel, 1)
	mutate(rt, func() { channel.Region = "us" })
	expectNoNotification(t, channel, 50*time.Millisecond)
	if committedCount(rt) != 1 {
		t.Fatal("a Channel-only change must not create an occurrence")
	}
	setAmount(rt, event, 2)
	expectAmount(t, channel, 2)
	fields, _ := rt.store.persistedChannelFields(channel.id())
	if !strings.Contains(fields, `"value":"us"`) {
		t.Fatalf("latest persisted Channel snapshot = %s", fields)
	}
	if rt.store.nextSequence != 7 {
		t.Fatalf("next sequence = %d: want Channel+Event+Commit, then Channel+Event+Commit", rt.store.nextSequence)
	}
	mutate(rt, func() { channel.ID = testUUID(8) })
	if err := receiveError(t, channel); !strings.Contains(err.Error(), "mutated") {
		t.Fatalf("Channel identity mutation error = %v", err)
	}
}

func (c *Orders) id() uuid {
	schema, _ := resolveTypeSchema(&Orders{})
	id, _ := deriveIdentity(channelIdentityDomain, schema, uuid(c.ID))
	return id
}

// R-01, R-03, R-04, R-07, L-12, API-08/09: restart replays committed history
// in order, then continues live on the same progression with no gaps.
func TestReplayThenLiveAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	rt := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	event, _ := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 0)
	waitCommitted(t, rt, 1)
	for i := int64(1); i < 5; i++ {
		setAmount(rt, event, i)
		waitCommitted(t, rt, int(i)+1)
	}
	if err := rt.close(); err != nil {
		t.Fatal(err)
	}

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	live, channel := prepareOrders(t, restarted, testUUID(1))
	placeOrder(restarted, live, testUUID(2), 100)
	for i := int64(0); i < 5; i++ {
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
	placed, _ := prepareOrders(t, rt, testUUID(1))
	shipped, _, _ := prepareInRuntime[*OrderShipped, *Orders](context.Background(), rt, ordersFactory(testUUID(1), "eu"))
	placeOrder(rt, placed, testUUID(2), 1)
	waitCommitted(t, rt, 1)
	mutate(rt, func() {
		shipped.ID = testUUID(2)
		shipped.Carrier = "ups"
	})
	waitCommitted(t, rt, 2)
	_ = rt.close()

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	_, channel := prepareOrders(t, restarted, testUUID(1))
	expectNoNotification(t, channel, 100*time.Millisecond)
	select {
	case err := <-channel.Errors():
		t.Fatalf("a missing decoder must not be an error: %v", err)
	default:
	}
	if _, _, err := prepareInRuntime[*OrderShipped, *Orders](context.Background(), restarted, ordersFactory(testUUID(1), "eu")); err != nil {
		t.Fatal(err)
	}
	if _, ok := receiveEvent(t, channel).(*OrderPlaced); !ok {
		t.Fatal("expected OrderPlaced first")
	}
	if _, ok := receiveEvent(t, channel).(*OrderShipped); !ok {
		t.Fatal("expected OrderShipped second")
	}
}

// N-07, N-08, N-10: a temporary failure retries the same occurrence and
// later occurrences cannot overtake it.
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
	event, channel := prepareOrders(t, rt, testUUID(1))
	placeOrder(rt, event, testUUID(2), 1)
	waitCommitted(t, rt, 1)
	setAmount(rt, event, 2)
	expectAmount(t, channel, 1)
	expectAmount(t, channel, 2)
	if failures.Load() >= 0 {
		t.Fatal("the fault was not exercised")
	}
}

// N-09, N-14, R-13, RT-12: retry exhaustion terminates only that Channel's
// Reader, reports the first fatal error, and closes its streams; persistence
// and other Channels continue, and history stays recoverable.
func TestDeliveryRetryExhaustionTerminatesOnlyThatChannel(t *testing.T) {
	policy := fastPolicy()
	doomed := CustomerID(testUUID(1))
	policy.deliveryFault = func(channelID uuid, _ Event) error {
		var c Orders
		c.ID = doomed
		if channelID == c.id() {
			return errors.New("permanent")
		}
		return nil
	}
	dir := t.TempDir()
	rt := openTestRuntime(t, dir, fastStoreOptions(), policy)
	failing, failingChannel := prepareOrders(t, rt, doomed)
	healthy, healthyChannel := prepareOrders(t, rt, testUUID(2))
	placeOrder(rt, failing, testUUID(3), 1)
	err := receiveError(t, failingChannel)
	if err == nil || !strings.Contains(err.Error(), "failed after") {
		t.Fatalf("fatal error = %v", err)
	}
	if _, open := <-failingChannel.Errors(); open {
		t.Fatal("error stream must close after the fatal error")
	}
	if _, open := <-failingChannel.Notifications(); open {
		t.Fatal("notification stream must close")
	}
	placeOrder(rt, healthy, testUUID(4), 7)
	expectAmount(t, healthyChannel, 7)
	setAmount(rt, failing, 2)
	waitCommitted(t, rt, 3)
	_ = rt.close()

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	_, channel := prepareOrders(t, restarted, doomed)
	expectAmount(t, channel, 1)
	expectAmount(t, channel, 2)
}

// A consumer closing its own notification stream is a delivery failure, not a
// crash.
func TestConsumerClosedStreamIsDeliveryFailure(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	event, channel := prepareOrders(t, rt, testUUID(1))
	close(channel.notifications)
	placeOrder(rt, event, testUUID(2), 1)
	if err := receiveError(t, channel); err == nil || !strings.Contains(err.Error(), "closed channel") {
		t.Fatalf("error = %v", err)
	}
}

// R-12: a persisted record that does not match its registered type is never
// silently skipped.
func TestUndecodableOccurrenceIsFatal(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	_, channel := prepareOrders(t, rt, testUUID(1))
	schema, _ := resolveTypeSchema(&OrderPlaced{})
	_, err := rt.store.appendOccurrence(
		channelWrite{id: channel.id(), typ: typeKey{pkgPath: "github.com/gunesis-game/eventlog", name: "Orders"}, fields: map[string]interface{}{"ID": fieldValue{Type: "uuid", Value: uuid(testUUID(1)).String()}, "Region": fieldValue{Type: "string", Value: "eu"}}},
		eventWrite{id: uuid(testUUID(5)), typ: schema.key(), fields: map[string]interface{}{"Unexpected": fieldValue{Type: "bool", Value: true}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiveError(t, channel); err == nil || !strings.Contains(err.Error(), "fields") {
		t.Fatalf("error = %v", err)
	}
}

// RT-05/06/07: cancelling a preparer's Context does not stop shared workers.
func TestCallerContextDoesNotOwnRuntime(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	ctx, cancel := context.WithCancel(context.Background())
	event, channel, err := prepareInRuntime[*OrderPlaced, *Orders](ctx, rt, ordersFactory(testUUID(1), "eu"))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	placeOrder(rt, event, testUUID(2), 1)
	expectAmount(t, channel, 1)
}

// R-14: coalesced or missed wake-ups cannot lose commits because the Reader
// rechecks the committed log after each wake.
func TestBurstOfCommitsDeliversEveryOccurrence(t *testing.T) {
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), fastPolicy())
	_, channel := prepareOrders(t, rt, testUUID(1))
	schema, _ := resolveTypeSchema(&OrderPlaced{})
	channelFields := map[string]interface{}{"ID": fieldValue{Type: "uuid", Value: uuid(testUUID(1)).String()}, "Region": fieldValue{Type: "string", Value: "eu"}}
	orderID := uuid(testUUID(2))
	eventID, _ := deriveIdentity(eventIdentityDomain, schema, orderID)
	const burst = 200
	for i := 0; i < burst; i++ {
		_, err := rt.store.appendOccurrence(
			channelWrite{id: channel.id(), typ: typeKey{pkgPath: "github.com/gunesis-game/eventlog", name: "Orders"}, fields: channelFields},
			eventWrite{id: eventID, typ: schema.key(), fields: map[string]interface{}{
				"ID": fieldValue{Type: "uuid", Value: orderID.String()}, "Amount": fieldValue{Type: "int64", Value: fmt.Sprint(i)},
				"Status": fieldValue{Type: "string", Value: ""}, "Note": fieldValue{Type: "null"},
			}},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := int64(0); i < burst; i++ {
		expectAmount(t, channel, i)
	}
}
