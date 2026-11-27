package eventlog

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/storage"
)

// refusingStore stands in for storage that refuses this device's writes: from
// the start when refusal is set, and from the first append once refuseAppends
// is set, as *storage.Store reports them.
type refusingStore struct {
	occurrenceStore
	refusal       atomic.Pointer[error]
	refuseAppends atomic.Bool
}

func (s *refusingStore) refuse() {
	err := fmt.Errorf("storage refuses writes: %w", fs.ErrPermission)
	s.refusal.Store(&err)
}

func (s *refusingStore) WriteRefusal() error {
	if err := s.refusal.Load(); err != nil {
		return *err
	}
	return nil
}

func (s *refusingStore) AppendTransaction(channel storage.ChannelWrite, occurrences []storage.OccurrenceWrite) error {
	if err := s.WriteRefusal(); err != nil {
		return err
	}
	if s.refuseAppends.Load() {
		s.refuse()
		return s.WriteRefusal()
	}
	return s.occurrenceStore.AppendTransaction(channel, occurrences)
}

func openRefusingRuntime(t *testing.T, dir string) (*logRuntime, *refusingStore) {
	t.Helper()
	store, err := storage.Open(dir, fastStoreOptions())
	if err != nil {
		t.Fatal(err)
	}
	refusing := &refusingStore{occurrenceStore: store}
	rt := newLogRuntime(refusing, fastPolicy())
	runtimeDirectories.Store(rt, dir)
	t.Cleanup(func() {
		_ = rt.close()
	})
	return rt, refusing
}

func expectNoError(t *testing.T, channel Channel, wait time.Duration) {
	t.Helper()
	select {
	case err := <-channel.Errors():
		t.Fatalf("unexpected error %v", err)
	case <-time.After(wait):
	}
}

// D-068: on storage that refuses this device's writes, transactions are
// refused with WriteNotPermitted before they run, together with everything
// queued behind them, while replay and notifications continue.
func TestStorageRefusingWritesRefusesTransactions(t *testing.T) {
	dir := t.TempDir()
	writer := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, writer, testUUID(1), testUUID(2))
	setAmount(event, 1)
	expectAmount(t, channel, 1)
	if err := writer.close(); err != nil {
		t.Fatal(err)
	}

	rt, refusing := openRefusingRuntime(t, dir)
	refusing.refuse()
	event, channel = prepareOrder(t, rt, testUUID(1), testUUID(2))
	expectAmount(t, channel, 1)

	var ran, queuedRan atomic.Bool
	event.Transaction() <- func() error { ran.Store(true); event.Amount = 5; return nil }
	event.Transaction() <- func() error { queuedRan.Store(true); return nil }

	failure := receiveError(t, channel)
	if failure.Kind != WriteNotPermitted || !errors.Is(failure, fs.ErrPermission) || len(failure.Events) != 0 {
		t.Fatalf("failure = %v (%d events)", failure, len(failure.Events))
	}
	if message := failure.Error(); !strings.Contains(message, "write not permitted") || !strings.Contains(message, "queued behind it are rejected") {
		t.Fatalf("message = %q", message)
	}
	expectNoError(t, channel, 30*time.Millisecond)
	if ran.Load() || queuedRan.Load() {
		t.Fatal("a refused transaction was run")
	}
	if committedCount(rt) != 1 {
		t.Fatalf("committed = %d, want 1", committedCount(rt))
	}
}

// D-068: a commit refused while running is CommitFailed and rolled back;
// later transactions are refused with WriteNotPermitted before they run.
func TestWriteRefusedWhileRunningSwitchesToWriteNotPermitted(t *testing.T) {
	rt, refusing := openRefusingRuntime(t, t.TempDir())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	setAmount(event, 1)
	expectAmount(t, channel, 1)

	refusing.refuseAppends.Store(true)
	setAmount(event, 2)
	failure := receiveError(t, channel)
	if failure.Kind != CommitFailed || !errors.Is(failure, fs.ErrPermission) || len(failure.Events) != 1 {
		t.Fatalf("failure = %v (%d events)", failure, len(failure.Events))
	}

	var ran atomic.Bool
	event.Transaction() <- func() error { ran.Store(true); event.Amount = 3; return nil }
	if failure := receiveError(t, channel); failure.Kind != WriteNotPermitted || len(failure.Events) != 0 {
		t.Fatalf("failure = %v (%d events)", failure, len(failure.Events))
	}
	if ran.Load() {
		t.Fatal("a refused transaction was run")
	}
	expectNoNotification(t, channel, 30*time.Millisecond)
}
