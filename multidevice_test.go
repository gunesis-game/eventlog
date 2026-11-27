package eventlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/storage"
)

// sharedStorage serves one directory holding the device state folders of
// several test devices and one shared log folder, as Web storage will,
// through the real file-system server and client.
type sharedStorage struct {
	dir    string
	server *filesystem.Server
	root   *filesystem.Client
}

func newSharedStorage(t *testing.T) *sharedStorage {
	t.Helper()
	s := &sharedStorage{dir: t.TempDir()}
	s.restart(t)
	return s
}

// restart serves the directory afresh, so the previous server releases its
// open files, which Windows needs before they are moved.
func (s *sharedStorage) restart(t *testing.T) {
	t.Helper()
	if s.server != nil {
		_ = s.server.Close()
	}
	backend, err := filesystem.NewOSBackend(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	server, err := filesystem.Serve(backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})
	s.server, s.root = server, filesystem.NewClient(server.URL(), server.Token())
}

// device opens the runtime of a device with its own state folder, named name,
// on the shared log.
func (s *sharedStorage) device(t *testing.T, name string) *logRuntime {
	t.Helper()
	store, err := storage.OpenFolders(s.root.Sub(name), s.root.Sub("eventlog"), fastStoreOptions())
	if err != nil {
		t.Fatalf("open device %s: %v", name, err)
	}
	rt := newLogRuntime(store, fastPolicy())
	t.Cleanup(func() {
		_ = rt.close()
	})
	return rt
}

// logDir returns the operating-system path of a device's log folder.
func (s *sharedStorage) logDir(rt *logRuntime) string {
	return filepath.Join(s.dir, "eventlog", "devices", rt.store.Device().String())
}

// refresh reads what other devices have appended, as a file-change event
// will once storage sends them.
func refresh(t *testing.T, rt *logRuntime) {
	t.Helper()
	if err := rt.store.(*storage.Store).Refresh(); err != nil {
		t.Fatal(err)
	}
}

// lastVector returns the version vector of the last Event record in a
// device's log.
func (s *sharedStorage) lastVector(t *testing.T, rt *logRuntime) map[string]uint64 {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(s.logDir(rt), "segments", "segment-*.ndjson"))
	var vector map[string]uint64
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			var decoded struct {
				Kind   string            `json:"kind"`
				Vector map[string]uint64 `json:"vector"`
			}
			if json.Unmarshal([]byte(line), &decoded) == nil && decoded.Kind == "event" {
				vector = decoded.Vector
			}
		}
	}
	return vector
}

func expectKind(t *testing.T, channel Channel, kind ErrorKind) *ChannelError {
	t.Helper()
	err := receiveError(t, channel)
	if err.Kind != kind {
		t.Fatalf("error kind = %v (%v), want %v", err.Kind, err, kind)
	}
	return err
}

func amounts(events []Event) []int64 {
	values := make([]int64, len(events))
	for i, event := range events {
		values[i] = event.(*OrderPlaced).Amount
	}
	return values
}

// D-057: another device's occurrences are delivered after a refresh, and an
// occurrence written after delivering them counts them in its version
// vector, so it is not a conflict on the device that wrote them.
func TestOtherDevicesOccurrencesAreDelivered(t *testing.T) {
	shared := newSharedStorage(t)
	a, b := shared.device(t, "a"), shared.device(t, "b")
	eventA, ordersA := prepareOrder(t, a, testUUID(1), testUUID(2))
	eventB, ordersB := prepareOrder(t, b, testUUID(1), testUUID(2))

	setAmount(eventA, 5)
	expectAmount(t, ordersA, 5)
	expectNoNotification(t, ordersB, 50*time.Millisecond)
	refresh(t, b)
	expectAmount(t, ordersB, 5)

	setAmount(eventB, 7)
	expectAmount(t, ordersB, 7)
	want := map[string]uint64{a.store.Device().String(): 1, b.store.Device().String(): 1}
	if got := shared.lastVector(t, b); len(got) != 2 || got[a.store.Device().String()] != 1 || got[b.store.Device().String()] != 1 {
		t.Fatalf("vector = %v, want %v", got, want)
	}
	refresh(t, a)
	expectAmount(t, ordersA, 7)
	expectNoError(t, ordersA, 100*time.Millisecond)
}

