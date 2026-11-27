package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gunesis-game/eventlog/internal/fieldschema"
	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
	"github.com/gunesis-game/eventlog/internal/record"
)

// errCorruptStorage marks authoritative storage that cannot be recovered
// without guessing. Recovery fails closed rather than skipping data.
var errCorruptStorage = errors.New("corrupt storage")

// recovery rebuilds the store's state from the storage directory: it
// validates the manifest, every segment, transaction, and sequence; truncates
// an incomplete trailing transaction; rebuilds the derived index; and only then
// cleans orphaned artifacts. A read-only recovery writes nothing: it ignores an
// incomplete trailing transaction instead of truncating it, and keeps a
// rebuilt manifest and index in memory only.
type recovery struct {
	files     *filesystem.Client
	validator *record.Validator
	readOnly  bool
	// empty marks fresh storage opened read-only: no manifest or segment
	// exists, and none may be created.
	empty        bool
	index        *index
	nextSequence uint64
	manifest     manifest
	finalized    []*segment
	active       *segment
	indexRebuilt bool
}

func (r *recovery) run() (*recovery, error) {
	m, err := r.resolveManifest()
	if err != nil {
		return nil, err
	}
	r.manifest = m
	if r.empty {
		r.active = &segment{id: 1, path: segmentPath(1)}
		return r, nil
	}
	for i, name := range m.Segments {
		if err := r.recoverSegment(name, i == len(m.Segments)-1); err != nil {
			return nil, err
		}
	}
	if r.readOnly {
		return r, nil
	}
	if r.indexRebuilt, err = r.index.persisted(r.manifest.Generation, r.nextSequence-1).reconcile(r.files); err != nil {
		return nil, err
	}
	r.cleanOrphans()
	return r, nil
}

// resolveManifest returns the authoritative manifest. A fresh directory gets a
// new generation. A missing or unreadable manifest is reconstructed only when
// the segment files form exactly one unambiguous contiguous history.
func (r *recovery) resolveManifest() (manifest, error) {
	m, err := readManifest(r.files)
	if err == nil {
		return m, nil
	}
	missing := errors.Is(err, fs.ErrNotExist)
	ids, listErr := r.listSegmentIDs()
	if listErr != nil {
		return manifest{}, listErr
	}
	if len(ids) == 0 {
		if !missing {
			return manifest{}, fmt.Errorf("%w: manifest is unusable and no segments exist: %v", errCorruptStorage, err)
		}
		fresh := manifest{FormatVersion: manifestFormatVersion, Generation: 1, Segments: []string{segmentFileName(1)}, NextSegmentID: 2}
		if r.readOnly {
			r.empty = true
			return fresh, nil
		}
		if err := r.files.CreateExclusive(segmentPath(1)); err != nil {
			return manifest{}, err
		}
		return fresh, writeManifest(r.files, fresh)
	}
	rebuilt := manifest{FormatVersion: manifestFormatVersion, Generation: 1, NextSegmentID: ids[len(ids)-1] + 1}
	for _, id := range ids {
		rebuilt.Segments = append(rebuilt.Segments, segmentFileName(id))
	}
	if chainErr := r.checkUnambiguousChain(ids); chainErr != nil {
		return manifest{}, fmt.Errorf("%w: manifest unusable (%v) and segments are ambiguous: %v", errCorruptStorage, err, chainErr)
	}
	if r.readOnly {
		return rebuilt, nil
	}
	return rebuilt, writeManifest(r.files, rebuilt)
}

