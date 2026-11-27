package googletest

import (
	"net/http"
	"testing"
)

// permissionOf finds an account's permission in a permissions listing.
func permissionOf(t *testing.T, drive *driveClient, file, email string) map[string]any {
	t.Helper()
	status, body := drive.call("GET", "/drive/v3/files/"+file+"/permissions?fields=permissions(id,emailAddress,role,pendingOwner,permissionDetails)", nil)
	expectStatus(t, "list permissions", status, http.StatusOK, body)
	for _, p := range body["permissions"].([]any) {
		if p := p.(map[string]any); p["emailAddress"] == email {
			return p
		}
	}
	t.Fatalf("no permission for %s in %v", email, body)
	return nil
}

// D-069: the ownership check's steps, as observed on real Drive. The editor
// creates a file and owns it; creating a permission for the owner returns
// the owner's inherited writer permission unchanged; updating it with
// pendingOwner offers ownership; the owner accepts with role owner and
// transferOwnership; afterwards the editor can change the content but cannot
// trash or delete the file.
func TestOwnershipHandover(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, _, folder := sharedFolder(t, e)
	file := editorDrive.mustUpload("segment-1", folder, []byte("v1"))

	status, created := editorDrive.call("POST", "/drive/v3/files/"+file+"/permissions?fields=id,role,pendingOwner,emailAddress", map[string]any{"type": "user", "role": "writer", "emailAddress": owner})
	expectStatus(t, "create the owner's permission", status, http.StatusOK, created)
	if created["role"] != roleWriter || created["pendingOwner"] != false {
		t.Fatalf("created = %v, want the inherited writer permission unchanged", created)
	}
	inherited := permissionOf(t, editorDrive, file, owner)
	if inherited["id"] != created["id"] || inherited["permissionDetails"].([]any)[0].(map[string]any)["inherited"] != true {
		t.Fatalf("owner's permission = %v", inherited)
	}

	status, offered := editorDrive.call("PATCH", "/drive/v3/files/"+file+"/permissions/"+created["id"].(string)+"?fields=pendingOwner", map[string]any{"pendingOwner": true})
	if status != http.StatusOK || offered["pendingOwner"] != true {
		t.Fatalf("offer = %d %v", status, offered)
	}
	pending := permissionOf(t, ownerDrive, file, owner)
	if pending["pendingOwner"] != true {
		t.Fatalf("owner sees %v, want a pending offer", pending)
	}

	status, accepted := ownerDrive.call("PATCH", "/drive/v3/files/"+file+"/permissions/"+created["id"].(string)+"?transferOwnership=true&fields=role", map[string]any{"role": "owner"})
	if status != http.StatusOK || accepted["role"] != roleOwner {
		t.Fatalf("accept = %d %v", status, accepted)
	}
	if permissionOf(t, ownerDrive, file, editor)["role"] != roleWriter {
		t.Fatal("the editor did not become a writer")
	}

	status, meta := editorDrive.call("GET", "/drive/v3/files/"+file+"?fields=owners(emailAddress),capabilities", nil)
	expectStatus(t, "verify", status, http.StatusOK, meta)
	capabilities := meta["capabilities"].(map[string]any)
	if meta["owners"].([]any)[0].(map[string]any)["emailAddress"] != owner || capabilities["canModifyContent"] != true || capabilities["canTrash"] != false || capabilities["canDelete"] != false || capabilities["canShare"] != true {
		t.Fatalf("after the transfer = %v", meta)
	}
	if status, body := editorDrive.replaceContent(file, []byte("v2")); status != http.StatusOK {
		t.Fatalf("editor upload after the transfer: %d %v", status, body)
	}
	status, body := editorDrive.call("PATCH", "/drive/v3/files/"+file, map[string]any{"trashed": true})
	expectRefused(t, "editor trash after the transfer", status, body, http.StatusForbidden, "insufficientFilePermissions")
}