// D-058, D-074: changes of one Event made on two devices, each without knowing
// of the other, are reported as ConcurrentChange on both; every device, a new
// one and a restarted one included, settles on the same current state and
// replays the occurrences in the same order.
func TestConcurrentChangesConverge(t *testing.T) {
	shared := newSharedStorage(t)
	a, b := shared.device(t, "a"), shared.device(t, "b")
	eventA, ordersA := prepareOrder(t, a, testUUID(1), testUUID(2))
	eventB, ordersB := prepareOrder(t, b, testUUID(1), testUUID(2))
	setAmount(eventA, 1)
	expectAmount(t, ordersA, 1)
	setAmount(eventB, 2)
	expectAmount(t, ordersB, 2)

	refresh(t, a)
	expectAmount(t, ordersA, 2)
	conflictA := expectKind(t, ordersA, ConcurrentChange)
	refresh(t, b)
	expectAmount(t, ordersB, 1)
	conflictB := expectKind(t, ordersB, ConcurrentChange)
	current := amounts(conflictA.Events)
	if got := amounts(conflictB.Events); len(current) != 2 || got[0] != current[0] || got[1] != current[1] || current[0] == current[1] {
		t.Fatalf("conflicts disagree: device a %v, device b %v", current, got)
	}

	replay := func(rt *logRuntime) ([]int64, []int64) {
		t.Helper()
		_, orders := prepareOrder(t, rt, testUUID(1), testUUID(2))
		order := []int64{receiveEvent(t, orders).(*OrderPlaced).Amount, receiveEvent(t, orders).(*OrderPlaced).Amount}
		return order, amounts(expectKind(t, orders, ConcurrentChange).Events)
	}
	orderC, conflictC := replay(shared.device(t, "c"))
	if orderC[1] != current[0] || conflictC[0] != current[0] || conflictC[1] != current[1] {
		t.Fatalf("new device replayed %v with conflict %v; want the current state %d last", orderC, conflictC, current[0])
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	orderA, conflictRestarted := replay(shared.device(t, "a"))
	if orderA[0] != orderC[0] || orderA[1] != orderC[1] || conflictRestarted[0] != current[0] {
		t.Fatalf("restarted device replayed %v with conflict %v; new device %v", orderA, conflictRestarted, orderC)
	}
}

// D-070: when another device's history is missing, the occurrences that
// depend on it are held, never skipped, HistoryMissing is reported once,
// unrelated occurrences keep flowing, and delivery resumes in order when the
// history returns.
func TestDependentsWaitForMissingHistory(t *testing.T) {
	shared := newSharedStorage(t)
	a, b := shared.device(t, "a"), shared.device(t, "b")
	eventA, ordersA := prepareOrder(t, a, testUUID(1), testUUID(2))
	setAmount(eventA, 1)
	expectAmount(t, ordersA, 1)
	eventB, ordersB := prepareOrder(t, b, testUUID(1), testUUID(2))
	refresh(t, b)
	expectAmount(t, ordersB, 1)
	setAmount(eventB, 2)
	expectAmount(t, ordersB, 2)
	missingDevice := a.store.Device()
	segments, _ := filepath.Glob(filepath.Join(shared.logDir(a), "segments", "segment-*.ndjson"))
	if len(segments) != 1 {
		t.Fatalf("device a segments = %v", segments)
	}
	_ = a.close()
	_ = b.close()
	shared.restart(t)
	hidden := segments[0] + ".hidden"
	if err := os.Rename(segments[0], hidden); err != nil {
		t.Fatal(err)
	}

	c := shared.device(t, "c")
	eventC, ordersC := prepareOrder(t, c, testUUID(1), testUUID(3))
	missing := expectKind(t, ordersC, HistoryMissing)
	if !strings.Contains(missing.Error(), missingDevice.String()) {
		t.Fatalf("HistoryMissing does not name device %s: %v", missingDevice, missing)
	}
	expectNoNotification(t, ordersC, 100*time.Millisecond)
	setAmount(eventC, 3)
	expectAmount(t, ordersC, 3)
	refresh(t, c)
	expectNoError(t, ordersC, 100*time.Millisecond)

	if err := os.Rename(hidden, segments[0]); err != nil {
		t.Fatal(err)
	}
	refresh(t, c)
	expectAmount(t, ordersC, 1)
	expectAmount(t, ordersC, 2)
	expectNoError(t, ordersC, 100*time.Millisecond)
}

// readLive reads a live Event's amount and its Channel's region inside a
// transaction, the race-free way to read live fields.
func readLive(event *OrderPlaced, channel *Orders) (amount int64, region string) {
	read(event, func() {
		amount, region = event.Amount, channel.Region
	})
	return amount, region
}

func increment(event *OrderPlaced) {
	event.Transaction() <- func() error {
		event.Amount++
		return nil
	}
}

// D-059, D-075: another device's commit sets the live Event and Channel to
// the converged state, and local transactions interleaved with remote ones
// start from it.
func TestLiveInstancesTakeOtherDevicesChanges(t *testing.T) {
	shared := newSharedStorage(t)
	a, b := shared.device(t, "a"), shared.device(t, "b")
	eventA, ordersA := prepareOrder(t, a, testUUID(1), testUUID(2))
	eventB, ordersB := prepareOrder(t, b, testUUID(1), testUUID(2))

	eventA.Transaction() <- func() error {
		eventA.Amount = 5
		ordersA.Region = "us"
		return nil
	}
	expectAmount(t, ordersA, 5)
	refresh(t, b)
	expectAmount(t, ordersB, 5)
	if amount, region := readLive(eventB, ordersB); amount != 5 || region != "us" {
		t.Fatalf("device b live state = %d %q, want 5 \"us\"", amount, region)
	}

	increment(eventB)
	expectAmount(t, ordersB, 6)
	refresh(t, a)
	expectAmount(t, ordersA, 6)
	increment(eventA)
	expectAmount(t, ordersA, 7)
	refresh(t, b)
	expectAmount(t, ordersB, 7)
	if amount, region := readLive(eventB, ordersB); amount != 7 || region != "us" {
		t.Fatalf("device b live state = %d %q, want 7 \"us\"", amount, region)
	}
	expectNoError(t, ordersA, 50*time.Millisecond)
	expectNoError(t, ordersB, 50*time.Millisecond)
}

// D-075: a device's own delivered occurrences never take its live Event back
// to an older state while later local transactions are being committed. The
// Reader is slowed so it lags behind the transaction processor.
func TestOwnDeliveriesKeepLaterLocalChanges(t *testing.T) {
	policy := fastPolicy()
	policy.deliveryFault = func(identity.UUID, Event) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	}
	rt := openTestRuntime(t, t.TempDir(), fastStoreOptions(), policy)
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	const n = 30
	for i := 0; i < n; i++ {
		increment(event)
		time.Sleep(time.Millisecond)
	}
	for i := int64(1); i <= n; i++ {
		expectAmount(t, channel, i)
	}
	if amount, _ := readLive(event, channel); amount != n {
		t.Fatalf("live amount = %d, want %d", amount, n)
	}
}

