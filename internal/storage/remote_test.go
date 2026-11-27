package storage

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

// sharedLog serves one directory holding the device state folders of several
// test devices and one shared log folder, as Web storage will, through the real
// file-system server and client.
type sharedLog struct {
	dir     string
	server  *filesystem.Server
	root    *filesystem.Client
	backend *countingBackend
}

// countingBackend records the reads of segment files, to show what a refresh
// reads.
type countingBackend struct {
	filesystem.Backend
	mu    sync.Mutex
	reads []string
}

func (b *countingBackend) Read(p string, offset, length int64) ([]byte, error) {
	if strings.Contains(p, "/segments/") {
		b.mu.Lock()
		b.reads = append(b.reads, fmt.Sprintf("%s@%d", filepath.Base(p), offset))
		b.mu.Unlock()
	}
	return b.Backend.Read(p, offset, length)
}

func (b *countingBackend) takeReads() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	reads := b.reads
	b.reads = nil
	return reads
}

func newSharedLog(t *testing.T) *sharedLog {
	t.Helper()
	l := &sharedLog{dir: t.TempDir()}
	l.restart(t)
	return l
}

// restart serves the directory afresh, so the previous server releases its
// open files, which Windows needs before they are removed.
func (l *sharedLog) restart(t *testing.T) {
	t.Helper()
	if l.server != nil {
		_ = l.server.Close()
	}
	dir := l.dir
	osBackend, err := filesystem.NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	backend := &countingBackend{Backend: osBackend}
	server, err := filesystem.Serve(backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})
	l.server, l.root, l.backend = server, filesystem.NewClient(server.URL(), server.Token()), backend
}

// device opens a device with its own state folder, named name, on the shared
// log.
func (l *sharedLog) device(t *testing.T, name string, options Options) *Store {
	t.Helper()
	s, err := open(l.root.Sub(name), l.root.Sub(localLogDir), options)
	if err != nil {
		t.Fatalf("open device %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

// logDir returns the operating-system path of a device's log folder.
func (l *sharedLog) logDir(s *Store) string {
	return filepath.Join(l.dir, localLogDir, devicesDirName, s.device.String())
}

func remotePayloads(t *testing.T, reader *Store, writer identity.UUID, channel byte, position int) []string {
	t.Helper()
	var payloads []string
	for {
		batch, frames, err := reader.RemoteOccurrences(writer, identity.UUID(testUUID(channel)), position, 2)
		if err != nil {
			t.Fatalf("read device %s: %v", writer, err)
		}
		if len(batch) == 0 {
			return payloads
		}
		for _, frame := range frames {
			start := bytes.Index(frame, []byte(`"payload":{"type":"string","value":"`))
			rest := frame[start+len(`"payload":{"type":"string","value":"`):]
			payloads = append(payloads, string(rest[:bytes.IndexByte(rest, '"')]))
		}
		position += len(batch)
	}
}

// D-073: each device reads the other's log, by Channel and position, sees
// what was appended after a refresh, and finds devices that appear later.
func TestDevicesReadEachOthersLogs(t *testing.T) {
	log := newSharedLog(t)
	a := log.device(t, "a", fastStoreOptions())
	appendTestOccurrence(t, a, 1, 10, "a1")
	if err := a.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(11, "a2"), testOccurrence(12, "a3")}); err != nil {
		t.Fatal(err)
	}
	appendTestOccurrence(t, a, 2, 13, "other channel")

	b := log.device(t, "b", fastStoreOptions())
	if got := b.Devices(); len(got) != 1 || got[0] != a.device {
		t.Fatalf("b reads devices %v, want [%s]", got, a.device)
	}
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 0), ","); got != "a1,a2,a3" {
		t.Fatalf("b read a's channel 1 = %s", got)
	}
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 2), ","); got != "a3" {
		t.Fatalf("b read a's channel 1 after position 2 = %s", got)
	}
	if missingFrom, cause := b.RemoteHistory(a.device); cause != nil {
		t.Fatalf("a's history is readable, got missing from %d: %v", missingFrom, cause)
	}

	appendTestOccurrence(t, a, 1, 10, "a4")
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 3), ","); got != "" {
		t.Fatalf("b saw %s before refreshing", got)
	}
	if err := b.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 3), ","); got != "a4" {
		t.Fatalf("b read %s after refreshing, want a4", got)
	}

	appendTestOccurrence(t, b, 1, 20, "b1")
	if len(a.Devices()) != 0 {
		t.Fatal("a knew b before refreshing")
	}
	if err := a.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(remotePayloads(t, a, b.device, 1, 0), ","); got != "b1" {
		t.Fatalf("a read b's channel 1 = %s", got)
	}
}

