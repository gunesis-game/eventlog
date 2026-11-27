package googletest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Calls are counted per account and per call, with the bytes of each request
// and response, so benchmarks can report Google calls and bytes.
func TestCallsAreCountedPerAccountAndCall(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := editorDrive.mustUpload("segment-1", folder, bytes.Repeat([]byte("x"), 1000))
	e.ResetStats()

	editorDrive.replaceContent(file, bytes.Repeat([]byte("y"), 2000))
	_, content := ownerDrive.read(file, "bytes=0-499")
	ownerDrive.read(file, "")
	ownerDrive.list(q("'%s' in parents", folder))
	stats := e.Stats()

	upload := stats[editor]["drive.files.update.upload"]
	if upload.Requests != 1 || upload.RequestBytes != 2000 || upload.ResponseBytes == 0 {
		t.Fatalf("editor upload = %+v", upload)
	}
	reads := stats[owner]["drive.files.get.media"]
	if len(content) != 500 || reads.Requests != 2 || reads.ResponseBytes != 2500 {
		t.Fatalf("owner reads = %+v", reads)
	}
	if stats[owner]["drive.files.list"].Requests != 1 || len(stats[owner]) != 2 || len(stats[editor]) != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	clientID, secret := e.Client()
	tokenRequest(t, e, map[string][]string{"client_id": {clientID}, "client_secret": {secret}, "grant_type": {"refresh_token"}, "refresh_token": {"unknown"}})
	if e.Stats()[""]["oauth.token"].Requests != 1 {
		t.Fatalf("a call without an account = %+v", e.Stats()[""])
	}
}

// Every call is delayed by the configured latency.
func TestLatencyIsAddedToEveryCall(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{Latency: 50 * time.Millisecond})
	e.AddTestUser(owner)
	drive, _ := signIn(t, e, owner, driveScope)
	start := time.Now()
	for i := 0; i < 3; i++ {
		drive.call("GET", "/drive/v3/files/root", nil)
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
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := editorDrive.mustUpload("segment-1", folder, []byte("persisted"))
	if content, err := os.ReadFile(filepath.Join(dir, contentDirName, file)); err != nil || string(content) != "persisted" {
		t.Fatalf("content on disk = %q, %v", content, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, metadataFileName))
	if err != nil || !json.Valid(raw) {
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
	ownerDrive.base = restarted.URL()
	if status, content := ownerDrive.read(file, ""); status != http.StatusOK || string(content) != "persisted" {
		t.Fatalf("after restart: %d %q", status, content)
	}
	if names := ownerDrive.list(q("'%s' in parents", folder)); len(names) != 1 || names[0] != "segment-1" {
		t.Fatalf("after restart lists %v", names)
	}
}
