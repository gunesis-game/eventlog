// Package storage owns eventlog's authoritative append log on the local
// filesystem: segments, manifest, the derived index, transaction writing,
// durability policy, recovery, and compaction.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"sync"
	"time"

	"github.com/gunesis-game/eventlog/internal/filesystem"

	"github.com/gunesis-game/eventlog/internal/causal"
	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

const segmentsDirName = "segments"

// segmentPath returns the path of a segment, relative to its device's log.
func segmentPath(id uint64) string {
	return path.Join(segmentsDirName, segmentFileName(id))
}

// Options are internal storage policy values, not public configuration.
type Options struct {
	TargetSegmentSize   int64
	CompactionThreshold int
	SyncInterval        time.Duration
	SyncBytes           int64
	// Now reads the wall clock for hybrid logical clock timestamps; nil means
	// time.Now. Tests set it to step the clock.
	Now func() time.Time
}

// DefaultOptions returns the production storage policy.
func DefaultOptions() Options {
	return Options{
		TargetSegmentSize:   64 << 20,
		CompactionThreshold: 8,
		SyncInterval:        time.Second,
		SyncBytes:           1 << 20,
	}
}

// ChannelWrite names the Channel a transaction belongs to.
type ChannelWrite struct {
	ID   identity.UUID
	Type fieldschema.TypeKey
}

// OccurrenceWrite is one Event occurrence with the Channel fields it was made
// with.
type OccurrenceWrite struct {
	ChannelFields map[string]interface{}
	EventID       identity.UUID
	EventType     fieldschema.TypeKey
	EventFields   map[string]interface{}
}

// Store is this device's open append log in one storage. It is safe for
// concurrent use.
type Store struct {
	// state is this device's state folder, log the shared log folder, and
	// files this device's own folder in it.
	state     *filesystem.Client
	log       *filesystem.Client
	files     *filesystem.Client
	server    *filesystem.Server
	validator *record.Validator
	options   Options
	// device is this device's identity, recorded with every Event occurrence.
	device identity.UUID
	// others are the other devices' logs, read but never written (D-073).
	others map[identity.UUID]*otherLog

	mu           sync.Mutex
	manifest     manifest
	finalized    []*segment
	active       *segment
	nextSequence uint64
	unsynced     int64
	index        *index
	// clock stamps each Event occurrence with a hybrid logical clock reading.
	clock *causal.Clock
	// vectors holds, per Channel, the version vector of this device's next
	// occurrence before its own entry is advanced: how many occurrences of the
	// Channel this device has written, and how many of each other device's it
	// has delivered.
	vectors map[identity.UUID]map[string]uint64
	// wake is closed and replaced when this device commits or a refresh reads
	// other devices' logs.
	wake         chan struct{}
	indexDirty   bool
	writeFailure error
	// writeRefusal is the permission error of storage that refuses this
	// device's writes, found at open or by a later write; nil while writes are
	// permitted.
	writeRefusal error
	// locked records whether this store holds the ownership lock; a store
	// opened read-only does not.
	locked       bool
	indexRebuilt bool
	// pendingDeletes holds retired segment paths whose removal failed, for
	// example because another process briefly held them open on Windows.
	pendingDeletes []string

	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
}

// Open creates or recovers the storage directory, laid out as described in
// layout.go, and returns this device's store, ready for appends. It starts the
// store's file-system server over the local operating system. Recovery
// validates every segment and fails closed on anything ambiguous or corrupt.
func Open(directory string, options Options) (*Store, error) {
	backend, err := filesystem.NewOSBackend(directory)
	if err != nil {
		return nil, err
	}
	server, err := filesystem.Serve(backend)
	if err != nil {
		_ = backend.Close()
		return nil, err
	}
	s, err := openLocal(filesystem.NewClient(server.URL(), server.Token()), options)
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	s.server = server
	return s, nil
}

// openLocal opens the store of a Local directory served at root, which holds
// the device state folder and the log folder side by side.
func openLocal(root *filesystem.Client, options Options) (*Store, error) {
	return OpenFolders(root.Sub(localDeviceStateDir), root.Sub(localLogDir), options)
}