// A refresh reads only the bytes appended since the last read, continuing
// into segments the other device has rotated to.
func TestRefreshReadsOnlyNewBytes(t *testing.T) {
	log := newSharedLog(t)
	a := log.device(t, "a", smallSegmentOptions())
	appendTestOccurrence(t, a, 1, 10, "a1")
	b := log.device(t, "b", fastStoreOptions())
	size := a.active.size
	log.backend.takeReads()

	appendTestOccurrence(t, a, 1, 10, "a2")
	if err := b.Refresh(); err != nil {
		t.Fatal(err)
	}
	reads := log.backend.takeReads()
	if want := fmt.Sprintf("segment-000001.ndjson@%d", size); len(reads) == 0 || reads[0] != want {
		t.Fatalf("refresh read %v, want to start with %s", reads, want)
	}
	for _, read := range reads[1:] {
		if !strings.HasSuffix(read, "@0") || strings.HasPrefix(read, "segment-000001") {
			t.Fatalf("refresh read %v: only new segments may be read from their start", reads)
		}
	}

	for i := 0; i < 6; i++ {
		appendTestOccurrence(t, a, 1, 10, fmt.Sprintf("r%d", i))
	}
	if len(a.manifest.Segments) < 2 {
		t.Fatalf("a did not rotate: %v", a.manifest.Segments)
	}
	if err := b.Refresh(); err != nil {
		t.Fatal(err)
	}
	for _, read := range log.backend.takeReads() {
		if strings.HasPrefix(read, "segment-000001.ndjson@0") {
			t.Fatalf("refresh read the whole first segment again: %s", read)
		}
	}
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 0), ","); got != "a1,a2,r0,r1,r2,r3,r4,r5" {
		t.Fatalf("b read %s", got)
	}
}

// Positions stay valid when the other device compacts its log: the files
// read before are gone, and the log is read again from its manifest.
func TestRemotePositionsSurviveCompaction(t *testing.T) {
	log := newSharedLog(t)
	options := smallSegmentOptions()
	options.CompactionThreshold = 2
	a := log.device(t, "a", options)
	for i := 0; i < 3; i++ {
		appendTestOccurrence(t, a, 1, 10, fmt.Sprintf("p%d", i))
	}
	b := log.device(t, "b", fastStoreOptions())
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 0), ","); got != "p0,p1,p2" {
		t.Fatalf("b read %s", got)
	}
	generation := a.manifest.Generation
	for i := 3; i < 12; i++ {
		appendTestOccurrence(t, a, 1, 10, fmt.Sprintf("p%d", i))
	}
	if a.manifest.Generation == generation {
		t.Fatal("a did not compact")
	}
	want := "p1,p2,p3,p4,p5,p6,p7,p8,p9,p10,p11"
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 1), ","); got != want {
		t.Fatalf("b read %s after a compacted, want %s: a file gone since the last read makes b read the log again", got, want)
	}
	if err := b.Refresh(); err != nil {
		t.Fatal(err)
	}
	want = "p2,p3,p4,p5,p6,p7,p8,p9,p10,p11"
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 2), ","); got != want {
		t.Fatalf("b read %s, want %s", got, want)
	}
}

// An incomplete transaction at the end of another device's log is not yet
// committed, and its file is left untouched.
func TestRemoteIncompleteTransactionIsNotCommitted(t *testing.T) {
	log := newSharedLog(t)
	a := log.device(t, "a", fastStoreOptions())
	appendTestOccurrence(t, a, 1, 10, "committed")
	path := filepath.Join(log.logDir(a), filepath.FromSlash(a.active.path))
	committed, _ := os.ReadFile(path)
	appendTestOccurrence(t, a, 1, 10, "in flight")
	full, _ := os.ReadFile(path)
	partial := full[:len(committed)+(len(full)-len(committed))/2]
	if err := os.WriteFile(path, partial, 0o600); err != nil {
		t.Fatal(err)
	}

	b := log.device(t, "b", fastStoreOptions())
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 0), ","); got != "committed" {
		t.Fatalf("b read %s", got)
	}
	if _, cause := b.RemoteHistory(a.device); cause != nil {
		t.Fatalf("an incomplete tail is not damage: %v", cause)
	}
	if data, _ := os.ReadFile(path); !bytes.Equal(data, partial) {
		t.Fatal("b changed a's segment")
	}
	if err := os.WriteFile(path, full, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := b.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(remotePayloads(t, b, a.device, 1, 0), ","); got != "committed,in flight" {
		t.Fatalf("b read %s once the transaction completed", got)
	}
}

