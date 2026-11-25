package eventlog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ErrShortStorageWrite = errors.New("short storage write")

// storageCreateDirectory creates the storage directory hierarchy if it is absent.
func storageCreateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create storage directory %q: %w", path, err)
	}
	return nil
}

// storageCreateSegment creates a new segment file exclusively, preventing accidental
// reuse or overwrite of an existing segment ID.
func storageCreateSegment(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create segment %q: %w", path, err)
	}
	return f, nil
}

func storageOpenSegmentRead(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open segment for reading %q: %w", path, err)
	}
	return f, nil
}

// storageOpenSegmentAppend opens a segment for writing at its current end.
// It intentionally does not use O_APPEND: Windows disallows truncation on handles
// opened with append semantics, and transaction recovery must be able to truncate.
func storageOpenSegmentAppend(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open segment for append %q: %w", path, err)
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("seek segment end %q: %w", path, err)
	}
	return f, nil
}

// storageWriteAll writes all bytes or returns an error. It does not sync the file;
// callers choose the transaction durability boundary.
func storageWriteAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return fmt.Errorf("write storage bytes: %w", err)
		}

		if n == 0 {
			return ErrShortStorageWrite
		}

		data = data[n:]
	}
	return nil
}

func storageSync(f *os.File) error {
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync storage file: %w", err)
	}
	return nil
}

func storageTruncate(f *os.File, size int64) error {
	if size < 0 {
		return fmt.Errorf("truncate storage file to negative size %d", size)
	}
	if err := f.Truncate(size); err != nil {
		return fmt.Errorf("truncate storage file to %d: %w", size, err)
	}
	return nil
}

func storageSegmentPath(directory string, segmentID uint64) string {
	return filepath.Join(directory, fmt.Sprintf("segment-%06d.ndjson", segmentID))
}
