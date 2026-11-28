package drive_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gunesis-game/eventlog/internal/drive"
	"github.com/gunesis-game/eventlog/internal/googleauth"
	"github.com/gunesis-game/eventlog/internal/googletest"
)

const (
	owner  = "owner@example.com"
	editor = "editor@example.com"
)

// signedIn signs an account in through the emulator and returns a Drive
// client for it, pointed at the emulator.
func signedIn(t *testing.T, e *googletest.Emulator, email string) *drive.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "client.json")
	if err := e.WriteClientFile(path); err != nil {
		t.Fatal(err)
	}
	client, err := googleauth.ReadClientFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a := client.Authorize("http://127.0.0.1:8765", []string{googleauth.ScopeDrive}, email)
	redirect, err := e.Browse(a.URL, email)
	if err != nil {
		t.Fatal(err)
	}
	code, err := a.Code(redirect)
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.Exchange(context.Background(), http.DefaultClient, a, code)
	if err != nil {
		t.Fatal(err)
	}
	return drive.New(http.DefaultClient, e.URL(), func(context.Context) (string, error) { return token.AccessToken, nil })
}

func emulator(t *testing.T) *googletest.Emulator {
	t.Helper()
	e, err := googletest.New(t.TempDir(), googletest.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	e.AddTestUser(owner)
	e.AddTestUser(editor)
	return e
}

var ctx = context.Background()

// Files are created, read whole or in ranges, replaced, renamed, moved,
// and trashed, with their metadata decoded, sizes included.
func TestFileCalls(t *testing.T) {
	e := emulator(t)
	d := signedIn(t, e, owner)
	folder, err := d.CreateFolder(ctx, "eventlog", "root", map[string]string{"eventlog": "root"})
	if err != nil {
		t.Fatal(err)
	}
	if !folder.IsFolder() || folder.AppProperties["eventlog"] != "root" || folder.Owners[0].EmailAddress != owner {
		t.Fatalf("folder = %+v", folder)
	}
	f, err := d.Create(ctx, "segment-1", folder.ID, []byte("0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	if f.Size != 10 || f.Parents[0] != folder.ID || f.CreatedTime.IsZero() || !f.Capabilities.CanTrash {
		t.Fatalf("file = %+v", f)
	}
	for _, c := range []struct {
		offset, length int64
		want           string
	}{{0, -1, "0123456789"}, {2, 3, "234"}, {7, -1, "789"}, {8, 100, "89"}, {4, 0, ""}} {
		got, err := d.Download(ctx, f.ID, c.offset, c.length)
		if err != nil || string(got) != c.want {
			t.Fatalf("download %d+%d = %q, %v; want %q", c.offset, c.length, got, err, c.want)
		}
	}
	if f, err = d.UpdateContent(ctx, f.ID, []byte("replaced")); err != nil || f.Size != 8 {
		t.Fatalf("update content = %+v, %v", f, err)
	}
	other, _ := d.CreateFolder(ctx, "other", folder.ID, nil)
	name, trashed := "segment-2", true
	f, err = d.Update(ctx, f.ID, drive.Changes{Name: &name, AddParents: []string{other.ID}, RemoveParents: []string{folder.ID}})
	if err != nil || f.Name != "segment-2" || f.Parents[0] != other.ID {
		t.Fatalf("rename and move = %+v, %v", f, err)
	}
	if f, err = d.Update(ctx, f.ID, drive.Changes{Trashed: &trashed}); err != nil || !f.Trashed {
		t.Fatalf("trash = %+v, %v", f, err)
	}
	if _, err := d.Download(ctx, f.ID, 0, -1); err != nil {
		t.Fatalf("a trashed file reads by ID: %v", err)
	}
}

// A file moved to the owner's trash remains readable by another account
 // that has access through the shared parent folder.
 func TestTrashedFileRemainsReadableBySharedEditor(t *testing.T) {
 	e := emulator(t)
 	ownerDrive, editorDrive := signedIn(t, e, owner), signedIn(t, e, editor)
 	folder, err := ownerDrive.CreateFolder(ctx, "eventlog", "root", nil)
 	if err != nil {
 		t.Fatal(err)
 	}
 	if _, err := ownerDrive.Share(ctx, folder.ID, editor, "writer"); err != nil {
 		t.Fatal(err)
 	}
 	f, err := ownerDrive.Create(ctx, "old-segment", folder.ID, []byte("committed history"))
 	if err != nil {
 		t.Fatal(err)
 	}
 	trashed := true
 	if _, err := ownerDrive.Update(ctx, f.ID, drive.Changes{Trashed: &trashed}); err != nil {
 		t.Fatal(err)
 	}
 	got, err := editorDrive.Download(ctx, f.ID, 0, -1)
 	if err != nil || string(got) != "committed history" {
 		t.Fatalf("shared editor read of trashed file = %q, %v", got, err)
 	}
 }

// List follows every page and keeps the requested order.
func TestListFollowsPages(t *testing.T) {
	e := emulator(t)
	d := signedIn(t, e, owner)
	folder, _ := d.CreateFolder(ctx, "eventlog", "root", nil)
	for i := 0; i < 5; i++ {
		if _, err := d.Create(ctx, fmt.Sprintf("file-%d", i), folder.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	files, err := d.List(ctx, fmt.Sprintf("'%s' in parents and trashed = false", folder.ID), "createdTime")
	if err != nil || len(files) != 5 || files[0].Name != "file-0" || files[4].Name != "file-4" {
		t.Fatalf("list = %d files, %v", len(files), err)
	}
}

// Drive errors are decoded with their status and reason.
func TestErrorsCarryTheReason(t *testing.T) {
	e := emulator(t)
	d := signedIn(t, e, owner)
	_, err := d.Get(ctx, "no-such-file")
	var driveErr *drive.Error
	if !errors.As(err, &driveErr) || driveErr.Status != http.StatusNotFound || !drive.HasReason(err, drive.ReasonNotFound) {
		t.Fatalf("missing file = %v", err)
	}
	folder, _ := d.CreateFolder(ctx, "eventlog", "root", nil)
	if _, err := signedIn(t, e, editor).Create(ctx, "x", folder.ID, nil); !drive.HasReason(err, drive.ReasonNotFound) {
		t.Fatalf("unshared folder = %v", err)
	}
	if _, err := d.Create(ctx, "big", folder.ID, bytes.Repeat([]byte("x"), drive.MaxSimpleUpload+1)); !errors.Is(err, drive.ErrTooLarge) {
		t.Fatalf("over the simple upload limit = %v", err)
	}
	expired := drive.New(http.DefaultClient, e.URL(), func(context.Context) (string, error) { return "expired", nil })
	if _, err := expired.Get(ctx, "root"); !drive.HasReason(err, drive.ReasonAuthError) {
		t.Fatalf("bad token = %v", err)
	}
}

// The permission calls: sharing, an ownership offer, and its acceptance.
func TestPermissionCalls(t *testing.T) {
	e := emulator(t)
	ownerDrive, editorDrive := signedIn(t, e, owner), signedIn(t, e, editor)
	folder, _ := ownerDrive.CreateFolder(ctx, "eventlog", "root", nil)
	shared, err := ownerDrive.Share(ctx, folder.ID, editor, "writer")
	if err != nil || shared.Role != "writer" || shared.EmailAddress != editor {
		t.Fatalf("share = %+v, %v", shared, err)
	}
	f, err := editorDrive.Create(ctx, "segment-1", folder.ID, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	permissions, err := editorDrive.Permissions(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	var ownerPermission drive.Permission
	for _, p := range permissions {
		if p.EmailAddress == owner {
			ownerPermission = p
		}
	}
	if !ownerPermission.Inherited || ownerPermission.Role != "writer" {
		t.Fatalf("owner's permission = %+v", ownerPermission)
	}
	if p, err := editorDrive.OfferOwnership(ctx, f.ID, ownerPermission.ID); err != nil || !p.PendingOwner {
		t.Fatalf("offer = %+v, %v", p, err)
	}
	if p, err := ownerDrive.AcceptOwnership(ctx, f.ID, ownerPermission.ID); err != nil || p.Role != "owner" {
		t.Fatalf("accept = %+v, %v", p, err)
	}
	after, err := editorDrive.Get(ctx, f.ID)
	if err != nil || after.Owners[0].EmailAddress != owner || after.Capabilities.CanTrash || !after.Capabilities.CanModifyContent {
		t.Fatalf("after the transfer = %+v, %v", after, err)
	}
}
