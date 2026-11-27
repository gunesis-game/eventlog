// Package storage owns eventlog's authoritative append log on the local
// filesystem: segments, manifest, the derived index, transaction writing,
// durability policy, recovery, and compaction.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

const lockFileName = "lock"

// Options are internal storage policy values, not public configuration.
type Options struct {
	TargetSegmentSize   int64
	CompactionThreshold int
	SyncInterval        time.Duration
	SyncBytes           int64
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

// Store is the open append log of one storage directory. It is safe for
// concurrent use.
type Store struct {
	directory   string
	segmentsDir string
	validator   *record.Validator
	options     Options
	lockFile    *os.File

	mu           sync.Mutex
	manifest     manifest
	finalized    []*segment
	active       *segment
	activeFile   *os.File
	nextSequence uint64
	unsynced     int64
	index        *index
	wake         chan struct{}
	indexDirty   bool
	writeFailure error
	indexRebuilt bool
	// pendingDeletes holds retired segment paths whose removal failed, for
	// example because another process briefly held them open on Windows.
	pendingDeletes []string

	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
}

// Open creates or recovers the storage directory and returns a store ready
// for appends. Recovery validates every segment and fails closed on anything
// ambiguous or corrupt.
func Open(directory string, options Options) (*Store, error) {
	if err := createDirectory(directory); err != nil {
		return nil, err
	}
	segmentsDir := filepath.Join(directory, segmentsDirName)
	if err := createDirectory(segmentsDir); err != nil {
		return nil, err
	}
	validator, err := record.NewValidator()
	if err != nil {
		return nil, err
	}
	lockFile, err := acquireLock(filepath.Join(directory, lockFileName))
	if err != nil {
		return nil, err
	}
	recovered, err := (&recovery{directory: directory, segmentsDir: segmentsDir, validator: validator, index: newIndex(), nextSequence: 1}).run()
	if err != nil {
		_ = lockFile.Close()
		return nil, err
	}
	s := &Store{
		directory:    directory,
		segmentsDir:  segmentsDir,
		validator:    validator,
		options:      options,
		lockFile:     lockFile,
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
	if s.activeFile, err = openSegmentAppend(s.active.path); err != nil {
		_ = lockFile.Close()
		return nil, err
	}
	go s.runSyncLoop()
	return s, nil
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
		sequence uint64
		offset   int64
		length   int64
		write    OccurrenceWrite
	}
	sequence := s.nextSequence
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
		eventRecord := record.Event{SchemaVersion: 1, Sequence: sequence, Kind: record.KindEvent, ID: occurrence.EventID.String(), ChannelID: channel.ID.String(), Type: record.Type{PackagePath: occurrence.EventType.PackagePath, Name: occurrence.EventType.Name}, Fields: occurrence.EventFields}
		frame, err := record.Encode(record.KindEvent, eventRecord, s.validator)
		if err != nil {
			return err
		}
		events = append(events, framedEvent{sequence: sequence, offset: int64(len(transaction)), length: int64(len(frame)), write: occurrence})
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
			return err
		}
	}
	start := s.active.size
	if err := writeAll(s.activeFile, transaction); err != nil {
		s.rollbackLocked(start, err)
		return err
	}
	s.active.size += int64(len(transaction))
	s.nextSequence = commit.Sequence + 1
	s.unsynced += int64(len(transaction))
	s.index.setChannel(channel.ID, channelSnapshot{typ: channel.Type, fields: lastChannelFields})
	for _, event := range events {
		s.index.add(&Occurrence{Sequence: event.sequence, EventID: event.write.EventID, ChannelID: channel.ID, Type: event.write.EventType, segment: s.active, offset: start + event.offset, length: event.length})
	}
	s.indexDirty = true
	if s.unsynced >= s.options.SyncBytes {
		s.syncLocked()
	}
	close(s.wake)
	s.wake = make(chan struct{})
	return nil
}