// Ownership moves only when the pending owner accepts with
// transferOwnership; nobody else can offer or take it.
func TestOwnershipTransferRefusals(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, editorDrive, viewerDrive, folder := sharedFolder(t, e)
	file := editorDrive.mustUpload("segment-1", folder, []byte("v1"))
	ownerID := permissionOf(t, editorDrive, file, owner)["id"].(string)
	accept := "/drive/v3/files/" + file + "/permissions/" + ownerID + "?transferOwnership=true"

	status, body := ownerDrive.call("PATCH", accept, map[string]any{"role": "owner"})
	expectRefused(t, "accept without an offer", status, body, http.StatusForbidden, "insufficientFilePermissions")
	status, body = ownerDrive.call("PATCH", "/drive/v3/files/"+file+"/permissions/"+ownerID, map[string]any{"pendingOwner": true})
	expectRefused(t, "an offer by a non-owner", status, body, http.StatusForbidden, "insufficientFilePermissions")
	viewerID := permissionOf(t, editorDrive, file, viewer)["id"].(string)
	status, body = editorDrive.call("PATCH", "/drive/v3/files/"+file+"/permissions/"+viewerID, map[string]any{"pendingOwner": true})
	expectRefused(t, "an offer to a reader", status, body, http.StatusForbidden, "insufficientFilePermissions")

	if status, _ := editorDrive.call("PATCH", "/drive/v3/files/"+file+"/permissions/"+ownerID, map[string]any{"pendingOwner": true}); status != http.StatusOK {
		t.Fatalf("offer: %d", status)
	}
	status, body = ownerDrive.call("PATCH", "/drive/v3/files/"+file+"/permissions/"+ownerID, map[string]any{"role": "owner"})
	expectRefused(t, "accept without transferOwnership", status, body, http.StatusForbidden, "forbidden")
	status, body = viewerDrive.call("PATCH", accept, map[string]any{"role": "owner"})
	expectRefused(t, "accept by another account", status, body, http.StatusForbidden, "insufficientFilePermissions")
	status, body = viewerDrive.call("GET", "/drive/v3/files/"+file+"/permissions", nil)
	expectRefused(t, "a reader lists permissions", status, body, http.StatusForbidden, "insufficientFilePermissions")
	if _, meta := ownerDrive.call("GET", "/drive/v3/files/"+file+"?fields=ownedByMe", nil); meta["ownedByMe"] != false {
		t.Fatalf("ownership moved: %v", meta)
	}
}

// Sharing through the API: a writer gives an account access, which it then
// has on the folder's files too; sharing with an unknown address is refused.
func TestSharingThroughTheAPI(t *testing.T) {
	e := startEmulator(t, t.TempDir(), Options{})
	ownerDrive, _, _, folder := sharedFolder(t, e)
	file := ownerDrive.mustUpload("log.json", folder, []byte("{}"))
	e.AddTestUser("new@example.com")
	newcomer, _ := signIn(t, e, "new@example.com", driveScope)
	if status, _ := newcomer.read(file, ""); status != http.StatusNotFound {
		t.Fatalf("before sharing: %d", status)
	}
	status, body := ownerDrive.call("POST", "/drive/v3/files/"+folder+"/permissions?fields=role,emailAddress", map[string]any{"type": "user", "role": "reader", "emailAddress": "New@Example.com"})
	if status != http.StatusOK || body["role"] != roleReader || body["emailAddress"] != "new@example.com" {
		t.Fatalf("share = %d %v", status, body)
	}
	if status, content := newcomer.read(file, ""); status != http.StatusOK || string(content) != "{}" {
		t.Fatalf("after sharing: %d %q", status, content)
	}
	status, body = ownerDrive.call("POST", "/drive/v3/files/"+folder+"/permissions", map[string]any{"type": "user", "role": "reader", "emailAddress": "nobody@example.com"})
	expectRefused(t, "share with an unknown address", status, body, http.StatusBadRequest, "invalidSharingRequest")
}
