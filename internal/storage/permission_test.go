package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/gunesis-game/eventlog/internal/filesystem"
	"github.com/gunesis-game/eventlog/internal/identity"
)

// refusingBackend is storage that refuses every write with fs.ErrPermission
// while refuse is set, as a read-only directory or a Drive Viewer does.
type refusingBackend struct {
	filesystem.Backend
	refuse atomic.Bool
}

func (b *refusingBackend) refused(op, p string) error {
	if b.refuse.Load() {
		return fmt.Errorf("%s %s: %w", op, p, fs.ErrPermission)
	}
	return nil
}

func (b *refusingBackend) MakeDirectory(p string) error {
	if err := b.refused("make directory", p); err != nil {
		return err
	}
	return b.Backend.MakeDirectory(p)
}

func (b *refusingBackend) CreateExclusive(p string) error {
	if err := b.refused("create", p); err != nil {
		return err
	}
	return b.Backend.CreateExclusive(p)
}

func (b *refusingBackend) Append(p string, data []byte) (int64, error) {
	if err := b.refused("append", p); err != nil {
		return 0, err
	}
	return b.Backend.Append(p, data)
}

func (b *refusingBackend) Truncate(p string, size int64) error {
	if err := b.refused("truncate", p); err != nil {
		return err
	}
	return b.Backend.Truncate(p, size)
}

func (b *refusingBackend) Sync(p string) error {
	if err := b.refused("sync", p); err != nil {
		return err
	}
	return b.Backend.Sync(p)
}

func (b *refusingBackend) Replace(p string, data []byte) error {
	if err := b.refused("replace", p); err != nil {
		return err
	}
	return b.Backend.Replace(p, data)
}

func (b *refusingBackend) Rename(from, to string) error {
	if err := b.refused("rename", from); err != nil {
		return err
	}
	return b.Backend.Rename(from, to)
}

func (b *refusingBackend) Remove(p string) error {
	if err := b.refused("remove", p); err != nil {
		return err
	}
	return b.Backend.Remove(p)
}

func (b *refusingBackend) Lock(name string) error {
	if err := b.refused("lock", name); err != nil {
		return err
	}
	return b.Backend.Lock(name)
}

// openRefusingStore opens the store in dir over a refusingBackend, through the
// real file-system server and client.
func openRefusingStore(t *testing.T, dir string, refuse bool) (*Store, *refusingBackend) {
	t.Helper()
	osBackend, err := filesystem.NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	backend := &refusingBackend{Backend: osBackend}
	backend.refuse.Store(refuse)
	server, err := filesystem.Serve(backend)
	if err != nil {
		t.Fatal(err)
	}
	s, err := openLocal(filesystem.NewClient(server.URL(), server.Token()), fastStoreOptions())
	if err != nil {
		_ = server.Close()
		t.Fatalf("open store: %v", err)
	}
	s.server = server
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s, backend
}

// snapshotFiles returns every file under dir with its contents.
func snapshotFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sortedNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func expectPermission(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want fs.ErrPermission", err)
	}
}

// Storage that refuses writes opens read-only: committed history is readable,
// appends fail with the permission error, and nothing is written.
func TestReadOnlyStorageOpensForReading(t *testing.T) {
	dir := t.TempDir()
	writer := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, writer, 1, 10, "a")
	appendTestOccurrence(t, writer, 1, 10, "b")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, dir)

	s, _ := openRefusingStore(t, dir, true)
	expectPermission(t, s.WriteRefusal())
	frame, err := readOneOccurrence(s, identity.UUID(testUUID(1)), 1)
	if err != nil || !bytes.Contains(frame, []byte(`"value":"b"`)) {
		t.Fatalf("read occurrence = %s, %v", frame, err)
	}
	expectPermission(t, s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(10, "c")}))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	after := snapshotFiles(t, dir)
	if fmt.Sprint(sortedNames(after)) != fmt.Sprint(sortedNames(before)) {
		t.Fatalf("files changed: %v -> %v", sortedNames(before), sortedNames(after))
	}
	for name, data := range before {
		if after[name] != data {
			t.Fatalf("%s changed", name)
		}
	}
}

