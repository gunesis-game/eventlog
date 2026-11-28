package eventlog

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/storage"
)

// Representative smoke benchmarks for the approved catalogue in
// docs/project/benchmarks.md. Each checks the correctness of its workload;
// they are not the full workload/profile protocol and carry no numeric
// targets (D-040).

// benchmarkWait bounds every wait, so a lost notification or commit fails
// the benchmark instead of hanging it.
const benchmarkWait = time.Minute

// openBenchmarkRuntime opens a runtime on a fresh directory and closes it
// (test-only) when the benchmark ends.
func openBenchmarkRuntime(b *testing.B) *logRuntime {
	b.Helper()
	dir := b.TempDir()
	rt, err := openLogRuntime(dir, storage.DefaultOptions(), defaultRuntimePolicy())
	if err != nil {
		b.Fatal(err)
	}
	runtimeDirectories.Store(rt, dir)
	b.Cleanup(func() {
		_ = rt.close()
	})
	return rt
}

// receiveAmount waits for the next notification and checks it carries amount.
func receiveAmount(b *testing.B, channel *Orders, amount int64) {
	b.Helper()
	select {
	case event, ok := <-channel.Notifications():
		if !ok {
			b.Fatal("notification stream closed")
		}
		placed, isPlaced := event.(*OrderPlaced)
		if !isPlaced || placed.Amount != amount {
			b.Fatalf("notification %#v, want *OrderPlaced with amount %d", event, amount)
		}
	case <-time.After(benchmarkWait):
		b.Fatalf("timed out waiting for the notification with amount %d", amount)
	}
}

// writeBenchmarkHistory commits size occurrences of one Order, with amounts
// 0 to size-1, directly to a store in dir.
func writeBenchmarkHistory(b *testing.B, dir string, size int) {
	b.Helper()
	setup, err := storage.Open(dir, storage.DefaultOptions())
	if err != nil {
		b.Fatal(err)
	}
	schema, _ := resolveSchema(&OrderPlaced{})
	channelSchema, _ := resolveSchema(&Orders{})
	channelID, _ := identity.Derive(identity.ChannelDomain, channelSchema.Descriptor(), identity.UUID(testUUID(1)))
	orderID := identity.UUID(testUUID(2))
	eventID, _ := identity.Derive(identity.EventDomain, schema.Descriptor(), orderID)
	channel := storage.ChannelWrite{ID: channelID, Type: channelSchema.Key()}
	channelFields := map[string]interface{}{"ID": fieldschema.Value{Type: "uuid", Value: identity.UUID(testUUID(1)).String()}, "Region": fieldschema.Value{Type: "string", Value: "eu"}}
	for i := 0; i < size; i++ {
		occurrence := storage.OccurrenceWrite{ChannelFields: channelFields, EventID: eventID, EventType: schema.Key(), EventFields: map[string]interface{}{
			"ID": fieldschema.Value{Type: "uuid", Value: orderID.String()}, "Amount": fieldschema.Value{Type: "int64", Value: fmt.Sprint(i)},
			"Status": fieldschema.Value{Type: "string", Value: "placed"}, "Note": fieldschema.Value{Type: "null"},
		}}
		if err := setup.AppendTransaction(channel, []storage.OccurrenceWrite{occurrence}); err != nil {
			b.Fatal(err)
		}
	}
	if err := setup.Close(); err != nil {
		b.Fatal(err)
	}
}

// B-R01: initial replay of a committed history of 1,000 occurrences through
// one Channel Reader, in order.
func BenchmarkReplayDelivery(b *testing.B) {
	const history = 1000
	dir := b.TempDir()
	writeBenchmarkHistory(b, dir, history)
	b.ReportAllocs()
	for b.Loop() {
		rt, err := openLogRuntime(dir, storage.DefaultOptions(), defaultRuntimePolicy())
		if err != nil {
			b.Fatal(err)
		}
		_, ordersChannel, err := prepareInRuntime[*OrderPlaced, *Orders](b.Context(), rt, placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < history; i++ {
			receiveAmount(b, ordersChannel, int64(i))
		}
		_ = rt.close()
	}
}

// B-L01: append individual Events through transactions on one Channel. Each
// operation sends 64 changes, one per transaction, and waits until all are
// committed; consecutive sends may share a group commit.
func BenchmarkAppendEvent(b *testing.B) {
	const perOperation = 64
	rt := openBenchmarkRuntime(b)
	event, channel, err := prepareInRuntime[*OrderPlaced, *Orders](b.Context(), rt, placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	var amount int64
	for b.Loop() {
		for i := 0; i < perOperation; i++ {
			amount++
			setAmount(event, amount)
		}
		// Each transaction changes the amount, so each commits one occurrence.
		waitStoreCommitted(b, rt, channel.id(), uint64(amount))
	}
}

// waitStoreCommitted waits until the store has committed want occurrences of
// the Channel, waking on each commit.
func waitStoreCommitted(b *testing.B, rt *logRuntime, channelID identity.UUID, want uint64) {
	b.Helper()
	timeout := time.After(benchmarkWait)
	for {
		changed := rt.store.Changed()
		if rt.store.Committed(channelID) >= want {
			return
		}
		select {
		case <-changed:
		case <-timeout:
			b.Fatalf("committed %d occurrences, want %d", rt.store.Committed(channelID), want)
		}
	}
}

// B-N01: successful live notification delivery, from sending a change until
// its notification is received.
func BenchmarkNotificationDelivery(b *testing.B) {
	rt := openBenchmarkRuntime(b)
	event, channel, err := prepareInRuntime[*OrderPlaced, *Orders](b.Context(), rt, placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	var amount int64
	for b.Loop() {
		amount++
		setAmount(event, amount)
		receiveAmount(b, channel, amount)
	}
}

// B-RT03: eight concurrent PrepareLog calls on one shared runtime, each for
// its own Order on the same Channel. The first round creates the live
// instances; later rounds reuse them.
func BenchmarkConcurrentPrepare(b *testing.B) {
	const callers = 8
	rt := openBenchmarkRuntime(b)
	events := make([]*OrderPlaced, callers)
	channels := make([]*Orders, callers)
	errs := make([]error, callers)
	var first []*OrderPlaced
	b.ReportAllocs()
	for b.Loop() {
		var wg sync.WaitGroup
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go prepareBenchmarkCaller(b, rt, i, &wg, events, channels, errs)
		}
		wg.Wait()
		for i := 0; i < callers; i++ {
			if errs[i] != nil {
				b.Fatal(errs[i])
			}
			if channels[i] != channels[0] {
				b.Fatal("concurrent preparations of one logical Channel returned different Channels")
			}
		}
		if first == nil {
			first = append([]*OrderPlaced(nil), events...)
		}
		for i := 0; i < callers; i++ {
			if events[i] != first[i] {
				b.Fatalf("caller %d: repeated preparation returned a new live Event", i)
			}
		}
	}
}

// prepareBenchmarkCaller prepares caller i's Order and records the result.
func prepareBenchmarkCaller(b *testing.B, rt *logRuntime, i int, wg *sync.WaitGroup, events []*OrderPlaced, channels []*Orders, errs []error) {
	defer wg.Done()
	events[i], channels[i], errs[i] = prepareInRuntime[*OrderPlaced, *Orders](b.Context(), rt, placedFactory(testUUID(byte(10+i))), ordersFactory(testUUID(1)))
}
