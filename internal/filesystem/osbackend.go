package filesystem

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// OSBackend stores files in a directory of the local operating system. It
// keeps one open handle per file and closes it before the file is renamed or
// removed, which Windows requires. Operations are serialized.
type OSBackend struct {
	root string

	mu      sync.Mutex
	handles map[string]*os.File
	locks   map[string]*os.File
}

// NewOSBackend returns a backend rooted at directory, creating it if absent.
func NewOSBackend(directory string) (*OSBackend, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create storage directory %q: %w", directory, err)
	}
	return &OSBackend{root: directory, handles: make(map[string]*os.File), locks: make(map[string]*os.File)}, nil
}

func (b *OSBackend) native(p string) string {
	return filepath.Join(b.root, filepath.FromSlash(p))
}

// handle returns the open handle for an existing file, opening it on first use.
// It deliberately avoids O_APPEND: Windows disallows truncating such handles.
func (b *OSBackend) handle(p string) (*os.File, error) {
	if f, ok := b.handles[p]; ok {
		return f, nil
	}
	f, err := os.OpenFile(b.native(p), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	b.handles[p] = f
	return f, nil
}

func (b *OSBackend) release(p string) {
	if f, ok := b.handles[p]; ok {
		_ = f.Close()
		delete(b.handles, p)
	}
}

func (b *OSBackend) MakeDirectory(p string) error {
	return os.MkdirAll(b.native(p), 0o700)
}

func (b *OSBackend) CreateExclusive(p string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := os.OpenFile(b.native(p), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	b.handles[p] = f
	return syncDirectory(filepath.Dir(b.native(p)))
}

func (b *OSBackend) Read(p string, offset, length int64) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.handle(p)
	if err != nil {
		return nil, err
	}
	if length < 0 {
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		length = info.Size() - offset
		if length < 0 {
			return nil, fmt.Errorf("read %s at %d beyond size %d", p, offset, info.Size())
		}
	}
	data := make([]byte, length)
	if _, err := f.ReadAt(data, offset); err != nil {
		return nil, fmt.Errorf("read %s at %d: %w", p, offset, err)
	}
	return data, nil
}

func (b *OSBackend) Append(p string, data []byte) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.handle(p)
	if err != nil {
		return 0, err
	}
	end, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	for written := 0; written < len(data); {
		n, err := f.Write(data[written:])
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, errors.New("short write")
		}
		written += n
	}
	return end + int64(len(data)), nil
}

func (b *OSBackend) Truncate(p string, size int64) error {
	if size < 0 {
		return fmt.Errorf("truncate %s to negative size %d", p, size)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.handle(p)
	if err != nil {
		return err
	}
	return f.Truncate(size)
}

func (b *OSBackend) Sync(p string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, err := b.handle(p)
	if err != nil {
		return err
	}
	return f.Sync()
}

// Replace writes a synced temporary sibling and renames it over p.
func (b *OSBackend) Replace(p string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	target := b.native(p)
	temporary := target + ".tmp"
	f, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	b.release(p)
	if err := renameWithRetry(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return syncDirectory(filepath.Dir(target))
}

func (b *OSBackend) Rename(from, to string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.release(from)
	b.release(to)
	if err := renameWithRetry(b.native(from), b.native(to)); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(b.native(to)))
}

func (b *OSBackend) Remove(p string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.release(p)
	return os.Remove(b.native(p))
}

func (b *OSBackend) List(p string) ([]Entry, error) {
	entries, err := os.ReadDir(b.native(p))
	if err != nil {
		return nil, err
	}
	listing := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		listing = append(listing, Entry{Name: entry.Name(), Dir: entry.IsDir()})
	}
	return listing, nil
}

// Lock takes the operating-system ownership lock for name, which the
// operating system releases when the process exits.
func (b *OSBackend) Lock(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, held := b.locks[name]; held {
		return ErrLocked
	}
	f, err := acquireLock(b.native(name))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLocked, err)
	}
	b.locks[name] = f
	return nil
}

func (b *OSBackend) Unlock(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	f, held := b.locks[name]
	if !held {
		return fs.ErrNotExist
	}
	delete(b.locks, name)
	return f.Close()
}

// Close releases every open handle and lock.
func (b *OSBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var errs []error
	for p, f := range b.handles {
		errs = append(errs, f.Close())
		delete(b.handles, p)
	}
	for name, f := range b.locks {
		errs = append(errs, f.Close())
		delete(b.locks, name)
	}
	return errors.Join(errs...)
}

// renameWithRetry renames old to new. On Windows a rename over a file that
// another process (an indexer or virus scanner) briefly holds open fails with
// access denied, so it is retried for a short bounded time there.
func renameWithRetry(old, new string) error {
	err := os.Rename(old, new)
	for attempt := 0; err != nil && runtime.GOOS == "windows" && attempt < 20; attempt++ {
		time.Sleep(10 * time.Millisecond)
		err = os.Rename(old, new)
	}
	return err
}

// syncDirectory makes directory entry changes durable where the platform
// supports syncing a directory handle. Windows does not, and NTFS metadata
// journaling covers renames, so it is a no-op there.
func syncDirectory(directory string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
