package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

// sharedEvents decodes the Event records of a device's active segment on a
// shared log.
func sharedEvents(t *testing.T, l *sharedLog, s *Store) []record.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(l.logDir(s), filepath.FromSlash(s.active.path)))
	if err != nil {
		t.Fatal(err)
	}
	var events []record.Event
	for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		decoded, err := record.Decode(line, s.validator)
		if err != nil {
			t.Fatal(err)
		}
		if event, ok := decoded.(record.Event); ok {
			events = append(events, event)
		}
	}
	return events
}

// D-064: occurrences of other devices that this device delivered are counted
// in the version vectors of its later occurrences of the Channel, its clock
// readings come after theirs even when its wall clock is behind, and both
// survive a restart.
func TestDeliveredOccurrencesAreCounted(t *testing.T) {
	log := newSharedLog(t)
	a := log.device(t, "a", fastStoreOptions())
	appendTestOccurrence(t, a, 1, 10, "a1")
	written := sharedEvents(t, log, a)[0]

	behind := fastStoreOptions()
	behind.Now = func() time.Time { return time.UnixMilli(1000) }
	b := log.device(t, "b", behind)
	b.Delivered(identity.UUID(testUUID(1)), a.device, 1, written.HLC)
	appendTestOccurrence(t, b, 1, 11, "b1")
	appendTestOccurrence(t, b, 2, 12, "other channel")
	events := sharedEvents(t, log, b)
	if want := map[string]uint64{a.device.String(): 1, b.device.String(): 1}; !equalVectors(events[0].Vector, want) {
		t.Fatalf("vector after delivery = %v, want %v", events[0].Vector, want)
	}
	if events[0].HLC.Compare(written.HLC) != 1 {
		t.Fatalf("clock reading %+v does not follow the delivered %+v", events[0].HLC, written.HLC)
	}
	if want := map[string]uint64{b.device.String(): 1}; !equalVectors(events[1].Vector, want) {
		t.Fatalf("vector of another Channel = %v, want %v", events[1].Vector, want)
	}

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	log.restart(t)
	b = log.device(t, "b", behind)
	appendTestOccurrence(t, b, 1, 11, "b2")
	events = sharedEvents(t, log, b)
	if want := map[string]uint64{a.device.String(): 1, b.device.String(): 2}; !equalVectors(events[len(events)-1].Vector, want) {
		t.Fatalf("vector after restart = %v, want %v", events[len(events)-1].Vector, want)
	}
}

func equalVectors(v, u map[string]uint64) bool {
	if len(v) != len(u) {
		return false
	}
	for device, count := range u {
		if v[device] != count {
			return false
		}
	}
	return true
}