// OpenFolders opens this device's store on a device state folder and a log
// folder served by file-system clients, which several devices may share. It
// recovers this device's log in the log folder, creating the log's and
// this device's identities on first use, and opens every other device's log
// read-only. When the storage refuses a write that opening needs
// (fs.ErrPermission), it opens again read-only: nothing is written, the
// ownership lock is not taken, recovery repairs nothing, and every append fails
// with that permission error.
func OpenFolders(state, log *filesystem.Client, options Options) (*Store, error) {
	validator, err := record.NewValidator()
	if err != nil {
		return nil, err
	}
	s, err := openWritable(state, log, options, validator)
	if errors.Is(err, fs.ErrPermission) {
		s, err = openReadOnly(state, log, options, validator, err)
	}
	if err != nil {
		return nil, err
	}
	if err := s.Refresh(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func openWritable(state, log *filesystem.Client, options Options, validator *record.Validator) (*Store, error) {
	if err := state.MakeDirectory(""); err != nil {
		return nil, fmt.Errorf("create device state folder: %w", err)
	}
	if err := state.Lock(lockFileName); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, err
		}
		return nil, fmt.Errorf("storage is owned by another process: %w", err)
	}
	s, err := openLockedLog(state, log, options, validator)
	if err != nil {
		_ = state.Unlock(lockFileName)
		return nil, err
	}
	s.locked = true
	return s, nil
}

func openLockedLog(state, log *filesystem.Client, options Options, validator *record.Validator) (*Store, error) {
	if err := log.MakeDirectory(devicesDirName); err != nil {
		return nil, fmt.Errorf("create log folder: %w", err)
	}
	if _, err := ensureLogID(log); err != nil {
		return nil, err
	}
	device, err := ensureDeviceID(state)
	if err != nil {
		return nil, err
	}
	files := log.Sub(deviceLogDir(device))
	if err := files.MakeDirectory(segmentsDirName); err != nil {
		return nil, fmt.Errorf("create segments folder: %w", err)
	}
	recovered, err := newRecovery(files, validator, device, false).run()
	if err != nil {
		return nil, err
	}
	return newStore(state, log, files, device, options, validator, recovered), nil
}

// openReadOnly opens this device's log without writing. Storage where this
// device has no identity yet holds no log of it: its own log is empty.
func openReadOnly(state, log *filesystem.Client, options Options, validator *record.Validator, refusal error) (*Store, error) {
	device, err := readDeviceID(state)
	var recovered *recovery
	switch {
	case errors.Is(err, fs.ErrNotExist):
		recovered = emptyRecovery()
	case err != nil:
		return nil, err
	default:
		recovered, err = newRecovery(log.Sub(deviceLogDir(device)), validator, device, true).run()
		if err != nil {
			return nil, err
		}
	}
	s := newStore(state, log, log.Sub(deviceLogDir(device)), device, options, validator, recovered)
	s.writeRefusal = refusal
	return s, nil
}

func newStore(state, log, files *filesystem.Client, device identity.UUID, options Options, validator *record.Validator, recovered *recovery) *Store {
	s := &Store{
		state:        state,
		log:          log,
		files:        files,
		others:       make(map[identity.UUID]*otherLog),
		device:       device,
		clock:        causal.NewClock(recovered.lastHLC, options.Now),
		vectors:      recovered.vectors,
		validator:    validator,
		options:      options,
		manifest:     recovered.manifest,
		finalized:    recovered.finalized,
		active:       recovered.active,
		nextSequence: recovered.nextSequence,
		index:        recovered.index,
		indexRebuilt: recovered.indexRebuilt,
		wake:         make(chan struct{}),
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
	}
	go s.runSyncLoop()
	return s
}

// WriteRefusal returns the permission error of storage that refuses this
// device's writes, or nil while writes are permitted.
func (s *Store) WriteRefusal() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeRefusal
}