// rollbackLocked removes a partially written transaction so later appends
// never follow garbage. If truncation also fails the store stops accepting
// writes; recovery on the next start discards the incomplete transaction.
func (s *Store) rollbackLocked(start int64, cause error) {
	if err := truncateFile(s.activeFile, start); err != nil {
		s.writeFailure = fmt.Errorf("%v; rollback failed: %w", cause, err)
		return
	}
	if _, err := s.activeFile.Seek(start, io.SeekStart); err != nil {
		s.writeFailure = fmt.Errorf("%v; rollback seek failed: %w", cause, err)
	}
}

// rotateLocked finalizes the active segment at a transaction boundary, creates
// the next segment, and publishes it in the manifest. It may then compact.
func (s *Store) rotateLocked() error {
	if err := syncFile(s.activeFile); err != nil {
		return err
	}
	s.unsynced = 0
	id := s.manifest.NextSegmentID
	path := segmentPath(s.segmentsDir, id)
	next, err := createSegment(path)
	if err != nil {
		return err
	}
	updated := s.manifest
	updated.Segments = append(append([]string(nil), s.manifest.Segments...), segmentFileName(id))
	updated.NextSegmentID = id + 1
	if err := writeManifest(s.directory, updated); err != nil {
		_ = next.Close()
		_ = os.Remove(path)
		return err
	}
	if err := s.activeFile.Close(); err != nil {
		s.writeFailure = fmt.Errorf("close finalized segment: %w", err)
	}
	s.manifest = updated
	s.finalized = append(s.finalized, s.active)
	s.active = &segment{id: id, path: path}
	s.activeFile = next
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
	result, err := (&compaction{directory: s.directory, segmentsDir: s.segmentsDir, validator: s.validator}).run(s.manifest, s.finalized, s.active)
	if err != nil {
		return err
	}
	_ = s.activeFile.Close()
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
	s.activeFile = result.activeFile
	s.indexDirty = true
	return s.writeIndexLocked()
}

func (s *Store) syncLocked() {
	if s.unsynced == 0 {
		return
	}
	if err := syncFile(s.activeFile); err != nil {
		s.writeFailure = err
		return
	}
	s.unsynced = 0
}

func (s *Store) writeIndexLocked() error {
	return s.index.persisted(s.manifest.Generation, s.nextSequence-1).write(s.directory)
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

// ChannelOccurrence returns the position-th committed occurrence of a Channel.
// When none exists yet it returns ok=false and a wake channel closed on the
// next commit; callers must recheck after waking.
func (s *Store) ChannelOccurrence(channelID identity.UUID, position int) (Occurrence, bool, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	occurrence, ok := s.index.occurrence(channelID, position)
	if !ok {
		return Occurrence{}, false, s.wake
	}
	return *occurrence, true, nil
}

// ReadOccurrenceFrame reads the record bytes, without the newline, of the
// position-th occurrence of a Channel, pinning its segment so compaction
// cannot delete it mid-read.
func (s *Store) ReadOccurrenceFrame(channelID identity.UUID, position int) ([]byte, error) {
	s.mu.Lock()
	occurrence, ok := s.index.occurrence(channelID, position)
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("channel %s has no occurrence %d", channelID, position)
	}
	located := *occurrence
	reader, err := located.segment.readHandle()
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	located.segment.refs++
	s.mu.Unlock()
	defer s.unpin(located.segment)

	frame := make([]byte, located.length)
	if _, err := reader.ReadAt(frame, located.offset); err != nil {
		return nil, fmt.Errorf("read occurrence %d of %s: %w", located.Sequence, located.segment.path, err)
	}
	if len(frame) == 0 || frame[len(frame)-1] != '\n' {
		return nil, fmt.Errorf("occurrence %d is not a complete frame", located.Sequence)
	}
	return frame[:len(frame)-1], nil
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
	seg.closeReader()
	s.removeFileLocked(seg.path)
}

func (s *Store) removeFileLocked(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.pendingDeletes = append(s.pendingDeletes, path)
	}
}

func (s *Store) retryPendingDeletesLocked() {
	pending := s.pendingDeletes
	s.pendingDeletes = nil
	for _, path := range pending {
		s.removeFileLocked(path)
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
		for _, seg := range append(append([]*segment(nil), s.finalized...), s.active) {
			seg.closeReader()
		}
		err = errors.Join(s.activeFile.Close(), s.lockFile.Close())
	})
	return err
}
