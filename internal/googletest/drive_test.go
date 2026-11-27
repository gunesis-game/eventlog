package googletest

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// sharedFolder sets up an owner's folder shared with an editor and a viewer,
// each signed in, and returns their clients and the folder's ID.
func sharedFolder(t *testing.T, e *Emulator) (ownerDrive, editorDrive, viewerDrive *driveClient, folder string) {
	t.Helper()
	for _, account := range []string{owner, editor, viewer} {
		e.AddTestUser(account)
	}
	ownerDrive, _ = signIn(t, e, owner, driveScope)
	editorDrive, _ = signIn(t, e, editor, driveScope)
	viewerDrive, _ = signIn(t, e, viewer, driveScope)
	folder = ownerDrive.createFolder("eventlog", "", map[string]string{"eventlog": "root"})
	if err := e.Share(folder, editor, roleWriter); err != nil {
		t.Fatal(err)
	}
	if err := e.Share(folder, viewer, roleReader); err != nil {
		t.Fatal(err)
	}
	return ownerDrive, editorDrive, viewerDrive, folder
}

// The creating account owns a file; Drive reports its size as a string and
// the owner's email address, and only the requested fields.
func TestCreatorOwnsFiles(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	id := editorDrive.mustUpload("segment-1", folder, []byte("hello"))
	status, body := editorDrive.call("GET", "/drive/v3/files/"+id+"?fields=id,name,size,parents,ownedByMe,owners(emailAddress)", nil)
	expectStatus(t, "get", status, http.StatusOK, body)
	owners := body["owners"].([]any)
	if body["size"] != "5" || body["ownedByMe"] != true || body["parents"].([]any)[0] != folder || len(owners) != 1 || owners[0].(map[string]any)["emailAddress"] != editor || len(body) != 6 {
		t.Fatalf("metadata = %v", body)
	}
	if _, plain := editorDrive.call("GET", "/drive/v3/files/"+id, nil); len(plain) != 4 || plain["kind"] != "drive#file" {
		t.Fatalf("default fields = %v", plain)
	}
}

// A folder that is not shared with an account is not found, nor is anything
// in it, and listing it finds nothing.
func TestUnsharedFolderIsNotFound(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	file := ownerDrive.mustUpload("log.json", folder, []byte("{}"))
	e.AddTestUser("stranger@example.com")
	stranger, _ := signIn(t, e, "stranger@example.com", driveScope)
	for _, id := range []string{folder, file} {
		status, body := stranger.call("GET", "/drive/v3/files/"+id, nil)
		expectRefused(t, "get "+id, status, body, http.StatusNotFound, "notFound")
	}
	status, body := stranger.upload("intruder", folder, []byte("x"))
	expectRefused(t, "create in the folder", status, body, http.StatusNotFound, "notFound")
	if names := stranger.list(q("'%s' in parents", folder)); len(names) != 0 {
		t.Fatalf("stranger lists %v", names)
	}
}

// A Viewer reads the folder's files but is refused every write.
func TestViewerIsRefusedWrites(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, viewerDrive, folder := sharedFolder(t, e)
	file := ownerDrive.mustUpload("log.json", folder, []byte("{}"))
	if status, content := viewerDrive.read(file, ""); status != http.StatusOK || string(content) != "{}" {
		t.Fatalf("viewer read = %d %q", status, content)
	}
	status, body := viewerDrive.upload("mine", folder, []byte("x"))
	expectRefused(t, "viewer create", status, body, http.StatusForbidden, "insufficientFilePermissions")
	status, body = viewerDrive.replaceContent(file, []byte("changed"))
	expectRefused(t, "viewer upload", status, body, http.StatusForbidden, "insufficientFilePermissions")
	status, body = viewerDrive.call("PATCH", "/drive/v3/files/"+file, map[string]any{"name": "renamed"})
	expectRefused(t, "viewer rename", status, body, http.StatusForbidden, "insufficientFilePermissions")
	if _, content := ownerDrive.read(file, ""); string(content) != "{}" {
		t.Fatalf("content changed to %q", content)
	}
}

