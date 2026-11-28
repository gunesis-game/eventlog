package filesystem

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// openClient starts a server over an OS backend in a temporary directory.
func openClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	backend, err := NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	server, err := Serve(backend)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(server.URL(), server.Token())
	t.Cleanup(func() {
		client.Close()
		_ = server.Close()
	})
	return client, dir
}

type eventBackend struct {
	*OSBackend
	events chan ChangeEvent
}

func (b *eventBackend) ChangeEvents(context.Context) (<-chan ChangeEvent, error) {
	return b.events, nil
}

func TestChangeEventStream(t *testing.T) {
	backend, err := NewOSBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	provider := &eventBackend{OSBackend: backend, events: make(chan ChangeEvent, 1)}
	server, err := Serve(provider)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(server.URL(), server.Token())
	t.Cleanup(func() {
		client.Close()
		_ = server.Close()
	})
	stream, err := client.ChangeEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	provider.events <- ChangeEvent{Path: "devices/device/segments/0001", Size: 123}
	got, err := stream.Next()
	if err != nil || got != (ChangeEvent{Path: "devices/device/segments/0001", Size: 123}) {
		t.Fatalf("change event = %+v, %v", got, err)
	}
}

func TestChangeEventsUnavailableOnLocalBackend(t *testing.T) {
	client, _ := openClient(t)
	if _, err := client.ChangeEvents(context.Background()); !errors.Is(err, ErrChangeEventsUnavailable) {
		t.Fatalf("local change events = %v, want ErrChangeEventsUnavailable", err)
	}
}

func TestCreateAppendReadTruncateSync(t *testing.T) {
	c, dir := openClient(t)
	if err := c.MakeDirectory("segments"); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateExclusive("segments/a"); err != nil {
		t.Fatal(err)
	}
	if err := c.CreateExclusive("segments/a"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("second exclusive create: %v", err)
	}
	for i, chunk := range []string{"hello ", "world\n"} {
		size, err := c.Append("segments/a", []byte(chunk))
		if err != nil {
			t.Fatal(err)
		}
		if want := []int64{6, 12}[i]; size != want {
			t.Fatalf("size after append %d = %d, want %d", i, size, want)
		}
	}
	if err := c.Sync("segments/a"); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		offset, length int64
		want           string
	}{{0, -1, "hello world\n"}, {6, 5, "world"}, {6, -1, "world\n"}, {3, 0, ""}}
	for _, check := range checks {
		got, err := c.Read("segments/a", check.offset, check.length)
		if err != nil || string(got) != check.want {
			t.Fatalf("Read(%d, %d) = %q, %v; want %q", check.offset, check.length, got, err, check.want)
		}
	}
	if _, err := c.Read("segments/a", 10, 10); err == nil {
		t.Fatal("reading past the end must fail")
	}
	if err := c.Truncate("segments/a", 5); err != nil {
		t.Fatal(err)
	}
	if size, err := c.Append("segments/a", []byte("!")); err != nil || size != 6 {
		t.Fatalf("append after truncate = %d, %v", size, err)
	}
	onDisk, _ := os.ReadFile(filepath.Join(dir, "segments", "a"))
	if string(onDisk) != "hello!" {
		t.Fatalf("disk content = %q", onDisk)
	}
}

func TestReplaceRenameRemoveList(t *testing.T) {
	c, _ := openClient(t)
	if err := c.Replace("manifest.json", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := c.Replace("manifest.json", []byte("two")); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Read("manifest.json", 0, -1); string(got) != "two" {
		t.Fatalf("replaced content = %q", got)
	}
	if err := c.Replace("other", []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := c.Rename("manifest.json", "other"); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Read("other", 0, -1); string(got) != "two" {
		t.Fatalf("rename must replace the target, got %q", got)
	}
	if _, err := c.Read("manifest.json", 0, -1); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("renamed source: %v", err)
	}
	if err := c.MakeDirectory("segments"); err != nil {
		t.Fatal(err)
	}
	entries, err := c.List("")
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	if len(entries) != 2 || entries[0] != (Entry{Name: "other"}) || entries[1] != (Entry{Name: "segments", Dir: true}) {
		t.Fatalf("listing = %+v", entries)
	}
	if err := c.Remove("other"); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove("other"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removing a missing file: %v", err)
	}
	if _, err := c.List("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("listing a missing folder: %v", err)
	}
}

func TestOwnershipLock(t *testing.T) {
	c, dir := openClient(t)
	if err := c.Lock("lock"); err != nil {
		t.Fatal(err)
	}
	if err := c.Lock("lock"); !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock on one server: %v", err)
	}
	second, err := NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := second.Lock("lock"); !errors.Is(err, ErrLocked) {
		t.Fatalf("lock from another backend: %v", err)
	}
	if err := c.Unlock("lock"); err != nil {
		t.Fatal(err)
	}
	if err := second.Lock("lock"); err != nil {
		t.Fatalf("lock after release: %v", err)
	}
}

