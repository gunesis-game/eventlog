package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// errCorruptStorage marks authoritative storage that cannot be recovered
// without guessing. Recovery fails closed rather than skipping data.
var errCorruptStorage = errors.New("corrupt storage")

// openSegmentStore creates or recovers the storage directory and returns a
// store ready for appends. Recovery validates the manifest, every segment,
// transaction, and sequence; truncates an incomplete trailing transaction;
// rebuilds the derived index; and only then cleans orphaned artifacts.
func openSegmentStore(directory string, options segmentStoreOptions) (*segmentStore, error) {
	if err := storageCreateDirectory(directory); err != nil {
		return nil, err
	}
	segmentsDir := filepath.Join(directory, segmentsDirName)
	if err := storageCreateDirectory(segmentsDir); err != nil {
		return nil, err
	}
	validator, err := newRecordSchemaValidator()
	if err != nil {
		return nil, err
	}
	s := &segmentStore{
		directory:    directory,
		segmentsDir:  segmentsDir,
		validator:    validator,
		options:      options,
		nextSequence: 1,
		byChannel:    make(map[uuid][]*indexEntry),
		channels:     make(map[uuid]persistedChannel),
		wake:         make(chan struct{}),
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
	}
	if s.lockFile, err = acquireStorageLock(s.lockPath()); err != nil {
		return nil, err
	}
	if err := s.recover(); err != nil {
		_ = s.lockFile.Close()
		if s.activeFile != nil {
			_ = s.activeFile.Close()
		}
		return nil, err
	}
	go s.runSyncLoop()
	return s, nil
}

func (s *segmentStore) recover() error {
	m, err := s.resolveManifest()
	if err != nil {
		return err
	}
	s.manifest = m
	for i, name := range m.Segments {
		last := i == len(m.Segments)-1
		if err := s.recoverSegment(name, last); err != nil {
			return err
		}
	}
	s.mu.Lock()
	err = s.reconcileIndexLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.cleanOrphans()
	s.activeFile, err = storageOpenSegmentAppend(s.active.path)
	return err
}

// resolveManifest returns the authoritative manifest. A fresh directory gets a
// new generation. A missing or unreadable manifest is reconstructed only when
// the segment files form exactly one unambiguous contiguous history.
func (s *segmentStore) resolveManifest() (manifest, error) {
	m, err := readManifest(s.directory)
	if err == nil {
		return m, nil
	}
	missing := errors.Is(err, fs.ErrNotExist)
	ids, listErr := s.listSegmentIDs()
	if listErr != nil {
		return manifest{}, listErr
	}
	if len(ids) == 0 {
		if !missing {
			return manifest{}, fmt.Errorf("%w: manifest is unusable and no segments exist: %v", errCorruptStorage, err)
		}
		fresh := manifest{FormatVersion: manifestFormatVersion, Generation: 1, Segments: []string{segmentFileName(1)}, NextSegmentID: 2}
		f, createErr := storageCreateSegment(storageSegmentPath(s.segmentsDir, 1))
		if createErr != nil {
			return manifest{}, createErr
		}
		if closeErr := f.Close(); closeErr != nil {
			return manifest{}, closeErr
		}
		return fresh, writeManifest(s.directory, fresh)
	}
	rebuilt := manifest{FormatVersion: manifestFormatVersion, Generation: 1, NextSegmentID: ids[len(ids)-1] + 1}
	for _, id := range ids {
		rebuilt.Segments = append(rebuilt.Segments, segmentFileName(id))
	}
	if chainErr := s.checkUnambiguousChain(ids); chainErr != nil {
		return manifest{}, fmt.Errorf("%w: manifest unusable (%v) and segments are ambiguous: %v", errCorruptStorage, err, chainErr)
	}
	return rebuilt, writeManifest(s.directory, rebuilt)
}

func (s *segmentStore) listSegmentIDs() ([]uint64, error) {
	entries, err := os.ReadDir(s.segmentsDir)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	var ids []uint64
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), temporaryFileSuffix) {
			continue
		}
		id, err := segmentIDFromName(entry.Name())
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids, nil
}

// checkUnambiguousChain verifies that the segment files, in ID order, form one
// contiguous global sequence from 1. Overlapping generations (for example the
// sources and output of an interrupted compaction) are ambiguous.
func (s *segmentStore) checkUnambiguousChain(ids []uint64) error {
	expected := uint64(1)
	for i, id := range ids {
		data, err := os.ReadFile(storageSegmentPath(s.segmentsDir, id))
		if err != nil {
			return err
		}
		result, err := validateRecordStream(data, s.validator, expected)
		if err != nil {
			return fmt.Errorf("segment %d: %w", id, err)
		}
		if result.IncompleteStart != nil && i != len(ids)-1 {
			return fmt.Errorf("segment %d has an incomplete transaction but is not last", id)
		}
		expected = result.CommittedSequence + 1
	}
	return nil
}

