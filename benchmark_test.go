package eventlog

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Representative smoke benchmarks for the approved catalogue in
// docs/project/benchmarks.md. They establish that the workloads run; they are
// not the full workload/profile protocol and carry no numeric targets (D-040).

func benchmarkEventFields(payload string) map[string]interface{} {
	return map[string]interface{}{"payload": fieldValue{Type: "string", Value: payload}}
}

// B-F01: encode and schema-validate one 1 KiB Event record.
func BenchmarkEncodeEventRecord(b *testing.B) {
	validator, err := newRecordSchemaValidator()
	if err != nil {
		b.Fatal(err)
	}
	record := eventRecord{SchemaVersion: 1, Sequence: 1, Kind: recordKindEvent, ID: uuid(testUUID(1)).String(), ChannelID: uuid(testUUID(2)).String(), Type: recordType{PackagePath: "example.org/bench", Name: "Event"}, Fields: benchmarkEventFields(strings.Repeat("x", 1024))}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := encodeRecord(recordKindEvent, record, validator); err != nil {
			b.Fatal(err)
		}
	}
}

// B-L01: append individual 1 KiB Event occurrences to an existing Channel.
func BenchmarkAppendOccurrence(b *testing.B) {
	s, err := openSegmentStore(b.TempDir(), defaultSegmentStoreOptions())
	if err != nil {
		b.Fatal(err)
	}
	defer s.close()
	channel := channelWrite{id: uuid(testUUID(1)), typ: testChannelType}
	occurrences := []occurrenceWrite{{channelFields: map[string]interface{}{}, eventID: uuid(testUUID(2)), eventType: testEventType, eventFields: benchmarkEventFields(strings.Repeat("x", 1024))}}
	b.ReportAllocs()
	b.SetBytes(1024)
	for b.Loop() {
		if err := s.appendTransaction(channel, occurrences); err != nil {
			b.Fatal(err)
		}
	}
}

// B-T03 / B-M06: recover a history of committed transactions at startup.
func BenchmarkRecoverHistory(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("events=%d", size), func(b *testing.B) {
			dir := b.TempDir()
			s, err := openSegmentStore(dir, defaultSegmentStoreOptions())
			if err != nil {
				b.Fatal(err)
			}
			channel := channelWrite{id: uuid(testUUID(1)), typ: testChannelType}
			for i := 0; i < size; i++ {
				occurrence := occurrenceWrite{channelFields: map[string]interface{}{}, eventID: uuid(testUUID(2)), eventType: testEventType, eventFields: benchmarkEventFields(fmt.Sprint(i))}
				if err := s.appendTransaction(channel, []occurrenceWrite{occurrence}); err != nil {
					b.Fatal(err)
				}
			}
			_ = s.close()
			b.ReportAllocs()
			for b.Loop() {
				recovered, err := openSegmentStore(dir, defaultSegmentStoreOptions())
				if err != nil {
					b.Fatal(err)
				}
				_ = recovered.close()
			}
		})
	}
}

// B-R (replay): deliver a committed history through one Channel Reader.
func BenchmarkReplayDelivery(b *testing.B) {
	const history = 1000
	dir := b.TempDir()
	setup, err := openLogRuntime(dir, defaultSegmentStoreOptions(), defaultRuntimePolicy())
	if err != nil {
		b.Fatal(err)
	}
	schema, _ := resolveTypeSchema(&OrderPlaced{})
	channelSchema, _ := resolveTypeSchema(&Orders{})
	channelID, _ := deriveIdentity(channelIdentityDomain, channelSchema, uuid(testUUID(1)))
	orderID := uuid(testUUID(2))
	eventID, _ := deriveIdentity(eventIdentityDomain, schema, orderID)
	channel := channelWrite{id: channelID, typ: channelSchema.key()}
	channelFields := map[string]interface{}{"ID": fieldValue{Type: "uuid", Value: uuid(testUUID(1)).String()}, "Region": fieldValue{Type: "string", Value: "eu"}}
	for i := 0; i < history; i++ {
		occurrence := occurrenceWrite{channelFields: channelFields, eventID: eventID, eventType: schema.key(), eventFields: map[string]interface{}{
			"ID": fieldValue{Type: "uuid", Value: orderID.String()}, "Amount": fieldValue{Type: "int64", Value: fmt.Sprint(i)},
			"Status": fieldValue{Type: "string", Value: "placed"}, "Note": fieldValue{Type: "null"},
		}}
		if err := setup.store.appendTransaction(channel, []occurrenceWrite{occurrence}); err != nil {
			b.Fatal(err)
		}
	}
	_ = setup.close()
	b.ReportAllocs()
	for b.Loop() {
		rt, err := openLogRuntime(dir, defaultSegmentStoreOptions(), defaultRuntimePolicy())
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
	rt, err := openLogRuntime(b.TempDir(), defaultSegmentStoreOptions(), defaultRuntimePolicy())
	if err != nil {
		b.Fatal(err)
	}
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
