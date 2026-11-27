package eventlog

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/storage"
)

// errRuntimeStopped is returned internally when a test closes a runtime. The
// production runtime is process-lifetime scoped and never stops.
var errRuntimeStopped = errors.New("eventlog runtime stopped")

// runtimePolicy holds internal buffer, batch, and retry parameters. They are
// implementation/test policy, not public configuration.
type runtimePolicy struct {
	notificationBuffer int
	transactionBuffer  int
	batchLimit         int
	deliveryAttempts   int
	deliveryBackoff    time.Duration
	// deliveryFault is a test-only fault-injection hook; nil in production.
	deliveryFault func(channelID identity.UUID, event Event) error
}

func defaultRuntimePolicy() runtimePolicy {
	return runtimePolicy{
		notificationBuffer: 64,
		transactionBuffer:  64,
		batchLimit:         64,
		deliveryAttempts:   5,
		deliveryBackoff:    10 * time.Millisecond,
	}
}

// logRuntime is the process-scoped owner of storage, decoder registrations,
// Channel runtimes, their Readers, and their transaction processors. It is not
// tied to any PrepareLog caller's Context.
type logRuntime struct {
	store    occurrenceStore
	registry *typeRegistry
	policy   runtimePolicy

	mu       sync.Mutex
	channels map[identity.UUID]*channelRuntime

	closed    chan struct{}
	closeOnce sync.Once
	workers   sync.WaitGroup
}

// occurrenceStore is the authoritative append log the runtime persists to and
// reads from; *storage.Store implements it.
type occurrenceStore interface {
	AppendTransaction(channel storage.ChannelWrite, occurrences []storage.OccurrenceWrite) error
	ChannelOccurrence(channelID identity.UUID, position int) (storage.Occurrence, bool, <-chan struct{})
	ReadOccurrenceFrame(channelID identity.UUID, position int) ([]byte, error)
	ChannelHistoryTypes(channelID identity.UUID) []fieldschema.TypeKey
	Close() error
}

func openLogRuntime(directory string, storeOptions storage.Options, policy runtimePolicy) (*logRuntime, error) {
	store, err := storage.Open(directory, storeOptions)
	if err != nil {
		return nil, err
	}
	return newLogRuntime(store, policy), nil
}

func newLogRuntime(store occurrenceStore, policy runtimePolicy) *logRuntime {
	return &logRuntime{
		store:    store,
		registry: newTypeRegistry(),
		policy:   policy,
		channels: make(map[identity.UUID]*channelRuntime),
		closed:   make(chan struct{}),
	}
}

func (rt *logRuntime) startWorker(work func()) {
	rt.workers.Add(1)
	go func() {
		defer rt.workers.Done()
		work()
	}()
}

// close stops every worker and releases storage. It is test-only; eventlog
// exposes no shutdown and the production runtime lives until process exit.
func (rt *logRuntime) close() error {
	var err error
	rt.closeOnce.Do(func() {
		close(rt.closed)
		rt.workers.Wait()
		err = rt.store.Close()
	})
	return err
}

var processLog struct {
	mu      sync.Mutex
	runtime *logRuntime
	pending *runtimeInitialization
}

type runtimeInitialization struct {
	done    chan struct{}
	runtime *logRuntime
	err     error
}

// sharedLogRuntime returns the process runtime, starting initialization at
// most once at a time. Concurrent callers share the same attempt and each
// honors only its own Context; a failed attempt may be retried later.
func sharedLogRuntime(ctx context.Context, settings *configSettings) (*logRuntime, error) {
	processLog.mu.Lock()
	if rt := processLog.runtime; rt != nil {
		processLog.mu.Unlock()
		return rt, nil
	}
	attempt := processLog.pending
	if attempt == nil {
		attempt = &runtimeInitialization{done: make(chan struct{})}
		processLog.pending = attempt
		go initializeProcessRuntime(attempt, settings.storageDirectory)
	}
	processLog.mu.Unlock()
	select {
	case <-attempt.done:
		return attempt.runtime, attempt.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func initializeProcessRuntime(attempt *runtimeInitialization, directory string) {
	rt, err := openLogRuntime(directory, storage.DefaultOptions(), defaultRuntimePolicy())
	processLog.mu.Lock()
	if err == nil {
		processLog.runtime = rt
	}
	processLog.pending = nil
	attempt.runtime, attempt.err = rt, err
	processLog.mu.Unlock()
	close(attempt.done)
}