func TestServerRejectsBadRequests(t *testing.T) {
	c, _ := openClient(t)
	intruder := NewClient(c.base, "wrong")
	if err := intruder.Replace("x", []byte("y")); err == nil {
		t.Fatal("a wrong token must be rejected")
	}
	for _, p := range []string{"../escape", "a/../../b", "/abs", "a\\b", "c:x"} {
		if err := c.Replace(p, []byte("x")); err == nil {
			t.Fatalf("path %q must be rejected", p)
		}
	}
	request, _ := http.NewRequest(http.MethodPost, c.base+"/files/x?op=explode", nil)
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown op status = %d", response.StatusCode)
	}
}

func TestParseRange(t *testing.T) {
	cases := map[string][2]int64{"": {0, -1}, "bytes=0-0": {0, 1}, "bytes=5-9": {5, 5}, "bytes=7-": {7, -1}}
	for header, want := range cases {
		offset, length, err := parseRange(header)
		if err != nil || offset != want[0] || length != want[1] {
			t.Fatalf("parseRange(%q) = %d, %d, %v", header, offset, length, err)
		}
	}
	for _, bad := range []string{"bytes=9-5", "items=0-1", "bytes=-5", "bytes=x-1"} {
		if _, _, err := parseRange(bad); err == nil {
			t.Fatalf("parseRange(%q) must fail", bad)
		}
	}
}

// D-07, D-08: data survives a new server over the same directory, and creating
// a file in a missing folder fails rather than succeeding silently.
func TestReopenAndMissingFolder(t *testing.T) {
	c, dir := openClient(t)
	if err := c.CreateExclusive("data"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Append("data", []byte("kept")); err != nil {
		t.Fatal(err)
	}
	if err := c.Sync("data"); err != nil {
		t.Fatal(err)
	}
	backend, err := NewOSBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	server, err := Serve(backend)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	reopened := NewClient(server.URL(), server.Token())
	defer reopened.Close()
	if got, err := reopened.Read("data", 0, -1); err != nil || string(got) != "kept" {
		t.Fatalf("reopened read = %q, %v", got, err)
	}
	if err := c.CreateExclusive("missing/file"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("create in a missing folder: %v", err)
	}
	if err := c.Truncate("data", -1); err == nil {
		t.Fatal("negative truncate must fail")
	}
}

// D-055: the server speaks only HTTP/2 with prior knowledge, and the client
// carries every concurrent request on one connection.
func TestSingleHTTP2Connection(t *testing.T) {
	c, _ := openClient(t)
	if err := c.Replace("f", []byte("data")); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, c.base+"/files/f", nil)
	request.Header.Set("Authorization", "Bearer "+c.token)
	if response, err := http.DefaultClient.Do(request); err == nil {
		response.Body.Close()
		t.Fatalf("an HTTP/1.1 request must be refused, got %s", response.Status)
	}

	var mu sync.Mutex
	connections := make(map[string]bool)
	protocols := make(map[string]bool)
	trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		mu.Lock()
		connections[info.Conn.LocalAddr().String()] = true
		mu.Unlock()
	}}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request, _ := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, c.base+"/files/f", nil)
			request.Header.Set("Authorization", "Bearer "+c.token)
			response, err := c.http.Do(request)
			if err != nil {
				t.Error(err)
				return
			}
			response.Body.Close()
			mu.Lock()
			protocols[response.Proto] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(connections) != 1 {
		t.Fatalf("requests used %d connections, want 1", len(connections))
	}
	if len(protocols) != 1 || !protocols["HTTP/2.0"] {
		t.Fatalf("protocols = %v, want only HTTP/2.0", protocols)
	}
}

// A Sub client works inside its folder: paths and lock names are relative to
// it, and paths cannot leave it.
func TestSubClient(t *testing.T) {
	client, dir := openClient(t)
	sub := client.Sub("a/b")
	if err := sub.MakeDirectory("c"); err != nil {
		t.Fatal(err)
	}
	if err := sub.Replace("c/file", []byte("data")); err != nil {
		t.Fatal(err)
	}
	if data, err := client.Read("a/b/c/file", 0, -1); err != nil || string(data) != "data" {
		t.Fatalf("read through the root = %q, %v", data, err)
	}
	if err := sub.Rename("c/file", "moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "b", "moved")); err != nil {
		t.Fatalf("rename target: %v", err)
	}
	if err := sub.Lock("lock"); err != nil {
		t.Fatal(err)
	}
	if err := client.Lock("a/b/lock"); !errors.Is(err, ErrLocked) {
		t.Fatalf("lock through the root = %v, want ErrLocked", err)
	}
	if err := sub.Unlock("lock"); err != nil {
		t.Fatal(err)
	}
	for _, escape := range []string{"../x", "/x", "c/../../x"} {
		if err := sub.Replace(escape, nil); err == nil {
			t.Fatalf("path %q left the folder", escape)
		}
		if err := sub.Rename("moved", escape); err == nil {
			t.Fatalf("rename target %q left the folder", escape)
		}
	}
	entries, err := sub.Sub("c").List("")
	if err != nil || len(entries) != 0 {
		t.Fatalf("nested Sub listing = %v, %v", entries, err)
	}
}