// AppendTransaction atomically appends one transaction of a Channel's Event
// occurrences. Each occurrence is preceded by a Channel record when the
// Channel is new or its fields differ from the last Channel record written.
// Validation failures write nothing.
func (s *Store) AppendTransaction(channel ChannelWrite, occurrences []OccurrenceWrite) error {
	if len(occurrences) == 0 {
		return errors.New("transaction has no occurrences")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeRefusal != nil {
		return s.writeRefusal
	}
	if s.writeFailure != nil {
		return fmt.Errorf("storage is unwritable after an unrecoverable write failure: %w", s.writeFailure)
	}
	existing, known := s.index.channel(channel.ID)
	if known && existing.typ != channel.Type {
		return fmt.Errorf("channel %s is persisted as %s, not %s", channel.ID, existing.typ, channel.Type)
	}
	lastChannelFields := existing.fields
	if !known {
		lastChannelFields = ""
	}

	type framedEvent struct {
		sequence      uint64
		offset        int64
		length        int64
		write         OccurrenceWrite
		channelFields string
	}
	sequence := s.nextSequence
	vector := maps.Clone(s.vectors[channel.ID])
	if vector == nil {
		vector = make(map[string]uint64)
	}
	device := s.device.String()
	var frames [][]byte
	var events []framedEvent
	var transaction []byte
	for _, occurrence := range occurrences {
		channelFields, err := json.Marshal(occurrence.ChannelFields)
		if err != nil {
			return fmt.Errorf("encode channel fields: %w", err)
		}
		if string(channelFields) != lastChannelFields {
			channelRecord := record.Channel{SchemaVersion: 1, Sequence: sequence, Kind: record.KindChannel, ID: channel.ID.String(), Type: record.Type{PackagePath: channel.Type.PackagePath, Name: channel.Type.Name}, Fields: occurrence.ChannelFields}
			frame, err := record.Encode(record.KindChannel, channelRecord, s.validator)
			if err != nil {
				return err
			}
			frames = append(frames, frame)
			transaction = append(transaction, frame...)
			lastChannelFields = string(channelFields)
			sequence++
		}
		vector[device]++
		eventRecord := record.Event{SchemaVersion: 1, Sequence: sequence, Kind: record.KindEvent, ID: occurrence.EventID.String(), ChannelID: channel.ID.String(), Type: record.Type{PackagePath: occurrence.EventType.PackagePath, Name: occurrence.EventType.Name}, Fields: occurrence.EventFields,
			DeviceID: device, Vector: maps.Clone(vector), HLC: s.clock.Tick()}
		frame, err := record.Encode(record.KindEvent, eventRecord, s.validator)
		if err != nil {
			return err
		}
		events = append(events, framedEvent{sequence: sequence, offset: int64(len(transaction)), length: int64(len(frame)), write: occurrence, channelFields: lastChannelFields})
		frames = append(frames, frame)
		transaction = append(transaction, frame...)
		sequence++
	}
	commit := record.Commit{SchemaVersion: 1, Sequence: sequence, Kind: record.KindCommit, TransactionID: s.nextSequence, RecordCount: uint64(len(frames)), Checksum: record.Checksum(frames)}
	commitFrame, err := record.Encode(record.KindCommit, commit, s.validator)
	if err != nil {
		return err
	}
	transaction = append(transaction, commitFrame...)

	if s.active.size > 0 && s.active.size+int64(len(transaction)) > s.options.TargetSegmentSize {
		if err := s.rotateLocked(); err != nil {
			if errors.Is(err, fs.ErrPermission) {
				s.writeRefusal = err
			}
			return err
		}
	}
	start := s.active.size
	size, err := s.files.Append(s.active.path, transaction)
	if errors.Is(err, fs.ErrPermission) {
		// Storage refused the write without changing anything; later
		// appends fail the same way without trying.
		s.writeRefusal = err
		return err
	}
	if err == nil && size != start+int64(len(transaction)) {
		err = fmt.Errorf("append to %s ended at %d, want %d", s.active.path, size, start+int64(len(transaction)))
	}
	if err != nil {
		s.rollbackLocked(start, err)
		return err
	}
	s.active.size += int64(len(transaction))
	s.nextSequence = commit.Sequence + 1
	s.unsynced += int64(len(transaction))
	s.index.setChannel(channel.ID, channelSnapshot{typ: channel.Type, fields: lastChannelFields})
	s.vectors[channel.ID] = vector
	for _, event := range events {
		s.index.add(&Occurrence{Sequence: event.sequence, EventID: event.write.EventID, ChannelID: channel.ID, Type: event.write.EventType, ChannelFields: event.channelFields, segment: s.active, offset: start + event.offset, length: event.length})
	}
	s.indexDirty = true
	if s.unsynced >= s.options.SyncBytes {
		s.syncLocked()
	}
	s.changedLocked()
	return nil
}

// rollbackLocked removes a partially written transaction so later appends
// never follow garbage. If truncation also fails the store stops accepting
// writes; recovery on the next start discards the incomplete transaction.
func (s *Store) rollbackLocked(start int64, cause error) {
	if err := s.files.Truncate(s.active.path, start); err != nil {
		s.writeFailure = fmt.Errorf("%v; rollback failed: %w", cause, err)
	}
}

// rotateLocked finalizes the active segment at a transaction boundary, creates
// the next segment, and publishes it in the manifest. It may then compact.
func (s *Store) rotateLocked() error {
	if err := s.files.Sync(s.active.path); err != nil {
		return err
	}
	s.unsynced = 0
	id := s.manifest.NextSegmentID
	next := segmentPath(id)
	if err := s.files.CreateExclusive(next); err != nil {
		return err
	}
	updated := s.manifest
	updated.Segments = append(append([]string(nil), s.manifest.Segments...), segmentFileName(id))
	updated.NextSegmentID = id + 1
	if err := writeManifest(s.files, updated); err != nil {
		_ = s.files.Remove(next)
		return err
	}
	s.manifest = updated
	s.finalized = append(s.finalized, s.active)
	s.active = &segment{id: id, path: next}
	if len(s.finalized) >= s.options.CompactionThreshold {
		if err := s.compactLocked(); err != nil {
			return fmt.Errorf("compact segments: %w", err)
		}
	}
	return nil
}

// compactLocked consolidates the finalized segments into one replacement and
// switches the store to the new generation. Sources still being read are
// deleted when their last read finishes.
func (s *Store) compactLocked() error {
	if len(s.finalized) < 2 {
		return nil
	}
	result, err := (&compaction{files: s.files, validator: s.validator}).run(s.manifest, s.finalized, s.active)
	if err != nil {
		return err
	}
	s.removeRetiredLocked(s.active)
	s.index.relocate(result.bases, result.replacement)
	for _, source := range s.finalized {
		source.retired = true
		if source.refs == 0 {
			s.removeRetiredLocked(source)
		}
	}
	s.manifest = result.manifest
	s.finalized = []*segment{result.replacement}
	s.active = result.active
	s.indexDirty = true
	return s.writeIndexLocked()
}

func (s *Store) syncLocked() {
	if s.unsynced == 0 {
		return
	}
	if err := s.files.Sync(s.active.path); err != nil {
		s.writeFailure = err
		return
	}
	s.unsynced = 0
}

func (s *Store) writeIndexLocked() error {
	return s.index.persisted(s.manifest.Generation, s.nextSequence-1).write(s.files)
}

// runSyncLoop applies the periodic durability policy, persists the derived
// index when it changed, and retries failed segment deletions.
func (s *Store) runSyncLoop() {
	defer close(s.stopped)
	ticker := time.NewTicker(s.options.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			s.syncLocked()
			if s.indexDirty {
				if err := s.writeIndexLocked(); err == nil {
					s.indexDirty = false
				}
			}
			s.retryPendingDeletesLocked()
			s.mu.Unlock()
		}
	}
}

