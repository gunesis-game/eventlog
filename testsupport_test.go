package eventlog

import (
	"context"
	"testing"
	"time"
)

// Consumer-style test types: named UUID identity types, primitives, named
// primitives, and nullable fields, mirroring how library consumers define them.

type OrderID [16]byte
type CustomerID [16]byte
type OrderStatus string

type OrderPlaced struct {
	ID     OrderID
	Amount int64
	Status OrderStatus
	Note   *string
	ignore string
	tx     chan<- func() error
}

func (o *OrderPlaced) Fields() Fields {
	return Fields{
		IdentityOf(func(o *OrderPlaced) *OrderID { return &o.ID }),
		FieldOf(func(o *OrderPlaced) *int64 { return &o.Amount }),
		FieldOf(func(o *OrderPlaced) *OrderStatus { return &o.Status }),
		FieldOfNullable(func(o *OrderPlaced) **string { return &o.Note }),
	}
}

func (o *OrderPlaced) Transaction() chan<- func() error { return o.tx }

type OrderShipped struct {
	ID      OrderID
	Carrier string
	tx      chan<- func() error
}

func (o *OrderShipped) Fields() Fields {
	return Fields{
		IdentityOf(func(o *OrderShipped) *OrderID { return &o.ID }),
		FieldOf(func(o *OrderShipped) *string { return &o.Carrier }),
	}
}

func (o *OrderShipped) Transaction() chan<- func() error { return o.tx }

type Orders struct {
	ID            CustomerID
	Region        string
	notifications chan Event
	errs          chan *ChannelError
}

func (c *Orders) Fields() Fields {
	return Fields{
		IdentityOf(func(c *Orders) *CustomerID { return &c.ID }),
		FieldOf(func(c *Orders) *string { return &c.Region }),
	}
}

func (c *Orders) Notifications() <-chan Event  { return c.notifications }
func (c *Orders) Errors() <-chan *ChannelError { return c.errs }

func ordersFactory(id CustomerID) func(chan Event, chan *ChannelError) *Orders {
	return func(notifications chan Event, errs chan *ChannelError) *Orders {
		return &Orders{ID: id, Region: "eu", notifications: notifications, errs: errs}
	}
}

func placedFactory(id OrderID) func(chan<- func() error) *OrderPlaced {
	return func(tx chan<- func() error) *OrderPlaced {
		return &OrderPlaced{ID: id, tx: tx}
	}
}

func shippedFactory(id OrderID) func(chan<- func() error) *OrderShipped {
	return func(tx chan<- func() error) *OrderShipped {
		return &OrderShipped{ID: id, tx: tx}
	}
}

func testUUID(n byte) [16]byte {
	var u [16]byte
	for i := range u {
		u[i] = n
	}
	return u
}

const testWait = 5 * time.Second

func fastStoreOptions() segmentStoreOptions {
	options := defaultSegmentStoreOptions()
	options.syncInterval = 20 * time.Millisecond
	return options
}

func fastPolicy() runtimePolicy {
	policy := defaultRuntimePolicy()
	policy.deliveryBackoff = time.Millisecond
	return policy
}

// openTestRuntime opens a runtime on dir and closes it (test-only) at cleanup.
func openTestRuntime(t *testing.T, dir string, options segmentStoreOptions, policy runtimePolicy) *logRuntime {
	t.Helper()
	rt, err := openLogRuntime(dir, options, policy)
	if err != nil {
		t.Fatalf("open runtime: %v", err)
	}
	t.Cleanup(func() {
		_ = rt.close()
	})
	return rt
}

func prepareOrder(t *testing.T, rt *logRuntime, channelID CustomerID, orderID OrderID) (*OrderPlaced, *Orders) {
	t.Helper()
	event, channel, err := prepareInRuntime[*OrderPlaced, *Orders](context.Background(), rt, placedFactory(orderID), ordersFactory(channelID))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return event, channel
}

// gate blocks the Channel's transaction processor until released, so a test
// can queue transactions that are then applied together in one batch.
func gate(event Event) (release func()) {
	opened := make(chan struct{})
	event.Transaction() <- func() error {
		<-opened
		return nil
	}
	return func() {
		close(opened)
	}
}

func setAmount(event *OrderPlaced, amount int64) {
	event.Transaction() <- func() error {
		event.Amount = amount
		return nil
	}
}

// waitCommitted waits until the store holds n committed Event occurrences.
func waitCommitted(t *testing.T, rt *logRuntime, n int) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for time.Now().Before(deadline) {
		if committedCount(rt) >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d committed occurrences (have %d)", n, committedCount(rt))
}

func committedCount(rt *logRuntime) int {
	rt.store.mu.Lock()
	defer rt.store.mu.Unlock()
	return len(rt.store.entries)
}

// read runs fn inside a transaction, the race-free way to read live fields,
// and waits for it to finish.
func read(event Event, fn func()) {
	done := make(chan struct{})
	event.Transaction() <- func() error {
		fn()
		close(done)
		return nil
	}
	<-done
}

func receiveEvent(t *testing.T, channel Channel) Event {
	t.Helper()
	select {
	case event, ok := <-channel.Notifications():
		if !ok {
			t.Fatal("notification stream closed")
		}
		return event
	case <-time.After(testWait):
		t.Fatal("timed out waiting for a notification")
		return nil
	}
}

func receiveError(t *testing.T, channel Channel) *ChannelError {
	t.Helper()
	select {
	case err := <-channel.Errors():
		return err
	case <-time.After(testWait):
		t.Fatal("timed out waiting for an error")
		return nil
	}
}

func expectNoNotification(t *testing.T, channel Channel, wait time.Duration) {
	t.Helper()
	select {
	case event := <-channel.Notifications():
		t.Fatalf("unexpected notification %#v", event)
	case <-time.After(wait):
	}
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

func stringPointer(s string) *string {
	return &s
}

func (c *Orders) id() uuid {
	schema, _ := resolveTypeSchema(&Orders{})
	id, _ := deriveIdentity(channelIdentityDomain, schema, uuid(c.ID))
	return id
}

// persistedChannelFields returns the latest persisted Channel fields JSON.
func persistedChannelFields(s *segmentStore, channelID uuid) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.channels[channelID]
	return channel.fields, ok
}
