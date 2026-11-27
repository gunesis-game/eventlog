package storage

import (
	"fmt"
	"io"
	"os"

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
	directory   string
	segmentsDir string
	validator   *record.Validator
}

type compactionResult struct {
	manifest    manifest
	replacement *segment
	bases       map[*segment]int64
	active      *segment
	activeFile  *os.File
}

func (c *compaction) run(current manifest, sources []*segment, active *segment) (compactionResult, error) {
	if active.size != 0 {
		return compactionResult{}, fmt.Errorf("compaction requires an empty active segment")
	}
	id := current.NextSegmentID
	finalPath := segmentPath(c.segmentsDir, id)
	temporaryPath := finalPath + temporaryFileSuffix
	bases, total, err := consolidate(temporaryPath, sources)
	if err != nil {
		_ = os.Remove(temporaryPath)
		return compactionResult{}, err
	}
	if err := c.validate(temporaryPath, firstSequenceOf(sources[0], c.validator)); err != nil {
		_ = os.Remove(temporaryPath)
		return compactionResult{}, err
	}
	if err := renameFile(temporaryPath, finalPath); err != nil {
		_ = os.Remove(temporaryPath)
		return compactionResult{}, fmt.Errorf("finalize compacted segment: %w", err)
	}
	activeID := id + 1
	activePath := segmentPath(c.segmentsDir, activeID)
	activeFile, err := createSegment(activePath)
	if err != nil {
		_ = os.Remove(finalPath)
		return compactionResult{}, err
	}
	if err := syncDirectory(c.segmentsDir); err != nil {
		_ = activeFile.Close()
		return compactionResult{}, err
	}
	updated := manifest{
		FormatVersion: manifestFormatVersion,
		Generation:    current.Generation + 1,
		Segments:      []string{segmentFileName(id), segmentFileName(activeID)},
		NextSegmentID: activeID + 1,
	}
	if err := writeManifest(c.directory, updated); err != nil {
		_ = activeFile.Close()
		_ = os.Remove(activePath)
		_ = os.Remove(finalPath)
		return compactionResult{}, err
	}
	return compactionResult{
		manifest:    updated,
		replacement: &segment{id: id, path: finalPath, size: total},
		bases:       bases,
		active:      &segment{id: activeID, path: activePath},
		activeFile:  activeFile,
	}, nil
}

// consolidate concatenates the source segments into path and returns each
// source's byte offset within the result and the total size.
func consolidate(path string, sources []*segment) (map[*segment]int64, int64, error) {
	out, err := createSegment(path)
	if err != nil {
		return nil, 0, err
	}
	bases := make(map[*segment]int64, len(sources))
	var offset int64
	for _, source := range sources {
		bases[source] = offset
		in, err := openSegmentRead(source.path)
		if err != nil {
			_ = out.Close()
			return nil, 0, err
		}
		copied, err := io.Copy(out, in)
		_ = in.Close()
		if err != nil || copied != source.size {
			_ = out.Close()
			return nil, 0, fmt.Errorf("copy segment %s: copied %d of %d bytes: %v", source.path, copied, source.size, err)
		}
		offset += copied
	}
	if err := syncFile(out); err != nil {
		_ = out.Close()
		return nil, 0, err
	}
	return bases, offset, out.Close()
}

func (c *compaction) validate(path string, firstSequence uint64) error {
	data, err := os.ReadFile(path)
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
// reading its first record.
func firstSequenceOf(seg *segment, validator *record.Validator) uint64 {
	f, err := openSegmentRead(seg.path)
	if err != nil {
		return 0
	}
	defer f.Close()
	frame, err := record.NewFramer(f).Next()
	if err != nil {
		return 0
	}
	decoded, err := record.Decode(frame, validator)
	if err != nil {
		return 0
	}
	sequence, _, _ := record.SequenceAndKind(decoded)
	return sequence
}
