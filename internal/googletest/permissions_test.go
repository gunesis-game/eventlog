package googletest

import (
	"net/http"
	"testing"

	"github.com/gunesis-game/eventlog/internal/drive"
)

// permissionOf finds an account's permission on a file.
func permissionOf(t *testing.T, d *drive.Client, file, email string) drive.Permission {
	t.Helper()
	permissions, err := d.Permissions(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range permissions {
		if p.EmailAddress == email {
			return p
		}
	}
	t.Fatalf("no permission for %s in %+v", email, permissions)
	return drive.Permission{}
}

// D-069: the ownership check's steps, as observed on real Drive. The editor
// creates a file and owns it; sharing it with the owner returns the owner's
// inherited writer permission unchanged; updating that permission with
// pendingOwner offers ownership; the owner accepts with role owner and
// transferOwnership; afterwards the editor can change the content but cannot
// trash the file.
func TestOwnershipHandover(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := mustCreate(t, editorDrive, "segment-1", folder, []byte("v1"))

	created, err := editorDrive.Share(ctx, file, owner, "writer")
	if err != nil || created.Role != roleWriter || created.PendingOwner {
		t.Fatalf("share with the owner = %+v, %v; want the inherited writer permission unchanged", created, err)
	}
	inherited := permissionOf(t, editorDrive, file, owner)
	if inherited.ID != created.ID || !inherited.Inherited {
		t.Fatalf("owner's permission = %+v", inherited)
	}
	if offered, err := editorDrive.OfferOwnership(ctx, file, created.ID); err != nil || !offered.PendingOwner {
		t.Fatalf("offer = %+v, %v", offered, err)
	}
	if !permissionOf(t, ownerDrive, file, owner).PendingOwner {
		t.Fatal("the owner sees no pending offer")
	}
	if accepted, err := ownerDrive.AcceptOwnership(ctx, file, created.ID); err != nil || accepted.Role != roleOwner {
		t.Fatalf("accept = %+v, %v", accepted, err)
	}
	if permissionOf(t, ownerDrive, file, editor).Role != roleWriter {
		t.Fatal("the editor did not become a writer")
	}
	after, err := editorDrive.Get(ctx, file)
	if err != nil || after.Owners[0].EmailAddress != owner || !after.Capabilities.CanModifyContent || after.Capabilities.CanTrash || after.Capabilities.CanDelete || !after.Capabilities.CanShare {
		t.Fatalf("after the transfer = %+v, %v", after, err)
	}
	if _, err := editorDrive.UpdateContent(ctx, file, []byte("v2")); err != nil {
		t.Fatalf("editor upload after the transfer: %v", err)
	}
	trash := true
	_, err = editorDrive.Update(ctx, file, drive.Changes{Trashed: &trash})
	expectDriveError(t, "editor trash after the transfer", err, http.StatusForbidden, drive.ReasonInsufficientFilePermissions)
}

// Ownership moves only when the pending owner accepts; nobody else can
// offer or take it, and a reader cannot list permissions.
func TestOwnershipTransferRefusals(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, viewerDrive, folder := sharedFolder(t, e)
	file := mustCreate(t, editorDrive, "segment-1", folder, []byte("v1"))
	ownerID := permissionOf(t, editorDrive, file, owner).ID
	refused := func(what string, err error) {
		t.Helper()
		expectDriveError(t, what, err, http.StatusForbidden, drive.ReasonInsufficientFilePermissions)
	}
	_, err := ownerDrive.AcceptOwnership(ctx, file, ownerID)
	refused("accept without an offer", err)
	_, err = ownerDrive.OfferOwnership(ctx, file, ownerID)
	refused("an offer by a non-owner", err)
	_, err = editorDrive.OfferOwnership(ctx, file, permissionOf(t, editorDrive, file, viewer).ID)
	refused("an offer to a reader", err)
	if _, err := editorDrive.OfferOwnership(ctx, file, ownerID); err != nil {
		t.Fatalf("offer: %v", err)
	}
	_, err = viewerDrive.AcceptOwnership(ctx, file, ownerID)
	refused("accept by another account", err)
	_, err = viewerDrive.Permissions(ctx, file)
	refused("a reader lists permissions", err)
	if f, _ := editorDrive.Get(ctx, file); f.Owners[0].EmailAddress != editor {
		t.Fatalf("ownership moved: %+v", f.Owners)
	}
}

// Accepting needs transferOwnership; a request without it, which no correct
// client sends, is refused.
func TestOwnershipTransferNeedsAcknowledgement(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	_, editorDrive, _, folder := sharedFolder(t, e)
	file := mustCreate(t, editorDrive, "segment-1", folder, []byte("v1"))
	ownerID := permissionOf(t, editorDrive, file, owner).ID
	if _, err := editorDrive.OfferOwnership(ctx, file, ownerID); err != nil {
		t.Fatal(err)
	}
	token := signIn(t, e, owner, "https://www.googleapis.com/auth/drive").AccessToken
	request, _ := http.NewRequest("PATCH", e.URL()+"/drive/v3/files/"+file+"/permissions/"+ownerID, stringsReader(`{"role":"owner"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("accept without transferOwnership = %d", response.StatusCode)
	}
}

// Sharing through the API gives an account access, also to the folder's
// files, whatever capitals its address is written with; sharing with an
// address that is no Google account is refused.
func TestSharingThroughTheAPI(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	file := mustCreate(t, ownerDrive, "log.json", folder, []byte("{}"))
	e.AddTestUser("new@example.com")
	newcomer := driveAs(t, e, "new@example.com")
	_, err := newcomer.Download(ctx, file, 0, -1)
	expectDriveError(t, "before sharing", err, http.StatusNotFound, drive.ReasonNotFound)
	if p, err := ownerDrive.Share(ctx, folder, "New@Example.com", "reader"); err != nil || p.Role != roleReader || p.EmailAddress != "new@example.com" {
		t.Fatalf("share = %+v, %v", p, err)
	}
	if content, err := newcomer.Download(ctx, file, 0, -1); err != nil || string(content) != "{}" {
		t.Fatalf("after sharing = %q, %v", content, err)
	}
	_, err = ownerDrive.Share(ctx, folder, "nobody@example.com", "reader")
	expectDriveError(t, "share with an unknown address", err, http.StatusBadRequest, "invalidSharingRequest")
}