// recoverSegment validates one manifest segment and indexes its committed
// occurrences. Only the last (active) segment may end with an incomplete
// transaction, which is truncated from its start.
func (s *segmentStore) recoverSegment(name string, last bool) error {
	id, err := segmentIDFromName(name)
	if err != nil {
		return err
	}
	path := storageSegmentPath(s.segmentsDir, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: manifest segment %s: %v", errCorruptStorage, name, err)
	}
	result, err := validateRecordStream(data, s.validator, s.nextSequence)
	if err != nil {
		return fmt.Errorf("%w: segment %s: %v", errCorruptStorage, name, err)
	}
	segment := &segmentFile{id: id, path: path, size: int64(len(data))}
	if result.IncompleteStart != nil {
		if !last {
			return fmt.Errorf("%w: finalized segment %s ends with an incomplete transaction", errCorruptStorage, name)
		}
		if err := truncateSegment(path, *result.IncompleteStart); err != nil {
			return err
		}
		data = data[:*result.IncompleteStart]
		segment.size = *result.IncompleteStart
	}
	if err := s.indexSegment(segment, data); err != nil {
		return fmt.Errorf("%w: segment %s: %v", errCorruptStorage, name, err)
	}
	s.nextSequence = result.CommittedSequence + 1
	if last {
		s.active = segment
	} else {
		s.finalized = append(s.finalized, segment)
	}
	return nil
}

func truncateSegment(path string, size int64) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open segment for truncation: %w", err)
	}
	if err := storageTruncate(f, size); err != nil {
		_ = f.Close()
		return err
	}
	if err := storageSync(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// indexSegment applies the semantic checks the stream validator does not:
// valid UTF-8, value ranges, Channel references, and Channel type stability.
// data contains only complete, committed transactions.
func (s *segmentStore) indexSegment(segment *segmentFile, data []byte) error {
	offset := int64(0)
	for offset < int64(len(data)) {
		end := offset
		for data[end] != '\n' {
			end++
		}
		raw := data[offset:end]
		if !utf8.Valid(raw) {
			return fmt.Errorf("record at byte offset %d is not valid UTF-8", offset)
		}
		if err := s.indexRecord(segment, raw, offset); err != nil {
			return fmt.Errorf("record at byte offset %d: %w", offset, err)
		}
		offset = end + 1
	}
	return nil
}

func (s *segmentStore) indexRecord(segment *segmentFile, raw []byte, offset int64) error {
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return err
	}
	switch header.Kind {
	case recordKindChannel:
		var record channelRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		id, err := parseUUID(record.ID)
		if err != nil {
			return err
		}
		if err := validateRecordFields(record.Fields); err != nil {
			return err
		}
		typ := typeKey{pkgPath: record.Type.PackagePath, name: record.Type.Name}
		if existing, ok := s.channels[id]; ok && existing.typ != typ {
			return fmt.Errorf("channel %s changes type from %s to %s", id, existing.typ, typ)
		}
		fields, err := canonicalRecordFields(record.Fields)
		if err != nil {
			return err
		}
		s.channels[id] = persistedChannel{typ: typ, fields: fields}
	case recordKindEvent:
		var record eventRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		id, err := parseUUID(record.ID)
		if err != nil {
			return err
		}
		channelID, err := parseUUID(record.ChannelID)
		if err != nil {
			return err
		}
		if _, ok := s.channels[channelID]; !ok {
			return fmt.Errorf("event %s references unknown channel %s", id, channelID)
		}
		if err := validateRecordFields(record.Fields); err != nil {
			return err
		}
		entry := &indexEntry{sequence: record.Sequence, eventID: id, channelID: channelID, typ: typeKey{pkgPath: record.Type.PackagePath, name: record.Type.Name}, segment: segment, offset: offset, length: int64(len(raw)) + 1}
		s.entries = append(s.entries, entry)
		s.byChannel[channelID] = append(s.byChannel[channelID], entry)
	}
	return nil
}

func validateRecordFields(fields map[string]interface{}) error {
	for name, raw := range fields {
		value, err := fieldValueFromRecord(raw)
		if err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
		if err := validatePersistedValue(value); err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
	}
	return nil
}

// canonicalRecordFields re-encodes decoded fields in the same canonical form
// appendTransaction uses, so persisted and live Channel snapshots compare equal.
func canonicalRecordFields(fields map[string]interface{}) (string, error) {
	canonical := make(map[string]interface{}, len(fields))
	for name, raw := range fields {
		value, err := fieldValueFromRecord(raw)
		if err != nil {
			return "", err
		}
		canonical[name] = value
	}
	encoded, err := json.Marshal(canonical)
	return string(encoded), err
}

// cleanOrphans removes temporary files and segments outside the manifest. It
// runs only after every manifest segment has validated.
func (s *segmentStore) cleanOrphans() {
	listed := make(map[string]bool, len(s.manifest.Segments))
	for _, name := range s.manifest.Segments {
		listed[name] = true
	}
	for _, dir := range []string{s.directory, s.segmentsDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				continue
			}
			orphanSegment := dir == s.segmentsDir && segmentFileNamePattern.MatchString(name) && !listed[name]
			if strings.HasSuffix(name, temporaryFileSuffix) || orphanSegment {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
	}
}
