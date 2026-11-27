package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

const (
	manifestFileName      = "manifest.json"
	manifestFormatVersion = 1
	segmentsDirName       = "segments"
	temporaryFileSuffix   = ".tmp"
)

var segmentFileNamePattern = regexp.MustCompile(`^segment-([0-9]{6,})\.ndjson$`)

// manifest identifies the authoritative segment set of the current generation.
// The last listed segment is the active, writable segment.
type manifest struct {
	FormatVersion int      `json:"format_version"`
	Generation    uint64   `json:"generation"`
	Segments      []string `json:"segments"`
	NextSegmentID uint64   `json:"next_segment_id"`
}

func (m manifest) validate() error {
	if m.FormatVersion != manifestFormatVersion {
		return fmt.Errorf("unsupported manifest format version %d", m.FormatVersion)
	}
	if m.Generation == 0 || len(m.Segments) == 0 {
		return errors.New("manifest has no generation or segments")
	}
	previous := uint64(0)
	for _, name := range m.Segments {
		id, err := segmentIDFromName(name)
		if err != nil {
			return err
		}
		if id <= previous {
			return fmt.Errorf("manifest segment %s is out of order", name)
		}
		previous = id
	}
	if m.NextSegmentID <= previous {
		return fmt.Errorf("manifest next segment ID %d does not exceed %d", m.NextSegmentID, previous)
	}
	return nil
}

func segmentIDFromName(name string) (uint64, error) {
	match := segmentFileNamePattern.FindStringSubmatch(name)
	if match == nil {
		return 0, fmt.Errorf("invalid segment file name %q", name)
	}
	id, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid segment file name %q", name)
	}
	return id, nil
}

func segmentFileName(id uint64) string {
	return filepath.Base(storageSegmentPath("", id))
}

// readManifest returns the manifest, os.ErrNotExist when absent, or a
// validation error when present but unusable.
func readManifest(directory string) (manifest, error) {
	raw, err := os.ReadFile(filepath.Join(directory, manifestFileName))
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := m.validate(); err != nil {
		return manifest{}, err
	}
	return m, nil
}

// writeManifest atomically replaces the manifest: write temporary, sync, rename.
func writeManifest(directory string, m manifest) error {
	if err := m.validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return storageReplaceFile(filepath.Join(directory, manifestFileName), append(raw, '\n'))
}
