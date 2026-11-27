// Package filesystem is eventlog's file-system boundary. internal/storage
// calls the HTTP Client; the Client talks to a localhost Server; the Server
// enforces the file contract and delegates to a Backend.
package filesystem

import "errors"

// ErrLocked reports that an ownership lock is already held.
var ErrLocked = errors.New("lock is held")

// Entry is one item of a folder listing.
type Entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
}

// Backend performs file operations on one storage root. Paths are clean,
// slash-separated, and relative to the root. A missing file is reported with
// fs.ErrNotExist and an existing one, on exclusive create, with fs.ErrExist.
//
// Each operation must meet the contract the Server promises clients:
// CreateExclusive, Replace, and Rename are atomic and durable on success;
// Append is ordered and returns the new size; Sync makes earlier appends
// durable; Read returns exactly the requested range.
type Backend interface {
	MakeDirectory(path string) error
	CreateExclusive(path string) error
	Read(path string, offset, length int64) ([]byte, error)
	Append(path string, data []byte) (int64, error)
	Truncate(path string, size int64) error
	Sync(path string) error
	Replace(path string, data []byte) error
	Rename(from, to string) error
	Remove(path string) error
	List(path string) ([]Entry, error)
	Lock(name string) error
	Unlock(name string) error
	Close() error
}