// Committed returns how many occurrences of a Channel this device has
// committed: its own entry in the Channel's version vector.
func (s *Store) Committed(channelID identity.UUID) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vectors[channelID][s.device.String()]
}

// Device returns this device's identity.
func (s *Store) Device() identity.UUID {
	return s.device
}

// Changed returns a channel closed on the next commit of this device and the
// next refresh of other devices' logs; callers take it before reading and
// read again after it closes.
func (s *Store) Changed() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wake
}

// changedLocked wakes the callers waiting on Changed.
func (s *Store) changedLocked() {
	close(s.wake)
	s.wake = make(chan struct{})
}

// Delivered records that this device delivered occurrence count of a Channel
// from another device's log, with clock reading at: later occurrences this
// device writes to the Channel count it in their version vectors, and their
// clock readings come after it (D-064).
func (s *Store) Delivered(channelID, device identity.UUID, count uint64, at causal.Timestamp) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vector := s.vectors[channelID]
	if vector == nil {
		vector = make(map[string]uint64)
		s.vectors[channelID] = vector
	}
	vector[device.String()] = max(vector[device.String()], count)
	s.clock.Observe(at)
}

// maxReadSpan bounds the bytes one batched occurrence read fetches.
const maxReadSpan = 1 << 20

// ReadOccurrences returns up to limit consecutive committed occurrences of a
// Channel starting at position, with their record bytes (without newlines).
// The batch stays within one segment and maxReadSpan bytes, so it is fetched
// with a single ranged read. The segment is pinned so compaction cannot
// delete it mid-read. An empty result means there is no occurrence at
// position yet.
func (s *Store) ReadOccurrences(channelID identity.UUID, position, limit int) ([]Occurrence, [][]byte, error) {
	s.mu.Lock()
	batch := collectBatch(s.index, channelID, position, limit)
	if len(batch) == 0 {
		s.mu.Unlock()
		return nil, nil, nil
	}
	seg := batch[0].segment
	seg.refs++
	s.mu.Unlock()
	defer s.unpin(seg)
	frames, err := readFrames(s.files, batch)
	if err != nil {
		return nil, nil, err
	}
	return batch, frames, nil
}

