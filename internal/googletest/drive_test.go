package googletest

import (
	"bytes"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gunesis-game/eventlog/internal/drive"
)

// The creating account owns a file, with its size and parent.
func TestCreatorOwnsFiles(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	id := mustCreate(t, editorDrive, "segment-1", folder, []byte("hello"))
	f, err := editorDrive.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if f.Size != 5 || f.Parents[0] != folder || len(f.Owners) != 1 || f.Owners[0].EmailAddress != editor || !f.Capabilities.CanTrash {
		t.Fatalf("metadata = %+v", f)
	}
}

// A folder that is not shared with an account is not found, nor is anything
// in it, and listing it finds nothing.
func TestUnsharedFolderIsNotFound(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	file := mustCreate(t, ownerDrive, "log.json", folder, []byte("{}"))
	e.AddTestUser(stranger)
	strangerDrive := driveAs(t, e, stranger)
	for _, id := range []string{folder, file} {
		_, err := strangerDrive.Get(ctx, id)
		expectDriveError(t, "get "+id, err, http.StatusNotFound, drive.ReasonNotFound)
	}
	_, err := strangerDrive.Create(ctx, "intruder", folder, []byte("x"))
	expectDriveError(t, "create in the folder", err, http.StatusNotFound, drive.ReasonNotFound)
	if files, err := strangerDrive.List(ctx, fmt.Sprintf("'%s' in parents", folder), ""); err != nil || len(files) != 0 {
		t.Fatalf("stranger lists %v, %v", names(files), err)
	}
}

// A Viewer reads the folder's files but is refused every write.
func TestViewerIsRefusedWrites(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, viewerDrive, folder := sharedFolder(t, e)
	file := mustCreate(t, ownerDrive, "log.json", folder, []byte("{}"))
	if content, err := viewerDrive.Download(ctx, file, 0, -1); err != nil || string(content) != "{}" {
		t.Fatalf("viewer read = %q, %v", content, err)
	}
	_, err := viewerDrive.Create(ctx, "mine", folder, []byte("x"))
	expectDriveError(t, "viewer create", err, http.StatusForbidden, drive.ReasonInsufficientFilePermissions)
	_, err = viewerDrive.UpdateContent(ctx, file, []byte("changed"))
	expectDriveError(t, "viewer upload", err, http.StatusForbidden, drive.ReasonInsufficientFilePermissions)
	renamed := "renamed"
	_, err = viewerDrive.Update(ctx, file, drive.Changes{Name: &renamed})
	expectDriveError(t, "viewer rename", err, http.StatusForbidden, drive.ReasonInsufficientFilePermissions)
	if content, _ := ownerDrive.Download(ctx, file, 0, -1); string(content) != "{}" {
		t.Fatalf("content changed to %q", content)
	}
}

