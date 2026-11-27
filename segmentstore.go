package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const lockFileName = "lock"

// segmentStoreOptions are internal storage policy values. They are not public
// configuration; tests lower them to exercise rotation, compaction, and sync.
type segmentStoreOptions struct {
	targetSegmentSize   int64
	compactionThreshold int
	syncInterval        time.Duration
	syncBytes           int64
}

func defaultSegmentStoreOptions() segmentStoreOptions {
	return segmentStoreOptions{
		targetSegmentSize:   64 << 20,
		compactionThreshold: 8,
		syncInterval:        time.Second,
		syncBytes:           1 << 20,
	}
}

// segmentFile is one authoritative segment of the current generation. refs
// counts Readers currently reading it; a retired segment is deleted once its
// last pin is released.
type segmentFile struct {
	id      uint64
	path    string
	size    int64
	refs    int
	retired bool
	// reader is a shared read handle opened on first read; ReadAt is safe for
	// concurrent use. It is closed before the segment file is removed.
	reader *os.File
}

// indexEntry is the derived location of one committed Event occurrence.
type indexEntry struct {
	sequence  uint64
	eventID   uuid
	channelID uuid
	typ       typeKey
	segment   *segmentFile
	offset    int64
	length    int64
}

// persistedChannel is the latest persisted snapshot of a logical Channel.
type persistedChannel struct {
	typ    typeKey
	fields string
}

// channelWrite describes the Channel a transaction belongs to; occurrenceWrite
// is one Event occurrence with the Channel fields it was made with.
type channelWrite struct {
	id  uuid
	typ typeKey
}

type occurrenceWrite struct {
	channelFields map[string]interface{}
	eventID       uuid
	eventType     typeKey
	eventFields   map[string]interface{}
}