// collectBatch returns up to limit consecutive occurrences of a Channel in x
// from position, all in one segment and within maxReadSpan bytes.
func collectBatch(x *index, channelID identity.UUID, position, limit int) []Occurrence {
	first, ok := x.occurrence(channelID, position)
	if !ok {
		return nil
	}
	batch := []Occurrence{*first}
	for len(batch) < limit {
		next, ok := x.occurrence(channelID, position+len(batch))
		if !ok || next.segment != first.segment || next.offset+next.length-first.offset > maxReadSpan {
			break
		}
		batch = append(batch, *next)
	}
	return batch
}

// readFrames fetches a batch from collectBatch with one ranged read and splits
// it into record frames, without newlines.
func readFrames(files *filesystem.Client, batch []Occurrence) ([][]byte, error) {
	first, last := batch[0], batch[len(batch)-1]
	span, err := files.Read(first.segment.path, first.offset, last.offset+last.length-first.offset)
	if err != nil {
		return nil, fmt.Errorf("read occurrences %d-%d of %s: %w", first.Sequence, last.Sequence, first.segment.path, err)
	}
	frames := make([][]byte, len(batch))
	for i, occurrence := range batch {
		frame := span[occurrence.offset-first.offset : occurrence.offset-first.offset+occurrence.length]
		if len(frame) == 0 || frame[len(frame)-1] != '\n' {
			return nil, fmt.Errorf("occurrence %d is not a complete frame", occurrence.Sequence)
		}
		frames[i] = frame[:len(frame)-1]
	}
	return frames, nil
}

func (s *Store) unpin(seg *segment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seg.refs--
	if seg.retired && seg.refs == 0 {
		s.removeRetiredLocked(seg)
	}
}

// removeRetiredLocked deletes a retired segment, queueing it for retry when
// removal fails. A leftover file is never authoritative: it is outside the
// manifest, and recovery removes such orphans after validation.
func (s *Store) removeRetiredLocked(seg *segment) {
	s.removeFileLocked(seg.path)
}

func (s *Store) removeFileLocked(p string) {
	if err := s.files.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.pendingDeletes = append(s.pendingDeletes, p)
	}
}

func (s *Store) retryPendingDeletesLocked() {
	pending := s.pendingDeletes
	s.pendingDeletes = nil
	for _, p := range pending {
		s.removeFileLocked(p)
	}
}

// ChannelHistoryTypes returns the distinct Event types in a Channel's
// committed history, in first-occurrence order.
func (s *Store) ChannelHistoryTypes(channelID identity.UUID) []fieldschema.TypeKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index.historyTypes(channelID)
}

// Close stops background work and releases files. The production runtime is
// process-lifetime scoped and never closes its store; tests do.
func (s *Store) Close() error {
	var err error
	s.stopOnce.Do(func() {
		close(s.stop)
		<-s.stopped
		s.mu.Lock()
		defer s.mu.Unlock()
		s.syncLocked()
		if s.indexDirty {
			_ = s.writeIndexLocked()
		}
		s.retryPendingDeletesLocked()
		if s.locked {
			err = s.state.Unlock(lockFileName)
		}
		s.files.Close()
		if s.server != nil {
			err = errors.Join(err, s.server.Close())
		}
	})
	return err
}