// Roles are inherited from the folder: an Editor reads and changes the
// owner's files in it, and the folder's owner holds writer on files the
// Editor creates there.
func TestRolesAreInheritedFromTheFolder(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, viewerDrive, folder := sharedFolder(t, e)
	devices := ownerDrive.createFolder("devices", folder, nil)
	ownerFile := ownerDrive.mustUpload("manifest.json", devices, []byte("owner"))
	editorFile := editorDrive.mustUpload("segment-1", devices, []byte("editor"))
	if status, _ := editorDrive.replaceContent(ownerFile, []byte("edited")); status != http.StatusOK {
		t.Fatalf("editor upload to the owner's file: %d", status)
	}
	if status, _ := ownerDrive.replaceContent(editorFile, []byte("by owner")); status != http.StatusOK {
		t.Fatalf("owner upload to the editor's file: %d", status)
	}
	if _, content := viewerDrive.read(editorFile, ""); string(content) != "by owner" {
		t.Fatalf("viewer reads %q", content)
	}
	if names := viewerDrive.list(q("'%s' in parents", devices)); !slices.Equal(names, []string{"manifest.json", "segment-1"}) {
		t.Fatalf("viewer lists %v", names)
	}
}

// Reads take a byte range: a range past the end is cut to the end, and a
// range starting past it is refused with 416.
func TestRangedReads(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	file := ownerDrive.mustUpload("segment-1", folder, []byte("0123456789"))
	for _, c := range []struct {
		header string
		status int
		want   string
	}{
		{"", http.StatusOK, "0123456789"},
		{"bytes=2-4", http.StatusPartialContent, "234"},
		{"bytes=7-", http.StatusPartialContent, "789"},
		{"bytes=8-100", http.StatusPartialContent, "89"},
	} {
		if status, content := ownerDrive.read(file, c.header); status != c.status || string(content) != c.want {
			t.Fatalf("range %q = %d %q, want %d %q", c.header, status, content, c.status, c.want)
		}
	}
	status, _, _ := ownerDrive.do("GET", "/drive/v3/files/"+file+"?alt=media", nil, http.Header{"Range": {"bytes=10-"}})
	expectStatus(t, "range past the end", status, http.StatusRequestedRangeNotSatisfiable, nil)
	status, body := ownerDrive.call("GET", "/drive/v3/files/"+folder+"?alt=media", nil)
	expectRefused(t, "download a folder", status, body, http.StatusForbidden, "fileNotDownloadable")
}

// Content is always uploaded whole: an upload replaces all of it.
func TestUploadsReplaceTheWholeContent(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	file := editorDrive.mustUpload("segment-1", folder, []byte("first line\n"))
	status, body := editorDrive.replaceContent(file, []byte("first line\nsecond line\n"))
	expectStatus(t, "upload", status, http.StatusOK, body)
	if _, content := editorDrive.read(file, ""); string(content) != "first line\nsecond line\n" {
		t.Fatalf("content = %q", content)
	}
	editorDrive.replaceContent(file, []byte("short"))
	if _, content := editorDrive.read(file, ""); string(content) != "short" {
		t.Fatalf("content after a shorter upload = %q", content)
	}
	if _, meta := editorDrive.call("GET", "/drive/v3/files/"+file+"?fields=size", nil); meta["size"] != "5" {
		t.Fatalf("size = %v", meta)
	}
}

// A file is renamed and moved with addParents and removeParents; writing
// parents directly is refused.
func TestRenameAndMove(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	other := editorDrive.createFolder("compaction", folder, nil)
	file := editorDrive.mustUpload("segment-1.tmp", folder, []byte("x"))
	status, body := editorDrive.call("PATCH", "/drive/v3/files/"+file+"?"+url.Values{"addParents": {other}, "removeParents": {folder}, "fields": {"name,parents"}}.Encode(), map[string]any{"name": "segment-1"})
	expectStatus(t, "move", status, http.StatusOK, body)
	if body["name"] != "segment-1" || body["parents"].([]any)[0] != other {
		t.Fatalf("after move = %v", body)
	}
	status, body = editorDrive.call("PATCH", "/drive/v3/files/"+file, map[string]any{"parents": []string{folder}})
	expectRefused(t, "write parents", status, body, http.StatusForbidden, "parentsNotWritable")
}

