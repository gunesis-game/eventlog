package storage

import (
	"encoding/json"
	"fmt"

	"github.com/gunesis-game/eventlog/internal/filesystem"
)

const (
	indexFileName      = "index.json"
	indexFormatVersion = 1
)

// indexFile is the persisted, generation-bound form of the derived index. It is
// never authoritative: recovery rebuilds the index from validated segments and
// replaces this file whenever it is missing, stale, or inconsistent.
type indexFile struct {
	FormatVersion     int              `json:"format_version"`
	Generation        uint64           `json:"generation"`
	CommittedSequence uint64           `json:"committed_sequence"`
	Entries           []indexFileEntry `json:"entries"`
}

type indexFileEntry struct {
	Sequence  uint64 `json:"sequence"`
	EventID   string `json:"event_id"`
	ChannelID string `json:"channel_id"`
	Segment   string `json:"segment"`
	Offset    int64  `json:"offset"`
	Length    int64  `json:"length"`
}

func (f indexFile) encode() ([]byte, error) {
	raw, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("encode index: %w", err)
	}
	return append(raw, '\n'), nil
}

// write atomically replaces the stored index file.
func (f indexFile) write(files *filesystem.Client) error {
	raw, err := f.encode()
	if err != nil {
		return err
	}
	return files.Replace(indexFileName, raw)
}

// reconcile replaces the stored index file when it differs from f and reports
// whether it did.
func (f indexFile) reconcile(files *filesystem.Client) (bool, error) {
	expected, err := f.encode()
	if err != nil {
		return false, err
	}
	stored, err := files.Read(indexFileName, 0, -1)
	if err == nil && string(stored) == string(expected) {
		return false, nil
	}
	return true, files.Replace(indexFileName, expected)
}
