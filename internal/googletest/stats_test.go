package googletest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stringsReader(s string) io.Reader {
	return strings.NewReader(s)
}

// Calls are counted per account and per call, with the bytes of each request
// and response, so benchmarks can report Google calls and bytes.
func TestCallsAreCountedPerAccountAndCall(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := mustCreate(t, editorDrive, "segment-1", folder, bytes.Repeat([]byte("x"), 1000))
	e.ResetStats()

	if _, err := editorDrive.UpdateContent(ctx, file, bytes.Repeat([]byte("y"), 2000)); err != nil {
		t.Fatal(err)
	}
	part, _ := ownerDrive.Download(ctx, file, 0, 500)
	if _, err := ownerDrive.Download(ctx, file, 0, -1); err != nil {
		t.Fatal(err)
	}
	if _, err := ownerDrive.List(ctx, fmt.Sprintf("'%s' in parents", folder), ""); err != nil {
		t.Fatal(err)
	}
	stats := e.Stats()
	upload := stats[editor]["drive.files.update.upload"]
	if upload.Requests != 1 || upload.RequestBytes != 2000 || upload.ResponseBytes == 0 {
		t.Fatalf("editor upload = %+v", upload)
	}
	reads := stats[owner]["drive.files.get.media"]
	if len(part) != 500 || reads.Requests != 2 || reads.ResponseBytes != 2500 {
		t.Fatalf("owner reads = %+v", reads)
	}
	if stats[owner]["drive.files.list"].Requests != 1 || len(stats[owner]) != 2 || len(stats[editor]) != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	if _, err := emulatedClient(t, e).Refresh(ctx, http.DefaultClient, "unknown"); err == nil {
		t.Fatal("an unknown refresh token was accepted")
	}
	if e.Stats()[""]["oauth.token"].Requests != 1 {
		t.Fatalf("a call without an account = %+v", e.Stats()[""])
	}
}

// Every call is delayed by the configured latency.
func TestLatencyIsAddedToEveryCall(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{Latency: 50 * time.Millisecond})
	e.AddTestUser(owner)
	d := driveAs(t, e, owner)
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := d.Get(ctx, "root"); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Fatalf("three calls took %v, want at least 150ms", elapsed)
	}
}

// The state is kept on a real file system: each file's content as a file
// named by its ID, and the metadata beside them, so an emulator restarted
// over the same folder keeps its files, accounts, and tokens.
func TestRestartKeepsTheState(t *testing.T) {
	dir := t.TempDir()
	e := startEmulator(t, dir, Options{})
	e.AddTestUser(owner)
	token := signIn(t, e, owner, "https://www.googleapis.com/auth/drive")
	d := driveWith(e, token.AccessToken)
	folder, err := d.CreateFolder(ctx, "eventlog", "root", nil)
	if err != nil {
		t.Fatal(err)
	}
	file := mustCreate(t, d, "segment-1", folder.ID, []byte("persisted"))
	if content, err := os.ReadFile(filepath.Join(dir, contentDirName, file)); err != nil || string(content) != "persisted" {
		t.Fatalf("content on disk = %q, %v", content, err)
	}
	if raw, err := os.ReadFile(filepath.Join(dir, metadataFileName)); err != nil || !json.Valid(raw) {
		t.Fatalf("metadata on disk: %v", err)
	}
	clientID, _ := e.Client()
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := startEmulator(t, dir, Options{})
	if id, _ := restarted.Client(); id != clientID {
		t.Fatal("the OAuth client changed")
	}
	again := driveWith(restarted, token.AccessToken)
	if content, err := again.Download(ctx, file, 0, -1); err != nil || string(content) != "persisted" {
		t.Fatalf("after restart = %q, %v", content, err)
	}
	if files, err := again.List(ctx, fmt.Sprintf("'%s' in parents", folder.ID), ""); err != nil || len(files) != 1 || files[0].Name != "segment-1" {
		t.Fatalf("after restart lists %v, %v", names(files), err)
	}
}
