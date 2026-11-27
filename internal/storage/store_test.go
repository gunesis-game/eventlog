package storage

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
)

var testEventType = fieldschema.TypeKey{PackagePath: "example.org/test", Name: "Event"}
var testChannelType = fieldschema.TypeKey{PackagePath: "example.org/test", Name: "Channel"}

func openTestStore(t *testing.T, dir string, options Options) *Store {
	t.Helper()
	s, err := Open(dir, options)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

func testOccurrence(event byte, payload string) OccurrenceWrite {
	return OccurrenceWrite{
		ChannelFields: map[string]interface{}{"region": fieldschema.Value{Type: "string", Value: "eu"}},
		EventID:       identity.UUID(testUUID(event)),
		EventType:     testEventType,
		EventFields:   map[string]interface{}{"payload": fieldschema.Value{Type: "string", Value: payload}},
	}
}

// appendTestOccurrence appends a one-occurrence transaction and returns the
// Event record's global sequence.
func appendTestOccurrence(t *testing.T, s *Store, channel byte, event byte, payload string) uint64 {
	t.Helper()
	if err := s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(channel)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(event, payload)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index.occurrences[len(s.index.occurrences)-1].Sequence
}

// storeSummary captures recovered state for equality comparisons.
func storeSummary(s *Store) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "next=%d gen=%d segs=%v\n", s.nextSequence, s.manifest.Generation, s.manifest.Segments)
	for _, e := range s.index.occurrences {
		fmt.Fprintf(&b, "%d %s %s %s\n", e.Sequence, e.EventID, e.ChannelID, e.Type)
	}
	return b.String()
}

func activeSegmentPath(s *Store) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active.path
}

// L-01, L-03, L-07, T-02, T-04, T-05, L-12, M-01: appends commit atomically
// with a new Channel record, sequences are global, and restart recovers them.
func TestSegmentStoreAppendAndRecover(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	if got := appendTestOccurrence(t, s, 1, 10, "a"); got != 2 {
		t.Fatalf("first event sequence = %d, want 2 (after its Channel record)", got)
	}
	if got := appendTestOccurrence(t, s, 1, 10, "b"); got != 4 {
		t.Fatalf("second event sequence = %d, want 4 (existing Channel: Event, Commit)", got)
	}
	if got := appendTestOccurrence(t, s, 2, 11, "c"); got != 7 {
		t.Fatalf("third event sequence = %d, want 7 (new Channel record first)", got)
	}
	before := storeSummary(s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, dir, fastStoreOptions())
	if after := storeSummary(reopened); after != before {
		t.Fatalf("recovered state differs:\n%s\nvs\n%s", after, before)
	}
	frame, err := reopened.ReadOccurrenceFrame(identity.UUID(testUUID(1)), 1)
	if err != nil || !bytes.Contains(frame, []byte(`"value":"b"`)) {
		t.Fatalf("read occurrence = %s, %v", frame, err)
	}
	if got := appendTestOccurrence(t, reopened, 1, 10, "d"); got != 9 {
		t.Fatalf("sequence continues after restart: got %d, want 9", got)
	}
}

// L-02, L-04: one invalid occurrence fails the whole transaction before
// anything is written.
func TestSegmentStoreValidationFailureWritesNothing(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	good := testOccurrence(2, "ok")
	bad := testOccurrence(3, "bad")
	bad.EventFields = map[string]interface{}{"bad": fieldschema.Value{Type: "int8", Value: "not-a-number"}}
	if err := s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{good, bad}); err == nil {
		t.Fatal("expected validation failure")
	}
	info, err := os.Stat(activeSegmentPath(s))
	if err != nil || info.Size() != 0 {
		t.Fatalf("segment size after failed append = %v, %v", info.Size(), err)
	}
	if got := appendTestOccurrence(t, s, 1, 2, "ok"); got != 2 {
		t.Fatalf("sequence after failed append = %d, want 2", got)
	}
}