// Roles are inherited from the folder: an Editor changes the owner's files
// in it, and the folder's owner holds writer on files the Editor creates
// there.
func TestRolesAreInheritedFromTheFolder(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, viewerDrive, folder := sharedFolder(t, e)
	devices, err := ownerDrive.CreateFolder(ctx, "devices", folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	ownerFile := mustCreate(t, ownerDrive, "manifest.json", devices.ID, []byte("owner"))
	editorFile := mustCreate(t, editorDrive, "segment-1", devices.ID, []byte("editor"))
	if _, err := editorDrive.UpdateContent(ctx, ownerFile, []byte("edited")); err != nil {
		t.Fatalf("editor upload to the owner's file: %v", err)
	}
	if _, err := ownerDrive.UpdateContent(ctx, editorFile, []byte("by owner")); err != nil {
		t.Fatalf("owner upload to the editor's file: %v", err)
	}
	if content, _ := viewerDrive.Download(ctx, editorFile, 0, -1); string(content) != "by owner" {
		t.Fatalf("viewer reads %q", content)
	}
	files, err := viewerDrive.List(ctx, fmt.Sprintf("'%s' in parents", devices.ID), "createdTime")
	if err != nil || !slices.Equal(names(files), []string{"manifest.json", "segment-1"}) {
		t.Fatalf("viewer lists %v, %v", names(files), err)
	}
}

// Reads take a byte range: a range past the end is cut to the end, and one
// starting past it is refused with 416; a folder has no content.
func TestRangedReads(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	file := mustCreate(t, ownerDrive, "segment-1", folder, []byte("0123456789"))
	for _, c := range []struct {
		offset, length int64
		want           string
	}{{0, -1, "0123456789"}, {2, 3, "234"}, {7, -1, "789"}, {8, 100, "89"}} {
		if content, err := ownerDrive.Download(ctx, file, c.offset, c.length); err != nil || string(content) != c.want {
			t.Fatalf("range %d+%d = %q, %v, want %q", c.offset, c.length, content, err, c.want)
		}
	}
	_, err := ownerDrive.Download(ctx, file, 10, -1)
	expectDriveError(t, "range past the end", err, http.StatusRequestedRangeNotSatisfiable, "requestedRangeNotSatisfiable")
	_, err = ownerDrive.Download(ctx, folder, 0, -1)
	expectDriveError(t, "download a folder", err, http.StatusForbidden, "fileNotDownloadable")
}

// Content is always uploaded whole: an upload replaces all of it.
func TestUploadsReplaceTheWholeContent(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	file := mustCreate(t, editorDrive, "segment-1", folder, []byte("first line\n"))
	if _, err := editorDrive.UpdateContent(ctx, file, []byte("first line\nsecond line\n")); err != nil {
		t.Fatal(err)
	}
	if content, _ := editorDrive.Download(ctx, file, 0, -1); string(content) != "first line\nsecond line\n" {
		t.Fatalf("content = %q", content)
	}
	f, err := editorDrive.UpdateContent(ctx, file, []byte("short"))
	if content, _ := editorDrive.Download(ctx, file, 0, -1); err != nil || string(content) != "short" || f.Size != 5 {
		t.Fatalf("after a shorter upload = %q, size %d, %v", content, f.Size, err)
	}
}

// A file is renamed and moved with addParents and removeParents; writing
// parents directly, which no correct client does, is refused.
func TestRenameAndMove(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	other, err := editorDrive.CreateFolder(ctx, "compaction", folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	file := mustCreate(t, editorDrive, "segment-1.tmp", folder, []byte("x"))
	name := "segment-1"
	f, err := editorDrive.Update(ctx, file, drive.Changes{Name: &name, AddParents: []string{other.ID}, RemoveParents: []string{folder}})
	if err != nil || f.Name != "segment-1" || !slices.Equal(f.Parents, []string{other.ID}) {
		t.Fatalf("after move = %+v, %v", f, err)
	}
	token := signIn(t, e, editor, "https://www.googleapis.com/auth/drive").AccessToken
	request, _ := http.NewRequest("PATCH", e.URL()+"/drive/v3/files/"+file, strings.NewReader(`{"parents":["`+folder+`"]}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("writing parents = %d", response.StatusCode)
	}
}

// Trashed files stay readable by ID and are listed unless the query asks
// for trashed = false; only the owner may trash.
func TestTrashedFilesStayReadableByID(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := mustCreate(t, editorDrive, "segment-1", folder, []byte("old"))
	ownerFile := mustCreate(t, ownerDrive, "log.json", folder, []byte("{}"))
	trash := true
	_, err := editorDrive.Update(ctx, ownerFile, drive.Changes{Trashed: &trash})
	expectDriveError(t, "editor trashes the owner's file", err, http.StatusForbidden, drive.ReasonInsufficientFilePermissions)
	if f, err := editorDrive.Update(ctx, file, drive.Changes{Trashed: &trash}); err != nil || !f.Trashed {
		t.Fatalf("trash = %+v, %v", f, err)
	}
	if content, err := ownerDrive.Download(ctx, file, 0, -1); err != nil || string(content) != "old" {
		t.Fatalf("read a trashed file = %q, %v", content, err)
	}
	listed := func(q string) []string {
		files, err := ownerDrive.List(ctx, q, "createdTime")
		if err != nil {
			t.Fatal(err)
		}
		return names(files)
	}
	if got := listed(fmt.Sprintf("'%s' in parents", folder)); !slices.Contains(got, "segment-1") {
		t.Fatalf("listing without trashed = false: %v", got)
	}
	if got := listed(fmt.Sprintf("'%s' in parents and trashed = false", folder)); !slices.Equal(got, []string{"log.json"}) {
		t.Fatalf("listing with trashed = false: %v", got)
	}
	if got := listed("trashed = true"); !slices.Equal(got, []string{"segment-1"}) {
		t.Fatalf("listing trashed files: %v", got)
	}
}

// The folder is found by its tag and its owner, whose address compares
// without regard to case, the oldest first; others' folders with the same
// tag are not.
func TestFolderFoundByTagAndOwner(t *testing.T) {
	clock := newTestClock()
	e := startEmulator(t, t.TempDir(), Options{Now: clock.Now})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	clock.Advance(time.Minute)
	if _, err := ownerDrive.CreateFolder(ctx, "eventlog", "root", map[string]string{"eventlog": "root"}); err != nil {
		t.Fatal(err)
	}
	impostor, err := editorDrive.CreateFolder(ctx, "eventlog", "root", map[string]string{"eventlog": "root"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Share(impostor.ID, owner, roleWriter); err != nil {
		t.Fatal(err)
	}
	search := fmt.Sprintf("appProperties has { key='eventlog' and value='root' } and '%s' in owners and trashed = false", strings.ToUpper(owner))
	found, err := editorDrive.List(ctx, search, "createdTime")
	if err != nil || len(found) != 1 || found[0].ID != folder {
		t.Fatalf("editor finds %v, %v; want only the shared folder", found, err)
	}
	found, err = ownerDrive.List(ctx, search, "createdTime")
	if err != nil || len(found) != 2 || found[0].ID != folder {
		t.Fatalf("owner finds %v, %v; want both, oldest first", found, err)
	}
}

// A listing larger than the page size spans pages, which the client
// follows; an unsupported query is refused.
func TestListingPagesAndRefusals(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{MaxPageSize: 2})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	for i := 0; i < 5; i++ {
		mustCreate(t, ownerDrive, fmt.Sprintf("file-%d", i), folder, nil)
	}
	e.ResetStats()
	files, err := ownerDrive.List(ctx, fmt.Sprintf("'%s' in parents", folder), "createdTime")
	if err != nil || !slices.Equal(names(files), []string{"file-0", "file-1", "file-2", "file-3", "file-4"}) {
		t.Fatalf("pages = %v, %v", names(files), err)
	}
	if pages := e.Stats()[owner]["drive.files.list"].Requests; pages != 3 {
		t.Fatalf("listing took %d pages, want 3", pages)
	}
	for _, query := range []string{"name contains 'file'", "fullText contains 'x'", "name = 'x' or name = 'y'", "name = 'unterminated"} {
		_, err := ownerDrive.List(ctx, query, "")
		expectDriveError(t, query, err, http.StatusBadRequest, "invalid")
	}
}

// Content larger than a simple upload allows is refused by the client
// before it is sent.
func TestLargeContentNeedsResumableUpload(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	e.ResetStats()
	if _, err := ownerDrive.Create(ctx, "big", folder, bytes.Repeat([]byte("x"), drive.MaxSimpleUpload+1)); err != drive.ErrTooLarge {
		t.Fatalf("large create = %v", err)
	}
	if calls := e.Stats()[owner]; len(calls) != 0 {
		t.Fatalf("calls made = %v", calls)
	}
}
