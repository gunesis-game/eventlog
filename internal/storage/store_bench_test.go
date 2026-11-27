package storage

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
)

// Smoke benchmarks for the storage layer; no numeric targets are approved
// (D-040).

func benchmarkOccurrence(payload string) OccurrenceWrite {
	return OccurrenceWrite{ChannelFields: map[string]interface{}{}, EventID: identity.UUID(testUUID(2)), EventType: testEventType, EventFields: map[string]interface{}{"payload": fieldschema.Value{Type: "string", Value: payload}}}
}

// B-L01: append individual 1 KiB Event occurrences to an existing Channel.
func BenchmarkAppendTransaction(b *testing.B) {
	s, err := Open(b.TempDir(), DefaultOptions())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	channel := ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}
	occurrences := []OccurrenceWrite{benchmarkOccurrence(strings.Repeat("x", 1024))}
	b.ReportAllocs()
	b.SetBytes(1024)
	for b.Loop() {
		if err := s.AppendTransaction(channel, occurrences); err != nil {
			b.Fatal(err)
		}
	}
}

// B-T03 / B-M06: recover a history of committed transactions at startup.
func BenchmarkRecoverHistory(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("events=%d", size), func(b *testing.B) {
			dir := b.TempDir()
			s, err := Open(dir, DefaultOptions())
			if err != nil {
				b.Fatal(err)
			}
			channel := ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}
			for i := 0; i < size; i++ {
				if err := s.AppendTransaction(channel, []OccurrenceWrite{benchmarkOccurrence(fmt.Sprint(i))}); err != nil {
					b.Fatal(err)
				}
			}
			_ = s.Close()
			b.ReportAllocs()
			for b.Loop() {
				recovered, err := Open(dir, DefaultOptions())
				if err != nil {
					b.Fatal(err)
				}
				_ = recovered.Close()
			}
		})
	}
}