// T-01, T-03, T-06, M-06, RT-08: an incomplete trailing transaction is
// discarded from its beginning, repeatedly and identically.
func TestRecoveryTruncatesIncompleteTrailingTransaction(t *testing.T) {
	for name, tail := range map[string]func(transaction []byte) []byte{
		"partial frame":       func(tx []byte) []byte { return tx[:len(tx)/3] },
		"records, no commit":  func(tx []byte) []byte { return tx[:bytes.LastIndexByte(tx[:len(tx)-1], '\n')+1] },
		"partial commit line": func(tx []byte) []byte { return tx[:len(tx)-5] },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s := openTestStore(t, dir, fastStoreOptions())
			appendTestOccurrence(t, s, 1, 10, "committed")
			path := activeSegmentPath(s)
			committed, _ := os.ReadFile(path)
			appendTestOccurrence(t, s, 2, 11, "interrupted")
			full, _ := os.ReadFile(path)
			_ = s.Close()
			interrupted := append(append([]byte(nil), committed...), tail(full[len(committed):])...)
			if err := os.WriteFile(path, interrupted, 0o600); err != nil {
				t.Fatal(err)
			}
			first := openTestStore(t, dir, fastStoreOptions())
			summary := storeSummary(first)
			if len(first.index.occurrences) != 1 {
				t.Fatalf("recovered entries:\n%s", summary)
			}
			_ = first.Close()
			data, _ := os.ReadFile(path)
			if !bytes.Equal(data, committed) {
				t.Fatal("incomplete transaction was not truncated to its start")
			}
			second := openTestStore(t, dir, fastStoreOptions())
			if storeSummary(second) != summary {
				t.Fatal("repeated recovery is not idempotent")
			}
		})
	}
}

// F-06, M-08: complete malformed or corrupt records fail closed.
func TestRecoveryFailsClosedOnCorruption(t *testing.T) {
	cases := map[string]func(data []byte) []byte{
		"malformed complete record": func(d []byte) []byte { return append(d, []byte("{not json}\n")...) },
		"checksum mismatch":         func(d []byte) []byte { return bytes.Replace(d, []byte(`"value":"a"`), []byte(`"value":"z"`), 1) },
		"invalid UTF-8":             func(d []byte) []byte { return bytes.Replace(d, []byte(`"value":"a"`), []byte("\"value\":\"\xff\""), 1) },
		"unsupported version": func(d []byte) []byte {
			return bytes.Replace(d, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1)
		},
		"record without kind": func(d []byte) []byte { return append(d, []byte("{}\n")...) },
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s := openTestStore(t, dir, fastStoreOptions())
			appendTestOccurrence(t, s, 1, 10, "a")
			path := activeSegmentPath(s)
			_ = s.Close()
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(path, corrupt(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if store, err := Open(dir, fastStoreOptions()); err == nil {
				_ = store.Close()
				t.Fatal("expected recovery to fail closed")
			} else if !errors.Is(err, errCorruptStorage) {
				t.Fatalf("error %v is not a corruption error", err)
			}
		})
	}
}

// M-02, M-03, M-04, M-05, M-07, M-09: the derived index is rebuilt from the
// authoritative segments whenever it is missing, stale, or wrong.
func TestRecoveryRebuildsDerivedIndex(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, s, 1, 10, "a")
	appendTestOccurrence(t, s, 1, 10, "b")
	_ = s.Close()
	indexPath := filepath.Join(dir, indexFileName)
	good, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	clean := openTestStore(t, dir, fastStoreOptions())
	if clean.indexRebuilt {
		t.Fatal("a consistent index must not be rebuilt")
	}
	_ = clean.Close()

	for name, damage := range map[string]func(){
		"missing": func() { _ = os.Remove(indexPath) },
		"inconsistent": func() {
			_ = os.WriteFile(indexPath, bytes.Replace(good, []byte(`"sequence":2`), []byte(`"sequence":7`), 1), 0o600)
		},
		"stale": func() {
			_ = os.WriteFile(indexPath, []byte(`{"format_version":1,"generation":1,"committed_sequence":0,"entries":[]}`+"\n"), 0o600)
		},
		"garbage": func() { _ = os.WriteFile(indexPath, []byte("garbage"), 0o600) },
	} {
		t.Run(name, func(t *testing.T) {
			damage()
			rebuilt := openTestStore(t, dir, fastStoreOptions())
			if !rebuilt.indexRebuilt {
				t.Fatal("expected index rebuild")
			}
			_ = rebuilt.Close()
			after, _ := os.ReadFile(indexPath)
			if !bytes.Equal(after, good) {
				t.Fatalf("rebuilt index differs:\n%s\nvs\n%s", after, good)
			}
		})
	}
}