// Trashed files stay readable by ID and are listed unless the query asks
// for trashed = false; only the owner may trash.
func TestTrashedFilesStayReadableByID(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := editorDrive.mustUpload("segment-1", folder, []byte("old"))
	ownerFile := ownerDrive.mustUpload("log.json", folder, []byte("{}"))
	status, body := editorDrive.call("PATCH", "/drive/v3/files/"+ownerFile, map[string]any{"trashed": true})
	expectRefused(t, "editor trashes the owner's file", status, body, http.StatusForbidden, "insufficientFilePermissions")
	status, body = editorDrive.call("PATCH", "/drive/v3/files/"+file+"?fields=trashed", map[string]any{"trashed": true})
	if status != http.StatusOK || body["trashed"] != true {
		t.Fatalf("trash = %d %v", status, body)
	}
	if status, content := ownerDrive.read(file, ""); status != http.StatusOK || string(content) != "old" {
		t.Fatalf("read a trashed file = %d %q", status, content)
	}
	if names := ownerDrive.list(q("'%s' in parents", folder)); !slices.Contains(names, "segment-1") {
		t.Fatalf("listing without trashed = false: %v", names)
	}
	if names := ownerDrive.list(q("'%s' in parents and trashed = false", folder)); !slices.Equal(names, []string{"log.json"}) {
		t.Fatalf("listing with trashed = false: %v", names)
	}
	if names := ownerDrive.list("trashed = true"); !slices.Equal(names, []string{"segment-1"}) {
		t.Fatalf("listing trashed files: %v", names)
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
	ownerDrive.createFolder("eventlog", "", map[string]string{"eventlog": "root"})
	impostor := editorDrive.createFolder("eventlog", "", map[string]string{"eventlog": "root"})
	if err := e.Share(impostor, owner, roleWriter); err != nil {
		t.Fatal(err)
	}
	search := url.Values{
		"q":       {q("appProperties has { key='eventlog' and value='root' } and '%s' in owners and trashed = false", strings.ToUpper(owner))},
		"orderBy": {"createdTime"},
		"fields":  {"files(id,owners(emailAddress))"},
	}.Encode()
	status, body := editorDrive.call("GET", "/drive/v3/files?"+search, nil)
	expectStatus(t, "search", status, http.StatusOK, body)
	files := body["files"].([]any)
	if len(files) != 1 || files[0].(map[string]any)["id"] != folder {
		t.Fatalf("editor finds %v, want only the shared folder %s", files, folder)
	}
	_, body = ownerDrive.call("GET", "/drive/v3/files?"+search, nil)
	if files := body["files"].([]any); len(files) != 2 || files[0].(map[string]any)["id"] != folder {
		t.Fatalf("owner finds %v, oldest first", files)
	}
}

// Listings come a page at a time, and unsupported queries and fields are
// refused.
func TestListingPagesAndRefusals(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	for i := 0; i < 5; i++ {
		ownerDrive.mustUpload(q("file-%d", i), folder, nil)
	}
	var names []string
	token := ""
	for pages := 0; ; pages++ {
		query := url.Values{"q": {q("'%s' in parents", folder)}, "pageSize": {"2"}, "fields": {"nextPageToken,files(name)"}}
		if token != "" {
			query.Set("pageToken", token)
		}
		_, body := ownerDrive.call("GET", "/drive/v3/files?"+query.Encode(), nil)
		for _, f := range body["files"].([]any) {
			names = append(names, f.(map[string]any)["name"].(string))
		}
		token, _ = body["nextPageToken"].(string)
		if token == "" {
			if pages != 2 {
				t.Fatalf("%d pages", pages+1)
			}
			break
		}
	}
	if !slices.Equal(names, []string{"file-0", "file-1", "file-2", "file-3", "file-4"}) {
		t.Fatalf("pages = %v", names)
	}
	for _, query := range []string{"name contains 'file'", "fullText contains 'x'", "name = 'x' or name = 'y'", "name = 'unterminated"} {
		status, body := ownerDrive.call("GET", "/drive/v3/files?"+url.Values{"q": {query}}.Encode(), nil)
		expectRefused(t, query, status, body, http.StatusBadRequest, "invalid")
	}
	status, body := ownerDrive.call("GET", "/drive/v3/files?fields=files(id", nil)
	expectRefused(t, "bad fields", status, body, http.StatusBadRequest, "invalidParameter")
}
