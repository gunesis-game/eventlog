package eventlog

import (
	"fmt"
	"io"
	"os"
)

// compactLocked consolidates every finalized segment into one replacement
// segment. Records are copied byte-for-byte, so every sequence, transaction,
// and checksum is preserved; compaction never removes history.
//
// Protocol: build a temporary file, sync, validate it, rename it into place,
// atomically switch the manifest to a new generation, repoint the in-memory
// index, then delete old segments once no Reader pins them. A crash before the
// manifest switch leaves the old generation authoritative and the new file an
// orphan; a crash after it leaves the old files as orphans. Recovery cleans
// orphans only after the manifest's segments validate.
//
// Compaction runs right after a rotation, while the active segment is empty.
// The empty active segment is reissued with the next ID so manifest segment IDs
// stay ascending in sequence order.
func (s *segmentStore) compactLocked() error {
	sources := s.finalized
	if len(sources) < 2 {
		return nil
	}
	if s.active.size != 0 {
		return fmt.Errorf("compaction requires an empty active segment")
	}
	id := s.manifest.NextSegmentID
	finalPath := storageSegmentPath(s.segmentsDir, id)
	temporaryPath := finalPath + temporaryFileSuffix
	bases, err := writeConsolidatedSegment(temporaryPath, sources)
	if err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	firstSequence := s.firstSequenceOf(sources[0])
	if err := s.validateConsolidated(temporaryPath, firstSequence); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	if err := storageRename(temporaryPath, finalPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("finalize compacted segment: %w", err)
	}
	activeID := id + 1
	activePath := storageSegmentPath(s.segmentsDir, activeID)
	activeFile, err := storageCreateSegment(activePath)
	if err != nil {
		_ = os.Remove(finalPath)
		return err
	}
	if err := storageSyncDirectory(s.segmentsDir); err != nil {
		_ = activeFile.Close()
		return err
	}
	updated := manifest{
		FormatVersion: manifestFormatVersion,
		Generation:    s.manifest.Generation + 1,
		Segments:      []string{segmentFileName(id), segmentFileName(activeID)},
		NextSegmentID: activeID + 1,
	}
	if err := writeManifest(s.directory, updated); err != nil {
		_ = activeFile.Close()
		_ = os.Remove(activePath)
		_ = os.Remove(finalPath)
		return err
	}
	retiredActive := s.active
	_ = s.activeFile.Close()
	s.removeRetiredLocked(retiredActive)
	s.active = &segmentFile{id: activeID, path: activePath}
	s.activeFile = activeFile
	var total int64
	for _, source := range sources {
		total += source.size
	}
	replacement := &segmentFile{id: id, path: finalPath, size: total}
	for _, entry := range s.entries {
		if base, ok := bases[entry.segment]; ok {
			entry.offset += base
			entry.segment = replacement
		}
	}
	for _, source := range sources {
		source.retired = true
		if source.refs == 0 {
			s.removeRetiredLocked(source)
		}
	}
	s.manifest = updated
	s.finalized = []*segmentFile{replacement}
	s.indexDirty = true
	return s.writeIndexLocked()
}

// writeConsolidatedSegment concatenates the source segments into path and
// returns each source's byte offset within the result.
func writeConsolidatedSegment(path string, sources []*segmentFile) (map[*segmentFile]int64, error) {
	out, err := storageCreateSegment(path)
	if err != nil {
		return nil, err
	}
	bases := make(map[*segmentFile]int64, len(sources))
	var offset int64
	for _, source := range sources {
		bases[source] = offset
		in, err := storageOpenSegmentRead(source.path)
		if err != nil {
			_ = out.Close()
			return nil, err
		}
		copied, err := io.Copy(out, in)
		_ = in.Close()
		if err != nil || copied != source.size {
			_ = out.Close()
			return nil, fmt.Errorf("copy segment %s: copied %d of %d bytes: %v", source.path, copied, source.size, err)
		}
		offset += copied
	}
	if err := storageSync(out); err != nil {
		_ = out.Close()
		return nil, err
	}
	return bases, out.Close()
}

func (s *segmentStore) validateConsolidated(path string, firstSequence uint64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	result, err := validateRecordStream(data, s.validator, firstSequence)
	if err != nil {
		return fmt.Errorf("validate compacted segment: %w", err)
	}
	if result.IncompleteStart != nil || result.CommittedOffset != int64(len(data)) {
		return fmt.Errorf("compacted segment is not a sequence of complete transactions")
	}
	return nil
}

// firstSequenceOf returns the first global sequence stored in a segment by
// reading its first record header.
func (s *segmentStore) firstSequenceOf(segment *segmentFile) uint64 {
	f, err := storageOpenSegmentRead(segment.path)
	if err != nil {
		return 0
	}
	defer f.Close()
	frame, err := newRecordFramer(f).next()
	if err != nil {
		return 0
	}
	record, err := decodeRecord(frame, s.validator)
	if err != nil {
		return 0
	}
	sequence, _, _ := recordSequenceAndKind(record)
	return sequence
}