// D-075: after a restart, live instances hold the persisted current state
// instead of their constructor values, whether prepared before or after
// replay reaches them.
func TestLiveInstancesTakePersistedStateAfterRestart(t *testing.T) {
	dir := t.TempDir()
	rt := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	event, channel := prepareOrder(t, rt, testUUID(1), testUUID(2))
	event.Transaction() <- func() error {
		event.Amount = 9
		channel.Region = "us"
		return nil
	}
	expectAmount(t, channel, 9)
	_ = rt.close()

	restarted := openTestRuntime(t, dir, fastStoreOptions(), fastPolicy())
	event, channel = prepareOrder(t, restarted, testUUID(1), testUUID(2))
	expectAmount(t, channel, 9)
	if amount, region := readLive(event, channel); amount != 9 || region != "us" {
		t.Fatalf("live state after replay = %d %q, want 9 \"us\"", amount, region)
	}
	later, _ := prepareOrder(t, restarted, testUUID(1), testUUID(2))
	if later != event {
		t.Fatal("preparing again did not reuse the live Event")
	}
	another, _, err := prepareInRuntime[*OrderShipped, *Orders](t.Context(), restarted, shippedFactory(testUUID(4)), ordersFactory(testUUID(1)))
	if err != nil {
		t.Fatal(err)
	}
	read(another, func() {
		if another.Carrier != "" {
			t.Errorf("an Event without history changed: %q", another.Carrier)
		}
	})
}

