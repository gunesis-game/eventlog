package storage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
)

// Smoke benchmarks for the storage layer (docs/project/benchmarks.md). Each
// checks the correctness of its workload; no numeric targets are approved
// (D-040).

func benchmarkOccurrence(payload string) OccurrenceWrite {
	return OccurrenceWrite{ChannelFields: map[string]interface{}{}, EventID: identity.UUID(testUUID(2)), EventType: testEventType, EventFields: map[string]interface{}{"payload": fieldschema.Value{Type: "string", Value: payload}}}
}

// benchmarkHistory writes size single-occurrence transactions to a new store
// in dir and closes it.
func benchmarkHistory(b *testing.B, dir string, size int) identity.UUID {
	b.Helper()
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
	device := s.Device()
	if err := s.Close(); err != nil {
		b.Fatal(err)
	}
	return device
}

// B-T01: commit single-occurrence transactions (1 KiB payload) to an
// existing Channel. Commits are not synced individually; the store's periodic
// sync is the durability boundary.
func BenchmarkCommitSingleTransaction(b *testing.B) {
	s, err := Open(b.TempDir(), DefaultOptions())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	channel := ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}
	occurrences := []OccurrenceWrite{benchmarkOccurrence(strings.Repeat("x", 1024))}
	b.ReportAllocs()
	b.SetBytes(1024)
	var commits uint64
	for b.Loop() {
		if err := s.AppendTransaction(channel, occurrences); err != nil {
			b.Fatal(err)
		}
		commits++
	}
	if got := s.Committed(channel.ID); got != commits {
		b.Fatalf("committed %d occurrences, want %d", got, commits)
	}
}

// B-M01: load the valid manifest and derived index of a closed store with
// 1,000 committed occurrences.
func BenchmarkLoadMetadata(b *testing.B) {
	const size = 1000
	dir := b.TempDir()
	device := benchmarkHistory(b, dir, size)
	backend, err := filesystem.NewOSBackend(filepath.Join(dir, localLogDir, filepath.FromSlash(deviceLogDir(device))))
	if err != nil {
		b.Fatal(err)
	}
	server, err := filesystem.Serve(backend)
	if err != nil {
		b.Fatal(err)
	}
	defer server.Close()
	files := filesystem.NewClient(server.URL(), server.Token())
	defer files.Close()
	b.ReportAllocs()
	for b.Loop() {
		m, err := readManifest(files)
		if err != nil {
			b.Fatal(err)
		}
		raw, err := files.Read(indexFileName, 0, -1)
		if err != nil {
			b.Fatal(err)
		}
		var index indexFile
		if err := json.Unmarshal(raw, &index); err != nil {
			b.Fatal(err)
		}
		if index.Generation != m.Generation || len(index.Entries) != size {
			b.Fatalf("index generation %d with %d entries, want generation %d with %d", index.Generation, len(index.Entries), m.Generation, size)
		}
	}
}

// B-T03 / B-M06: recover a history of committed transactions at startup.
func BenchmarkRecoverHistory(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("events=%d", size), func(b *testing.B) {
			dir := b.TempDir()
			benchmarkHistory(b, dir, size)
			channelID := identity.UUID(testUUID(1))
			b.ReportAllocs()
			for b.Loop() {
				recovered, err := Open(dir, DefaultOptions())
				if err != nil {
					b.Fatal(err)
				}
				if got := recovered.Committed(channelID); got != uint64(size) {
					b.Fatalf("recovered %d occurrences, want %d", got, size)
				}
				_ = recovered.Close()
			}
		})
	}
}
