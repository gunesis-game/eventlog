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
}

func (o *OrderPlaced) Fields() Fields {
	return Fields{
		IdentityOf(func(o *OrderPlaced) *OrderID { return &o.ID }),
		FieldOf(func(o *OrderPlaced) *int64 { return &o.Amount }),
		FieldOf(func(o *OrderPlaced) *OrderStatus { return &o.Status }),
		FieldOfNullable(func(o *OrderPlaced) **string { return &o.Note }),
	}
}

type OrderShipped struct {
	ID      OrderID
	Carrier string
}

func (o *OrderShipped) Fields() Fields {
	return Fields{
		IdentityOf(func(o *OrderShipped) *OrderID { return &o.ID }),
		FieldOf(func(o *OrderShipped) *string { return &o.Carrier }),
	}
}

type Orders struct {
	ID            CustomerID
	Region        string
	notifications chan Event
	errs          chan error
}

func (c *Orders) Fields() Fields {
	return Fields{
		IdentityOf(func(c *Orders) *CustomerID { return &c.ID }),
		FieldOf(func(c *Orders) *string { return &c.Region }),
	}
}

func (c *Orders) Notifications() <-chan Event { return c.notifications }
func (c *Orders) Errors() <-chan error        { return c.errs }

func ordersFactory(id CustomerID, region string) func(chan Event, chan error) *Orders {
	return func(notifications chan Event, errs chan error) *Orders {
		return &Orders{ID: id, Region: region, notifications: notifications, errs: errs}
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
	policy.pollInterval = 5 * time.Millisecond
	policy.deliveryBackoff = time.Millisecond
	policy.persistBackoff = time.Millisecond
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

func prepareOrders(t *testing.T, rt *logRuntime, channelID CustomerID) (*OrderPlaced, *Orders) {
	t.Helper()
	event, channel, err := prepareInRuntime[*OrderPlaced, *Orders](context.Background(), rt, ordersFactory(channelID, "eu"))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return event, channel
}

// mutate changes consumer-owned fields while no poll pass runs, which is the
// synchronization a real consumer must provide around concurrent mutation.
func mutate(rt *logRuntime, fn func()) {
	rt.changes.withPollingPaused(fn)
}

// waitCommitted waits until the store holds n committed Event occurrences.
func waitCommitted(t *testing.T, rt *logRuntime, n int) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for time.Now().Before(deadline) {
		rt.store.mu.Lock()
		count := len(rt.store.entries)
		rt.store.mu.Unlock()
		if count >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d committed occurrences", n)
}

func committedCount(rt *logRuntime) int {
	rt.store.mu.Lock()
	defer rt.store.mu.Unlock()
	return len(rt.store.entries)
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

func receiveError(t *testing.T, channel Channel) error {
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

func stringPointer(s string) *string {
	return &s
}