// segmentStore owns the authoritative append log: segments, manifest, the
// derived index, transaction writing, durability policy, and compaction.
type segmentStore struct {
	directory   string
	segmentsDir string
	validator   *recordSchemaValidator
	options     segmentStoreOptions
	lockFile    *os.File

	mu           sync.Mutex
	manifest     manifest
	finalized    []*segmentFile
	active       *segmentFile
	activeFile   *os.File
	nextSequence uint64
	unsynced     int64
	entries      []*indexEntry
	byChannel    map[uuid][]*indexEntry
	channels     map[uuid]persistedChannel
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

// appendTransaction atomically appends one transaction of a Channel's Event
// occurrences. Each occurrence is preceded by a Channel record when the Channel
// is new or its fields differ from the last Channel record written. Validation
// failures write nothing.
func (s *segmentStore) appendTransaction(channel channelWrite, occurrences []occurrenceWrite) error {
	if len(occurrences) == 0 {
		return errors.New("transaction has no occurrences")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeFailure != nil {
		return fmt.Errorf("storage is unwritable after an unrecoverable write failure: %w", s.writeFailure)
	}
	existing, known := s.channels[channel.id]
	if known && existing.typ != channel.typ {
		return fmt.Errorf("channel %s is persisted as %s, not %s", channel.id, existing.typ, channel.typ)
	}
	lastChannelFields := existing.fields
	if !known {
		lastChannelFields = ""
	}

	type framedEvent struct {
		sequence uint64
		offset   int64
		length   int64
		write    occurrenceWrite
	}
	sequence := s.nextSequence
	var frames [][]byte
	var events []framedEvent
	var transaction []byte
	for _, occurrence := range occurrences {
		channelFields, err := json.Marshal(occurrence.channelFields)
		if err != nil {
			return fmt.Errorf("encode channel fields: %w", err)
		}
		if string(channelFields) != lastChannelFields {
			record := channelRecord{SchemaVersion: 1, Sequence: sequence, Kind: recordKindChannel, ID: channel.id.String(), Type: recordType{PackagePath: channel.typ.pkgPath, Name: channel.typ.name}, Fields: occurrence.channelFields}
			frame, err := encodeRecord(recordKindChannel, record, s.validator)
			if err != nil {
				return err
			}
			frames = append(frames, frame)
			transaction = append(transaction, frame...)
			lastChannelFields = string(channelFields)
			sequence++
		}
		record := eventRecord{SchemaVersion: 1, Sequence: sequence, Kind: recordKindEvent, ID: occurrence.eventID.String(), ChannelID: channel.id.String(), Type: recordType{PackagePath: occurrence.eventType.pkgPath, Name: occurrence.eventType.name}, Fields: occurrence.eventFields}
		frame, err := encodeRecord(recordKindEvent, record, s.validator)
		if err != nil {
			return err
		}
		events = append(events, framedEvent{sequence: sequence, offset: int64(len(transaction)), length: int64(len(frame)), write: occurrence})
		frames = append(frames, frame)
		transaction = append(transaction, frame...)
		sequence++
	}
	commit := commitRecord{SchemaVersion: 1, Sequence: sequence, Kind: recordKindCommit, TransactionID: s.nextSequence, RecordCount: uint64(len(frames)), Checksum: transactionChecksum(frames)}
	commitFrame, err := encodeRecord(recordKindCommit, commit, s.validator)
	if err != nil {
		return err
	}
	transaction = append(transaction, commitFrame...)

	if s.active.size > 0 && s.active.size+int64(len(transaction)) > s.options.targetSegmentSize {
		if err := s.rotateLocked(); err != nil {
			return err
		}
	}
	start := s.active.size
	if err := storageWriteAll(s.activeFile, transaction); err != nil {
		s.rollbackLocked(start, err)
		return err
	}
	s.active.size += int64(len(transaction))
	s.nextSequence = commit.Sequence + 1
	s.unsynced += int64(len(transaction))
	s.channels[channel.id] = persistedChannel{typ: channel.typ, fields: lastChannelFields}
	for _, event := range events {
		entry := &indexEntry{sequence: event.sequence, eventID: event.write.eventID, channelID: channel.id, typ: event.write.eventType, segment: s.active, offset: start + event.offset, length: event.length}
		s.entries = append(s.entries, entry)
		s.byChannel[channel.id] = append(s.byChannel[channel.id], entry)
	}
	s.indexDirty = true
	if s.unsynced >= s.options.syncBytes {
		s.syncLocked()
	}
	close(s.wake)
	s.wake = make(chan struct{})
	return nil
}

// rollbackLocked removes a partially written transaction so later appends
// never follow garbage. If truncation also fails the store stops accepting
// writes; recovery on the next start discards the incomplete transaction.
func (s *segmentStore) rollbackLocked(start int64, cause error) {
	if err := storageTruncate(s.activeFile, start); err != nil {
		s.writeFailure = fmt.Errorf("%v; rollback failed: %w", cause, err)
		return
	}
	if _, err := s.activeFile.Seek(start, io.SeekStart); err != nil {
		s.writeFailure = fmt.Errorf("%v; rollback seek failed: %w", cause, err)
	}
}

// rotateLocked finalizes the active segment at a transaction boundary, creates
// the next segment, and publishes it in the manifest. It may then compact.
func (s *segmentStore) rotateLocked() error {
	if err := storageSync(s.activeFile); err != nil {
		return err
	}
	s.unsynced = 0
	id := s.manifest.NextSegmentID
	path := storageSegmentPath(s.segmentsDir, id)
	next, err := storageCreateSegment(path)
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
	s.active = &segmentFile{id: id, path: path}
	s.activeFile = next
	if len(s.finalized) >= s.options.compactionThreshold {
		if err := s.compactLocked(); err != nil {
			return fmt.Errorf("compact segments: %w", err)
		}
	}
	return nil
}

func (s *segmentStore) syncLocked() {
	if s.unsynced == 0 {
		return
	}
	if err := storageSync(s.activeFile); err != nil {
		s.writeFailure = err
		return
	}
	s.unsynced = 0
}

// runSyncLoop applies the periodic durability policy and persists the derived
// index when it has changed.
func (s *segmentStore) runSyncLoop() {
	defer close(s.stopped)
	ticker := time.NewTicker(s.options.syncInterval)
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

// channelOccurrence returns the position-th committed occurrence of a Channel.
// When none exists yet it returns ok=false and a wake channel closed on the
// next commit; callers must recheck after waking.
func (s *segmentStore) channelOccurrence(channelID uuid, position int) (indexEntry, bool, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.byChannel[channelID]
	if position >= len(entries) {
		return indexEntry{}, false, s.wake
	}
	return *entries[position], true, nil
}

// readOccurrenceFrame reads the record bytes for the position-th occurrence of
// a Channel, pinning its segment so compaction cannot delete it mid-read.
func (s *segmentStore) readOccurrenceFrame(channelID uuid, position int) ([]byte, error) {
	s.mu.Lock()
	entries := s.byChannel[channelID]
	if position >= len(entries) {
		s.mu.Unlock()
		return nil, fmt.Errorf("channel %s has no occurrence %d", channelID, position)
	}
	entry := *entries[position]
	segment := entry.segment
	if segment.reader == nil {
		f, err := storageOpenSegmentRead(segment.path)
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		segment.reader = f
	}
	segment.refs++
	reader := segment.reader
	s.mu.Unlock()
	defer s.unpin(segment)

	frame := make([]byte, entry.length)
	if _, err := reader.ReadAt(frame, entry.offset); err != nil {
		return nil, fmt.Errorf("read occurrence %d of %s: %w", entry.sequence, entry.segment.path, err)
	}
	if len(frame) == 0 || frame[len(frame)-1] != '\n' {
		return nil, fmt.Errorf("occurrence %d is not a complete frame", entry.sequence)
	}
	return frame[:len(frame)-1], nil
}

func (s *segmentStore) unpin(segment *segmentFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	segment.refs--
	if segment.retired && segment.refs == 0 {
		s.removeRetiredLocked(segment)
	}
}

// removeRetiredLocked deletes a retired segment, queueing it for retry when
// removal fails. A leftover file is never authoritative: it is outside the
// manifest, and recovery removes such orphans after validation.
func (s *segmentStore) removeRetiredLocked(segment *segmentFile) {
	if segment.reader != nil {
		_ = segment.reader.Close()
		segment.reader = nil
	}
	s.removeFileLocked(segment.path)
}

func (s *segmentStore) removeFileLocked(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.pendingDeletes = append(s.pendingDeletes, path)
	}
}

func (s *segmentStore) retryPendingDeletesLocked() {
	pending := s.pendingDeletes
	s.pendingDeletes = nil
	for _, path := range pending {
		s.removeFileLocked(path)
	}
}

// channelHistoryTypes returns the distinct Event types in a Channel's committed
// history, in first-occurrence order.
func (s *segmentStore) channelHistoryTypes(channelID uuid) []typeKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[typeKey]bool)
	var types []typeKey
	for _, entry := range s.byChannel[channelID] {
		if !seen[entry.typ] {
			seen[entry.typ] = true
			types = append(types, entry.typ)
		}
	}
	return types
}

// close stops background work and releases files. It is test-only: the
// production runtime is process-lifetime scoped and is never closed.
func (s *segmentStore) close() error {
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
		for _, segment := range append(append([]*segmentFile(nil), s.finalized...), s.active) {
			if segment.reader != nil {
				_ = segment.reader.Close()
				segment.reader = nil
			}
		}
		err = errors.Join(s.activeFile.Close(), s.lockFile.Close())
	})
	return err
}

func (s *segmentStore) lockPath() string {
	return filepath.Join(s.directory, lockFileName)
}