func (r *recovery) listSegmentIDs() ([]uint64, error) {
	entries, err := r.files.List(segmentsDirName)
	if errors.Is(err, fs.ErrNotExist) && r.readOnly {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	var ids []uint64
	for _, entry := range entries {
		if entry.Dir || strings.HasSuffix(entry.Name, temporaryFileSuffix) {
			continue
		}
		id, err := segmentIDFromName(entry.Name)
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
func (r *recovery) checkUnambiguousChain(ids []uint64) error {
	expected := uint64(1)
	for i, id := range ids {
		data, err := r.files.Read(segmentPath(id), 0, -1)
		if err != nil {
			return err
		}
		result, err := record.ValidateStream(data, r.validator, expected)
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
func (r *recovery) recoverSegment(name string, last bool) error {
	id, err := segmentIDFromName(name)
	if err != nil {
		return err
	}
	path := segmentPath(id)
	data, err := r.files.Read(path, 0, -1)
	if err != nil {
		return fmt.Errorf("%w: manifest segment %s: %v", errCorruptStorage, name, err)
	}
	result, err := record.ValidateStream(data, r.validator, r.nextSequence)
	if err != nil {
		return fmt.Errorf("%w: segment %s: %v", errCorruptStorage, name, err)
	}
	seg := &segment{id: id, path: path, size: int64(len(data))}
	if result.IncompleteStart != nil {
		if !last {
			return fmt.Errorf("%w: finalized segment %s ends with an incomplete transaction", errCorruptStorage, name)
		}
		if err := r.truncate(path, *result.IncompleteStart); err != nil {
			return err
		}
		data = data[:*result.IncompleteStart]
		seg.size = *result.IncompleteStart
	}
	if err := r.indexSegment(seg, data); err != nil {
		return fmt.Errorf("%w: segment %s: %v", errCorruptStorage, name, err)
	}
	r.nextSequence = result.CommittedSequence + 1
	if last {
		r.active = seg
	} else {
		r.finalized = append(r.finalized, seg)
	}
	return nil
}

func (r *recovery) truncate(path string, size int64) error {
	if r.readOnly {
		return nil
	}
	if err := r.files.Truncate(path, size); err != nil {
		return err
	}
	return r.files.Sync(path)
}

// indexSegment applies the semantic checks the stream validator does not:
// valid UTF-8, value ranges, Channel references, and Channel type stability.
// data contains only complete, committed transactions.
func (r *recovery) indexSegment(seg *segment, data []byte) error {
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
		if err := r.indexRecord(seg, raw, offset); err != nil {
			return fmt.Errorf("record at byte offset %d: %w", offset, err)
		}
		offset = end + 1
	}
	return nil
}

func (r *recovery) indexRecord(seg *segment, raw []byte, offset int64) error {
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return err
	}
	switch header.Kind {
	case record.KindChannel:
		var channel record.Channel
		if err := json.Unmarshal(raw, &channel); err != nil {
			return err
		}
		id, err := identity.ParseUUID(channel.ID)
		if err != nil {
			return err
		}
		if err := validateRecordFields(channel.Fields); err != nil {
			return err
		}
		typ := fieldschema.TypeKey{PackagePath: channel.Type.PackagePath, Name: channel.Type.Name}
		if existing, ok := r.index.channel(id); ok && existing.typ != typ {
			return fmt.Errorf("channel %s changes type from %s to %s", id, existing.typ, typ)
		}
		fields, err := canonicalRecordFields(channel.Fields)
		if err != nil {
			return err
		}
		r.index.setChannel(id, channelSnapshot{typ: typ, fields: fields})
	case record.KindEvent:
		var event record.Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return err
		}
		id, err := identity.ParseUUID(event.ID)
		if err != nil {
			return err
		}
		channelID, err := identity.ParseUUID(event.ChannelID)
		if err != nil {
			return err
		}
		if _, ok := r.index.channel(channelID); !ok {
			return fmt.Errorf("event %s references unknown channel %s", id, channelID)
		}
		if err := validateRecordFields(event.Fields); err != nil {
			return err
		}
		r.index.add(&Occurrence{Sequence: event.Sequence, EventID: id, ChannelID: channelID, Type: fieldschema.TypeKey{PackagePath: event.Type.PackagePath, Name: event.Type.Name}, segment: seg, offset: offset, length: int64(len(raw)) + 1})
	}
	return nil
}

func validateRecordFields(fields map[string]interface{}) error {
	for name, raw := range fields {
		value, err := fieldschema.ValueFromRecord(raw)
		if err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
		if err := fieldschema.ValidatePersisted(value); err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
	}
	return nil
}

// canonicalRecordFields re-encodes decoded fields in the same canonical form
// AppendTransaction uses, so persisted and live Channel snapshots compare equal.
func canonicalRecordFields(fields map[string]interface{}) (string, error) {
	canonical := make(map[string]interface{}, len(fields))
	for name, raw := range fields {
		value, err := fieldschema.ValueFromRecord(raw)
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
func (r *recovery) cleanOrphans() {
	listed := make(map[string]bool, len(r.manifest.Segments))
	for _, name := range r.manifest.Segments {
		listed[name] = true
	}
	for _, dir := range []string{"", segmentsDirName} {
		entries, err := r.files.List(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Dir {
				continue
			}
			orphanSegment := dir == segmentsDirName && segmentFileNamePattern.MatchString(entry.Name) && !listed[entry.Name]
			if strings.HasSuffix(entry.Name, temporaryFileSuffix) || orphanSegment {
				_ = r.files.Remove(path.Join(dir, entry.Name))
			}
		}
	}
}
