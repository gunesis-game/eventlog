package eventlog

import (
	"fmt"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/storage"
)

// Representative smoke benchmarks for the approved catalogue in
// docs/project/benchmarks.md. They establish that the workloads run; they are
// not the full workload/profile protocol and carry no numeric targets (D-040).

// B-R (replay): deliver a committed history through one Channel Reader.
func BenchmarkReplayDelivery(b *testing.B) {
	const history = 1000
	dir := b.TempDir()
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
	for i := 0; i < history; i++ {
		occurrence := storage.OccurrenceWrite{ChannelFields: channelFields, EventID: eventID, EventType: schema.Key(), EventFields: map[string]interface{}{
			"ID": fieldschema.Value{Type: "uuid", Value: orderID.String()}, "Amount": fieldschema.Value{Type: "int64", Value: fmt.Sprint(i)},
			"Status": fieldschema.Value{Type: "string", Value: "placed"}, "Note": fieldschema.Value{Type: "null"},
		}}
		if err := setup.AppendTransaction(channel, []storage.OccurrenceWrite{occurrence}); err != nil {
			b.Fatal(err)
		}
	}
	_ = setup.Close()
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
			<-ordersChannel.Notifications()
		}
		_ = rt.close()
	}
}

// B-L05: end-to-end transactions on one Channel, from send to commit.
func BenchmarkTransactionCommit(b *testing.B) {
	dir := b.TempDir()
	rt, err := openLogRuntime(dir, storage.DefaultOptions(), defaultRuntimePolicy())
	if err != nil {
		b.Fatal(err)
	}
	runtimeDirectories.Store(rt, dir)
	defer rt.close()
	event, _, err := prepareInRuntime[*OrderPlaced, *Orders](b.Context(), rt, placedFactory(testUUID(2)), ordersFactory(testUUID(1)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	n := 0
	for b.Loop() {
		n++
		setAmount(event, int64(n))
	}
	deadline := time.Now().Add(time.Minute)
	for committedCount(rt) < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}
