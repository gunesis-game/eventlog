package storage

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/causal"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

// persistedEvents decodes the Event records of the device's active segment.
func persistedEvents(t *testing.T, dir string, s *Store) []record.Event {
	t.Helper()
	data, err := os.ReadFile(diskPath(dir, activeSegmentPath(s)))
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

func deviceOf(t *testing.T, dir string) string {
	t.Helper()
	return readIdentityFromDisk(t, diskPath(dir, "../../../device/device.json"), "device_id").String()
}

// D-064: every Event occurrence names the writing device, carries the
// Channel's version vector, counted per Channel, and a strictly increasing
// hybrid logical clock reading.
func TestEventsCarryCausalMetadata(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, s, 1, 10, "a")
	if err := s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(11, "b"), testOccurrence(12, "c")}); err != nil {
		t.Fatal(err)
	}
	appendTestOccurrence(t, s, 2, 13, "d")
	appendTestOccurrence(t, s, 1, 10, "e")

	device := deviceOf(t, dir)
	events := persistedEvents(t, dir, s)
	want := []struct {
		channel byte
		count   uint64
	}{{1, 1}, {1, 2}, {1, 3}, {2, 1}, {1, 4}}
	if len(events) != len(want) {
		t.Fatalf("%d events, want %d", len(events), len(want))
	}
	var last causal.Timestamp
	for i, event := range events {
		if event.DeviceID != device {
			t.Fatalf("event %d device = %s, want %s", i, event.DeviceID, device)
		}
		if event.ChannelID != identity.UUID(testUUID(want[i].channel)).String() || len(event.Vector) != 1 || event.Vector[device] != want[i].count {
			t.Fatalf("event %d on channel %s has vector %v, want {%s: %d}", i, event.ChannelID, event.Vector, device, want[i].count)
		}
		if event.HLC.Compare(last) != 1 {
			t.Fatalf("event %d clock %+v does not follow %+v", i, event.HLC, last)
		}
		last = event.HLC
	}
}

// After a restart the vector counts continue per Channel and the clock keeps
// increasing, even when the wall clock has been set back.
func TestCausalStateSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	base := time.UnixMilli(1_800_000_000_000)
	options := fastStoreOptions()
	options.Now = func() time.Time { return base }
	s := openTestStore(t, dir, options)
	appendTestOccurrence(t, s, 1, 10, "a")
	appendTestOccurrence(t, s, 1, 10, "b")
	before := persistedEvents(t, dir, s)
	_ = s.Close()

	options.Now = func() time.Time { return base.Add(-time.Hour) }
	reopened := openTestStore(t, dir, options)
	appendTestOccurrence(t, reopened, 1, 10, "c")
	after := persistedEvents(t, dir, reopened)
	device := deviceOf(t, dir)
	latest := after[len(after)-1]
	if latest.Vector[device] != 3 {
		t.Fatalf("vector after restart = %v, want count 3", latest.Vector)
	}
	previous := before[len(before)-1].HLC
	if latest.HLC.Compare(previous) != 1 || latest.HLC.WallMS != base.UnixMilli() {
		t.Fatalf("clock after restart = %+v, want after %+v at the earlier wall time", latest.HLC, previous)
	}
}

// rewriteEvents rewrites the Event records of a segment through change and
// recomputes each commit's checksum, so only the causal fields are wrong.
func rewriteEvents(t *testing.T, path string, validator *record.Validator, change func(i int, event *record.Event)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	var frames [][]byte
	eventIndex := 0
	for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		decoded, err := record.Decode(line, validator)
		if err != nil {
			t.Fatal(err)
		}
		var frame []byte
		switch r := decoded.(type) {
		case record.Event:
			change(eventIndex, &r)
			eventIndex++
			frame, err = record.Encode(record.KindEvent, r, validator)
			frames = append(frames, frame)
		case record.Channel:
			frame, err = record.Encode(record.KindChannel, r, validator)
			frames = append(frames, frame)
		case record.Commit:
			r.Checksum = record.Checksum(frames)
			frames = nil
			frame, err = record.Encode(record.KindCommit, r, validator)
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, frame...)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Recovery fails closed when this device's own occurrences carry causal
// metadata it cannot have written.
func TestRecoveryRejectsInconsistentCausalMetadata(t *testing.T) {
	other := identity.NewRandom().String()
	cases := map[string]func(i int, event *record.Event){
		"another device": func(i int, event *record.Event) {
			if i == 1 {
				event.DeviceID = other
				event.Vector = map[string]uint64{other: 2}
			}
		},
		"count skips": func(i int, event *record.Event) {
			if i == 1 {
				event.Vector[event.DeviceID] = 3
			}
		},
		"count repeats": func(i int, event *record.Event) {
			if i == 1 {
				event.Vector[event.DeviceID] = 1
			}
		},
		"clock goes back": func(i int, event *record.Event) {
			if i == 1 {
				event.HLC = causal.Timestamp{WallMS: 1}
			}
		},
		"clock repeats": func(i int, event *record.Event) {
			if i == 1 {
				event.HLC = causal.Timestamp{WallMS: 1_800_000_000_000}
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			options := fastStoreOptions()
			options.Now = func() time.Time { return time.UnixMilli(1_800_000_000_000) }
			s := openTestStore(t, dir, options)
			appendTestOccurrence(t, s, 1, 10, "a")
			appendTestOccurrence(t, s, 1, 10, "b")
			path := diskPath(dir, activeSegmentPath(s))
			validator := s.validator
			_ = s.Close()
			rewriteEvents(t, path, validator, change)
			if reopened, err := Open(dir, options); !errors.Is(err, errCorruptStorage) {
				if err == nil {
					_ = reopened.Close()
				}
				t.Fatalf("err = %v, want corrupt storage", err)
			} else if !strings.Contains(err.Error(), "device") && !strings.Contains(err.Error(), "clock") && !strings.Contains(err.Error(), "vector") {
				t.Fatalf("err = %v, want a causal metadata failure", err)
			}
		})
	}
}