// D-073: damage in another device's log never stops a device from opening.
// The readable prefix is kept, the rest is missing from the damaged
// transaction on, and a refresh that finds the log readable again restores it.
func TestRemoteDamageKeepsReadablePrefix(t *testing.T) {
	// Lines of a's segment: Channel, Event "kept", commit; Event "damaged",
	// commit; Event "after", commit. The damaged transaction starts at
	// sequence 4.
	cases := map[string]func(t *testing.T, path string, lines [][]byte){
		"malformed record": func(t *testing.T, path string, lines [][]byte) {
			lines[3] = []byte(`{"kind":"event","not":"valid"}`)
			writeLines(t, path, lines)
		},
		"record of another device": func(t *testing.T, path string, lines [][]byte) {
			validator, err := record.NewValidator()
			if err != nil {
				t.Fatal(err)
			}
			other := identity.NewRandom().String()
			rewriteEvents(t, path, validator, func(i int, event *record.Event) {
				if i == 1 {
					event.DeviceID, event.Vector = other, map[string]uint64{other: 1}
				}
			})
		},
		"missing segment": func(t *testing.T, path string, lines [][]byte) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, damage := range cases {
		t.Run(name, func(t *testing.T) {
			log := newSharedLog(t)
			a := log.device(t, "a", fastStoreOptions())
			appendTestOccurrence(t, a, 1, 10, "kept")
			appendTestOccurrence(t, a, 1, 10, "damaged")
			appendTestOccurrence(t, a, 1, 10, "after")
			path := filepath.Join(log.logDir(a), filepath.FromSlash(a.active.path))
			original, _ := os.ReadFile(path)
			_ = a.Close()
			log.restart(t)
			damage(t, path, bytes.Split(bytes.TrimSuffix(original, []byte("\n")), []byte("\n")))

			b := log.device(t, "b", fastStoreOptions())
			missingFrom, cause := b.RemoteHistory(a.device)
			if cause == nil {
				t.Fatal("damage was not detected")
			}
			readable := strings.Join(remotePayloads(t, b, a.device, 1, 0), ",")
			if name == "missing segment" {
				if readable != "" || missingFrom != 1 {
					t.Fatalf("read %q, missing from %d", readable, missingFrom)
				}
			} else if readable != "kept" || missingFrom != 4 {
				t.Fatalf("read %q, missing from %d, want kept and 4 (%v)", readable, missingFrom, cause)
			}

			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := b.Refresh(); err != nil {
				t.Fatal(err)
			}
			if _, cause := b.RemoteHistory(a.device); cause != nil {
				t.Fatalf("restored log still damaged: %v", cause)
			}
			if got := strings.Join(remotePayloads(t, b, a.device, 1, 0), ","); got != "kept,damaged,after" {
				t.Fatalf("read %s after restoring", got)
			}
		})
	}
}

func writeLines(t *testing.T, path string, lines [][]byte) {
	t.Helper()
	if err := os.WriteFile(path, append(bytes.Join(lines, []byte("\n")), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A device that may not write, and so has no log of its own, still reads every
// other device's log.
func TestReadOnlyDeviceReadsOtherDevices(t *testing.T) {
	dir := t.TempDir()
	writer := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, writer, 1, 10, "w1")
	writerDevice := writer.device
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, localDeviceStateDir)); err != nil {
		t.Fatal(err)
	}

	reader, _ := openRefusingStore(t, dir, true)
	expectPermission(t, reader.WriteRefusal())
	if got := reader.Devices(); len(got) != 1 || got[0] != writerDevice {
		t.Fatalf("read-only device reads %v", got)
	}
	if got := strings.Join(remotePayloads(t, reader, writerDevice, 1, 0), ","); got != "w1" {
		t.Fatalf("read-only device read %s", got)
	}
}
