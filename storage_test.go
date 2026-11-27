package eventlog

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageCreateWriteAppendReadTruncateSyncReopen(t *testing.T) {
	dir := t.TempDir()
	segmentsDir := filepath.Join(dir, "segments")
	if err := storageCreateDirectory(segmentsDir); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	path := storageSegmentPath(segmentsDir, 1)
	f, err := storageCreateSegment(path)
	if err != nil {
		t.Fatalf("create segment: %v", err)
	}
	if err := storageWriteAll(f, []byte("first\n")); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := storageSync(f); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	f, err = storageOpenSegmentAppend(path)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if err := storageWriteAll(f, []byte("second\n")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := storageTruncate(f, int64(len("first\n"))); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := storageSync(f); err != nil {
		t.Fatalf("sync after truncate: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close after truncate: %v", err)
	}

	f, err = storageOpenSegmentRead(path)
	if err != nil {
		t.Fatalf("reopen read: %v", err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "first\n" {
		t.Fatalf("read back %q", got)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close final: %v", err)
	}
}

func TestStorageCreateSegmentIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "segment-000001.ndjson")
	f, err := storageCreateSegment(path)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := storageCreateSegment(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second create error = %v, want os.ErrExist", err)
	}
}

func TestStorageMissingDirectoryAndInvalidTruncate(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing", "segment.ndjson")
	if _, err := storageOpenSegmentAppend(missingPath); err == nil {
		t.Fatal("append open of missing file should fail")
	}
	f, err := storageCreateSegment(filepath.Join(t.TempDir(), "segment.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := storageTruncate(f, -1); err == nil {
		t.Fatal("negative truncate should fail")
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestStorageWriteAllErrors(t *testing.T) {
	if err := storageWriteAll(zeroWriter{}, []byte("x")); !errors.Is(err, errShortStorageWrite) {
		t.Fatalf("zero-write error = %v", err)
	}
	if err := storageWriteAll(failingWriter{}, []byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed write error = %v", err)
	}
	var b bytes.Buffer
	if err := storageWriteAll(&b, []byte("complete")); err != nil {
		t.Fatalf("write to buffer: %v", err)
	}
	if b.String() != "complete" {
		t.Fatalf("buffer contains %q", b.String())
	}
}