// D-08: missing storage is created on open; manifest and first segment exist.
func TestOpenInitializesMissingStorage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "log")
	s := openTestStore(t, dir, fastStoreOptions())
	for _, path := range []string{filepath.Join(dir, manifestFileName), segmentPath(filepath.Join(dir, segmentsDirName), 1)} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	if s.manifest.Generation != 1 || s.manifest.NextSegmentID != 2 {
		t.Fatalf("manifest = %+v", s.manifest)
	}
}

func TestStorageLockIsExclusive(t *testing.T) {
	dir := t.TempDir()
	openTestStore(t, dir, fastStoreOptions())
	if second, err := Open(dir, fastStoreOptions()); err == nil {
		_ = second.Close()
		t.Fatal("a second owner must be rejected")
	}
}

func smallSegmentOptions() Options {
	options := fastStoreOptions()
	options.TargetSegmentSize = 1
	options.CompactionThreshold = 1 << 30
	return options
}

// Segments rotate at transaction boundaries and sequences stay global.
func TestSegmentRotationKeepsGlobalSequence(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, smallSegmentOptions())
	for i := 0; i < 4; i++ {
		appendTestOccurrence(t, s, 1, 10, fmt.Sprint(i))
	}
	if len(s.manifest.Segments) != 4 {
		t.Fatalf("segments = %v", s.manifest.Segments)
	}
	before := storeSummary(s)
	_ = s.Close()
	reopened := openTestStore(t, dir, smallSegmentOptions())
	if storeSummary(reopened) != before {
		t.Fatal("rotated segments did not recover identically")
	}
}

// A missing manifest is rebuilt only from an unambiguous segment chain.
func TestMissingManifestRecovery(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, smallSegmentOptions())
	for i := 0; i < 3; i++ {
		appendTestOccurrence(t, s, 1, 10, fmt.Sprint(i))
	}
	before := storeSummary(s)
	_ = s.Close()
	_ = os.Remove(filepath.Join(dir, manifestFileName))
	reopened := openTestStore(t, dir, smallSegmentOptions())
	if storeSummary(reopened) != before {
		t.Fatal("manifest rebuild changed recovered history")
	}
	_ = reopened.Close()

	// Add an overlapping copy, as an interrupted compaction would leave.
	segments := filepath.Join(dir, segmentsDirName)
	first, _ := os.ReadFile(segmentPath(segments, 1))
	if err := os.WriteFile(segmentPath(segments, 99), first, 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, manifestFileName))
	if store, err := Open(dir, smallSegmentOptions()); err == nil {
		_ = store.Close()
		t.Fatal("ambiguous segments without a manifest must fail closed")
	}
}

// Compaction consolidates finalized segments, preserves every record and
// sequence byte-for-byte, and survives restart.
func TestCompactionPreservesHistory(t *testing.T) {
	dir := t.TempDir()
	options := fastStoreOptions()
	options.TargetSegmentSize = 1
	options.CompactionThreshold = 3
	s := openTestStore(t, dir, options)
	var concatenated []byte
	for i := 0; i < 7; i++ {
		appendTestOccurrence(t, s, 1, 10, fmt.Sprint(i))
	}
	s.mu.Lock()
	generation := s.manifest.Generation
	finalized := len(s.finalized)
	s.mu.Unlock()
	if generation < 2 || finalized >= options.CompactionThreshold {
		t.Fatalf("expected compaction: generation=%d finalized=%d", generation, finalized)
	}
	for i := 0; i < 7; i++ {
		frame, err := s.ReadOccurrenceFrame(identity.UUID(testUUID(1)), i)
		if err != nil || !bytes.Contains(frame, []byte(fmt.Sprintf(`"value":"%d"`, i))) {
			t.Fatalf("occurrence %d after compaction: %s, %v", i, frame, err)
		}
		concatenated = append(concatenated, frame...)
	}
	before := storeSummary(s)
	_ = s.Close()
	ids, _ := (&recovery{segmentsDir: filepath.Join(dir, segmentsDirName)}).listSegmentIDs()
	m, _ := readManifest(dir)
	if len(ids) != len(m.Segments) {
		t.Fatalf("old generation files were not removed: files=%v manifest=%v", ids, m.Segments)
	}
	reopened := openTestStore(t, dir, options)
	if after := storeSummary(reopened); after != before {
		t.Fatalf("compacted history recovered differently:\n%s\nvs\n%s", after, before)
	}
}