// Read-only storage ignores an incomplete trailing transaction instead of
// truncating it.
func TestReadOnlyStorageLeavesIncompleteTransaction(t *testing.T) {
	dir := t.TempDir()
	writer := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, writer, 1, 10, "committed")
	path := diskPath(dir, activeSegmentPath(writer))
	committed, _ := os.ReadFile(path)
	appendTestOccurrence(t, writer, 2, 11, "interrupted")
	full, _ := os.ReadFile(path)
	_ = writer.Close()
	interrupted := full[:len(committed)+(len(full)-len(committed))/2]
	if err := os.WriteFile(path, interrupted, 0o600); err != nil {
		t.Fatal(err)
	}

	s, _ := openRefusingStore(t, dir, true)
	if got := len(s.index.occurrences); got != 1 {
		t.Fatalf("recovered %d occurrences, want 1:\n%s", got, storeSummary(s))
	}
	_ = s.Close()
	if data, _ := os.ReadFile(path); !bytes.Equal(data, interrupted) {
		t.Fatal("read-only recovery changed the segment")
	}
}

// Fresh storage that refuses writes opens as an empty log and creates nothing.
func TestReadOnlyFreshStorageIsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, _ := openRefusingStore(t, dir, true)
	expectPermission(t, s.WriteRefusal())
	if batch, _, _ := s.ReadOccurrences(identity.UUID(testUUID(1)), 0, 1); len(batch) != 0 {
		t.Fatal("empty storage has an occurrence")
	}
	_ = s.Close()
	if files := snapshotFiles(t, dir); len(files) != 0 {
		t.Fatalf("read-only open created %v", sortedNames(files))
	}
}

// A write refused while running fails the append and every later one, even
// if the storage would permit writes again; committed history stays readable.
func TestWriteRefusedWhileRunning(t *testing.T) {
	dir := t.TempDir()
	s, backend := openRefusingStore(t, dir, false)
	if s.WriteRefusal() != nil {
		t.Fatal("writable storage reports a refusal")
	}
	appendTestOccurrence(t, s, 1, 10, "a")
	backend.refuse.Store(true)
	write := func() error {
		return s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(10, "b")})
	}
	expectPermission(t, write())
	expectPermission(t, s.WriteRefusal())
	backend.refuse.Store(false)
	expectPermission(t, write())
	if frame, err := readOneOccurrence(s, identity.UUID(testUUID(1)), 0); err != nil || !bytes.Contains(frame, []byte(`"value":"a"`)) {
		t.Fatalf("read occurrence = %s, %v", frame, err)
	}
}

// Storage the operating system does not let this process write opens
// read-only. Every file is made read-only, and on Unix every directory too.
// Windows keeps its directories writable: there a directory's read-only
// attribute does not stop creating files in it, and Go's os.Chmod sets only
// that attribute. Read-only files are enough, because the ownership lock file
// can then not be opened for writing. Skipped for root, whom permissions do
// not stop.
func TestReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not refuse root")
	}
	dir := t.TempDir()
	writer := openTestStore(t, dir, fastStoreOptions())
	appendTestOccurrence(t, writer, 1, 10, "a")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var paths []string
	_ = filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		paths = append(paths, p)
		return err
	})
	for i := len(paths) - 1; i >= 0; i-- {
		mode := os.FileMode(0o400)
		if info, err := os.Stat(paths[i]); err == nil && info.IsDir() {
			if runtime.GOOS == "windows" {
				continue
			}
			mode = 0o500
		}
		if err := os.Chmod(paths[i], mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, p := range paths {
			_ = os.Chmod(p, 0o700)
		}
	})

	s := openTestStore(t, dir, fastStoreOptions())
	expectPermission(t, s.WriteRefusal())
	if frame, err := readOneOccurrence(s, identity.UUID(testUUID(1)), 0); err != nil || !bytes.Contains(frame, []byte(`"value":"a"`)) {
		t.Fatalf("read occurrence = %s, %v", frame, err)
	}
	expectPermission(t, s.AppendTransaction(ChannelWrite{ID: identity.UUID(testUUID(1)), Type: testChannelType}, []OccurrenceWrite{testOccurrence(10, "b")}))
}
