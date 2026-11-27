package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// readOnlyBackend refuses every change with fs.ErrPermission.
type readOnlyBackend struct {
	Backend
}

func (readOnlyBackend) refused(p string) error {
	return fmt.Errorf("change %s: %w", p, fs.ErrPermission)
}

func (b readOnlyBackend) MakeDirectory(p string) error     { return b.refused(p) }
func (b readOnlyBackend) CreateExclusive(p string) error   { return b.refused(p) }
func (b readOnlyBackend) Truncate(p string, _ int64) error { return b.refused(p) }
func (b readOnlyBackend) Sync(p string) error              { return b.refused(p) }
func (b readOnlyBackend) Replace(p string, _ []byte) error { return b.refused(p) }
func (b readOnlyBackend) Rename(from, _ string) error      { return b.refused(from) }
func (b readOnlyBackend) Remove(p string) error            { return b.refused(p) }
func (b readOnlyBackend) Lock(name string) error           { return b.refused(name) }
func (b readOnlyBackend) Append(p string, _ []byte) (int64, error) {
	return 0, b.refused(p)
}

// A backend's fs.ErrPermission reaches the client as fs.ErrPermission, and
// reads still work.
func TestPermissionErrorCrossesTheBoundary(t *testing.T) {
	dir := t.TempDir()
	writable, err := NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := writable.Replace("file", []byte("data")); err != nil {
		t.Fatal(err)
	}
	server, err := Serve(readOnlyBackend{Backend: writable})
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(server.URL(), server.Token())
	t.Cleanup(func() {
		client.Close()
		_ = server.Close()
	})
	_, appendErr := client.Append("file", []byte("x"))
	for name, err := range map[string]error{
		"make directory": client.MakeDirectory("dir"),
		"create":         client.CreateExclusive("new"),
		"append":         appendErr,
		"truncate":       client.Truncate("file", 0),
		"sync":           client.Sync("file"),
		"replace":        client.Replace("file", nil),
		"rename":         client.Rename("file", "moved"),
		"remove":         client.Remove("file"),
		"lock":           client.Lock("lock"),
	} {
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("%s: err = %v, want fs.ErrPermission", name, err)
		}
	}
	if data, err := client.Read("file", 0, -1); err != nil || string(data) != "data" {
		t.Fatalf("read = %q, %v", data, err)
	}
}

// A file the operating system will not let this process write is still read,
// and a write to it is refused with fs.ErrPermission.
func TestOSBackendReadOnlyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not refuse root")
	}
	dir := t.TempDir()
	backend, err := NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})
	if err := backend.Replace("file", []byte("data")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "file")
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o600)
	})
	if data, err := backend.Read("file", 0, -1); err != nil || string(data) != "data" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if _, err := backend.Append("file", []byte("x")); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("append err = %v, want fs.ErrPermission", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if size, err := backend.Append("file", []byte("x")); err != nil || size != 5 {
		t.Fatalf("append after write access returned = %d, %v", size, err)
	}
}
