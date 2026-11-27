package storage

import (
	"bytes"
	"fmt"

	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/record"
)

// compaction consolidates every finalized segment into one replacement
// segment. Records are copied byte-for-byte, so every sequence, transaction,
// and checksum is preserved; compaction never removes history.
//
// Protocol: build a temporary file, sync, validate it, rename it into place,
// create a fresh active segment, and atomically switch the manifest to a new
// generation. A crash before the manifest switch leaves the old generation
// authoritative and the new files as orphans; a crash after it leaves the old
// files as orphans. Recovery cleans orphans only after the manifest's segments
// validate.
//
// Compaction runs right after a rotation, while the active segment is empty.
// The empty active segment is reissued with the next ID so manifest segment
// IDs stay ascending in sequence order.
type compaction struct {
	files     *filesystem.Client
	validator *record.Validator
}

type compactionResult struct {
	manifest    manifest
	replacement *segment
	bases       map[*segment]int64
	active      *segment
}

func (c *compaction) run(current manifest, sources []*segment, active *segment) (compactionResult, error) {
	if active.size != 0 {
		return compactionResult{}, fmt.Errorf("compaction requires an empty active segment")
	}
	id := current.NextSegmentID
	finalPath := segmentPath(id)
	temporaryPath := finalPath + temporaryFileSuffix
	bases, total, err := c.consolidate(temporaryPath, sources)
	if err != nil {
		_ = c.files.Remove(temporaryPath)
		return compactionResult{}, err
	}
	if err := c.validate(temporaryPath, c.firstSequenceOf(sources[0])); err != nil {
		_ = c.files.Remove(temporaryPath)
		return compactionResult{}, err
	}
	if err := c.files.Rename(temporaryPath, finalPath); err != nil {
		_ = c.files.Remove(temporaryPath)
		return compactionResult{}, fmt.Errorf("finalize compacted segment: %w", err)
	}
	activeID := id + 1
	activePath := segmentPath(activeID)
	if err := c.files.CreateExclusive(activePath); err != nil {
		_ = c.files.Remove(finalPath)
		return compactionResult{}, err
	}
	updated := manifest{
		FormatVersion: manifestFormatVersion,
		Generation:    current.Generation + 1,
		Segments:      []string{segmentFileName(id), segmentFileName(activeID)},
		NextSegmentID: activeID + 1,
	}
	if err := writeManifest(c.files, updated); err != nil {
		_ = c.files.Remove(activePath)
		_ = c.files.Remove(finalPath)
		return compactionResult{}, err
	}
	return compactionResult{
		manifest:    updated,
		replacement: &segment{id: id, path: finalPath, size: total},
		bases:       bases,
		active:      &segment{id: activeID, path: activePath},
	}, nil
}

// consolidate concatenates the source segments into path and returns each
// source's byte offset within the result and the total size.
func (c *compaction) consolidate(path string, sources []*segment) (map[*segment]int64, int64, error) {
	if err := c.files.CreateExclusive(path); err != nil {
		return nil, 0, err
	}
	bases := make(map[*segment]int64, len(sources))
	var offset int64
	for _, source := range sources {
		bases[source] = offset
		data, err := c.files.Read(source.path, 0, -1)
		if err != nil {
			return nil, 0, err
		}
		if int64(len(data)) != source.size {
			return nil, 0, fmt.Errorf("segment %s holds %d bytes, want %d", source.path, len(data), source.size)
		}
		if offset, err = c.files.Append(path, data); err != nil {
			return nil, 0, err
		}
	}
	return bases, offset, c.files.Sync(path)
}

func (c *compaction) validate(path string, firstSequence uint64) error {
	data, err := c.files.Read(path, 0, -1)
	if err != nil {
		return err
	}
	result, err := record.ValidateStream(data, c.validator, firstSequence)
	if err != nil {
		return fmt.Errorf("validate compacted segment: %w", err)
	}
	if result.IncompleteStart != nil || result.CommittedOffset != int64(len(data)) {
		return fmt.Errorf("compacted segment is not a sequence of complete transactions")
	}
	return nil
}

// firstSequenceOf returns the first global sequence stored in a segment by
// decoding its first record.
func (c *compaction) firstSequenceOf(seg *segment) uint64 {
	data, err := c.files.Read(seg.path, 0, -1)
	if err != nil {
		return 0
	}
	frame, err := record.NewFramer(bytes.NewReader(data)).Next()
	if err != nil {
		return 0
	}
	decoded, err := record.Decode(frame, c.validator)
	if err != nil {
		return 0
	}
	sequence, _, _ := record.SequenceAndKind(decoded)
	return sequence
}