// Readers pin segments: a retired segment is deleted only after release.
func TestCompactionRespectsReaderPins(t *testing.T) {
	dir := t.TempDir()
	options := fastStoreOptions()
	options.TargetSegmentSize = 1
	options.CompactionThreshold = 1 << 30
	s := openTestStore(t, dir, options)
	for i := 0; i < 3; i++ {
		appendTestOccurrence(t, s, 1, 10, fmt.Sprint(i))
	}
	s.mu.Lock()
	pinned := s.finalized[0]
	pinned.refs++
	s.options.CompactionThreshold = 1
	if err := s.rotateLocked(); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.mu.Unlock()
	if _, err := os.Stat(pinned.path); err != nil {
		t.Fatal("pinned segment must survive compaction")
	}
	s.unpin(pinned)
	if _, err := os.Stat(pinned.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retired segment must be deleted after its last pin is released")
	}
}

// Orphans and temporary files are cleaned after validation.
func TestRecoveryCleansOrphans(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, s, 1, 10, "a")
	_ = s.Close()
	segments := filepath.Join(dir, segmentsDirName)
	orphans := []string{segmentPath(segments, 50), segmentPath(segments, 51) + temporaryFileSuffix, filepath.Join(dir, manifestFileName+temporaryFileSuffix)}
	for _, path := range orphans {
		_ = os.WriteFile(path, []byte("x"), 0o600)
	}
	openTestStore(t, dir, fastStoreOptions())
	for _, path := range orphans {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("orphan %s was not cleaned", path)
		}
	}
}

// L-11: concurrent appends keep transactions atomic and sequences gap-free.
func TestConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				occurrence := OccurrenceWrite{ChannelFields: map[string]interface{}{}, EventID: identity.UUID(testUUID(byte(worker + 100))), EventType: testEventType, EventFields: map[string]interface{}{"i": fieldschema.Value{Type: "int", Value: fmt.Sprint(i)}}}
				if err := s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(byte(worker + 1))), Type: testChannelType}, []OccurrenceWrite{occurrence}); err != nil {
					t.Error(err)
				}
			}
		}(worker)
	}
	wg.Wait()
	_ = s.Close()
	reopened := openTestStore(t, dir, fastStoreOptions())
	if len(reopened.index.occurrences) != 200 {
		t.Fatalf("recovered %d occurrences, want 200", len(reopened.index.occurrences))
	}
	for channel, entries := range reopened.index.byChannel {
		for i := 1; i < len(entries); i++ {
			if entries[i].Sequence <= entries[i-1].Sequence {
				t.Fatalf("channel %s out of order", channel)
			}
		}
	}
}

// A multi-occurrence transaction recovers with every occurrence, and a Channel
// record is written only where the Channel snapshot changed.
func TestMultiOccurrenceTransactionRecovers(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	changed := testOccurrence(11, "c")
	changed.ChannelFields = map[string]interface{}{"region": fieldschema.Value{Type: "string", Value: "us"}}
	if err := s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(10, "a"), testOccurrence(10, "b"), changed}); err != nil {
		t.Fatal(err)
	}
	// Channel, Event, Event, Channel, Event, Commit
	if s.nextSequence != 7 {
		t.Fatalf("next sequence = %d, want 7", s.nextSequence)
	}
	before := storeSummary(s)
	_ = s.Close()
	reopened := openTestStore(t, dir, fastStoreOptions())
	if storeSummary(reopened) != before {
		t.Fatal("multi-occurrence transaction did not recover identically")
	}
	if fields, _ := persistedChannelFields(reopened, identity.UUID(testUUID(1))); !strings.Contains(fields, `"us"`) {
		t.Fatalf("latest Channel snapshot = %s", fields)
	}
}

func testUUID(n byte) [16]byte {
	var u [16]byte
	for i := range u {
		u[i] = n
	}
	return u
}

func fastStoreOptions() Options {
	options := DefaultOptions()
	options.SyncInterval = 20 * time.Millisecond
	return options
}

// persistedChannelFields returns the latest persisted Channel fields JSON.
func persistedChannelFields(s *Store, channelID identity.UUID) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.index.channel(channelID)
	return channel.fields, ok
}
