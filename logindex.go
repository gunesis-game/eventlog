package eventlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	indexFileName      = "index.json"
	indexFormatVersion = 1
)

// logIndex is the persisted, generation-bound form of the derived index. It is
// never authoritative: recovery rebuilds it from validated segments and replaces
// it whenever the stored copy is missing, stale, or inconsistent.
type logIndex struct {
	FormatVersion     int             `json:"format_version"`
	Generation        uint64          `json:"generation"`
	CommittedSequence uint64          `json:"committed_sequence"`
	Entries           []logIndexEntry `json:"entries"`
}

type logIndexEntry struct {
	Sequence  uint64 `json:"sequence"`
	EventID   string `json:"event_id"`
	ChannelID string `json:"channel_id"`
	Segment   string `json:"segment"`
	Offset    int64  `json:"offset"`
	Length    int64  `json:"length"`
}

func (s *segmentStore) buildIndexLocked() logIndex {
	index := logIndex{FormatVersion: indexFormatVersion, Generation: s.manifest.Generation, CommittedSequence: s.nextSequence - 1, Entries: make([]logIndexEntry, 0, len(s.entries))}
	for _, entry := range s.entries {
		index.Entries = append(index.Entries, logIndexEntry{
			Sequence:  entry.sequence,
			EventID:   entry.eventID.String(),
			ChannelID: entry.channelID.String(),
			Segment:   segmentFileName(entry.segment.id),
			Offset:    entry.offset,
			Length:    entry.length,
		})
	}
	return index
}

func encodeLogIndex(index logIndex) ([]byte, error) {
	raw, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("encode index: %w", err)
	}
	return append(raw, '\n'), nil
}

func (s *segmentStore) writeIndexLocked() error {
	raw, err := encodeLogIndex(s.buildIndexLocked())
	if err != nil {
		return err
	}
	return storageReplaceFile(filepath.Join(s.directory, indexFileName), raw)
}

// reconcileIndexLocked compares the stored index with the one rebuilt from the
// segments and atomically replaces the stored copy when they differ.
func (s *segmentStore) reconcileIndexLocked() error {
	expected, err := encodeLogIndex(s.buildIndexLocked())
	if err != nil {
		return err
	}
	stored, err := os.ReadFile(filepath.Join(s.directory, indexFileName))
	if err == nil && string(stored) == string(expected) {
		return nil
	}
	s.indexRebuilt = true
	return storageReplaceFile(filepath.Join(s.directory, indexFileName), expected)
}