// D-058, D-075: after concurrent changes, both devices' live Events hold the
// same current state.
func TestConcurrentChangesConvergeLiveInstances(t *testing.T) {
	shared := newSharedStorage(t)
	a, b := shared.device(t, "a"), shared.device(t, "b")
	eventA, ordersA := prepareOrder(t, a, testUUID(1), testUUID(2))
	eventB, ordersB := prepareOrder(t, b, testUUID(1), testUUID(2))
	setAmount(eventA, 1)
	expectAmount(t, ordersA, 1)
	setAmount(eventB, 2)
	expectAmount(t, ordersB, 2)
	refresh(t, a)
	expectAmount(t, ordersA, 2)
	current := expectKind(t, ordersA, ConcurrentChange).Events[0].(*OrderPlaced).Amount
	refresh(t, b)
	expectAmount(t, ordersB, 1)
	expectKind(t, ordersB, ConcurrentChange)
	amountA, _ := readLive(eventA, ordersA)
	amountB, _ := readLive(eventB, ordersB)
	if amountA != current || amountB != current {
		t.Fatalf("live amounts = %d and %d, want both %d", amountA, amountB, current)
	}
}

// D-075: applying the converged state never writes an identity field, even
// from a snapshot holding a different identity, and writes only the other
// fields whose values differ. A nullable field holds a pointer, so
// rewriting it with an equal value would replace the pointer; it must stay
// the same one.
func TestConvergedStateNeverWritesIdentity(t *testing.T) {
	schema, err := resolveSchema(&OrderPlaced{})
	if err != nil {
		t.Fatal(err)
	}
	note := stringPointer("note")
	live := &OrderPlaced{ID: testUUID(2), Amount: 1, Note: note}
	other := &OrderPlaced{ID: testUUID(9), Amount: 7, Note: stringPointer("note")}
	snapshot, _, err := fieldschema.Capture(schema, reflect.ValueOf(other))
	if err != nil {
		t.Fatal(err)
	}
	restoreChanged(schema, reflect.ValueOf(live), snapshot)
	if live.ID != OrderID(testUUID(2)) {
		t.Fatalf("identity was overwritten with %v", live.ID)
	}
	if live.Amount != 7 {
		t.Fatalf("amount = %d, want the converged 7", live.Amount)
	}
	if live.Note != note {
		t.Fatal("an unchanged nullable field was rewritten with a new pointer")
	}
}

// D-075: reading identity fields outside a transaction while other devices'
// changes are applied to the live Event and Channel never races with a
// write; the race detector, which CI runs on Linux, reports any write.
func TestConvergedStateNeverRacesWithIdentityReads(t *testing.T) {
	shared := newSharedStorage(t)
	a, b := shared.device(t, "a"), shared.device(t, "b")
	eventA, ordersA := prepareOrder(t, a, testUUID(1), testUUID(2))
	eventB, ordersB := prepareOrder(t, b, testUUID(1), testUUID(2))
	stop, done := make(chan struct{}), make(chan struct{})
	changed := false
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if eventB.ID != OrderID(testUUID(2)) || ordersB.ID != CustomerID(testUUID(1)) {
				changed = true
				return
			}
		}
	}()
	for i := int64(1); i <= 5; i++ {
		eventA.Transaction() <- func() error {
			eventA.Amount = i
			ordersA.Region = fmt.Sprint("region-", i)
			return nil
		}
		expectAmount(t, ordersA, i)
		refresh(t, b)
		expectAmount(t, ordersB, i)
	}
	close(stop)
	<-done
	if changed {
		t.Fatal("an identity field changed")
	}
	if amount, region := readLive(eventB, ordersB); amount != 5 || region != "region-5" {
		t.Fatalf("device b live state = %d %q, want 5 \"region-5\"", amount, region)
	}
}
