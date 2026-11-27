package googletest

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
)

// permissionRequest is the body of a permissions create or update request.
type permissionRequest struct {
	Type         string `json:"type"`
	Role         string `json:"role"`
	EmailAddress string `json:"emailAddress"`
	PendingOwner *bool  `json:"pendingOwner"`
}

// permissionResourceLocked renders an account's effective permission on f.
func (e *Emulator) permissionResourceLocked(f *file, account string) map[string]any {
	a := e.state.Accounts[account]
	role := e.roleOfLocked(f, account)
	direct := f.permission(account)
	pending := direct != nil && direct.PendingOwner
	inherited := f.Owner != account && (direct == nil || rank(e.inheritedRoleLocked(f, account)) >= rank(direct.Role))
	return map[string]any{
		"kind":              "drive#permission",
		"id":                a.PermissionID,
		"type":              "user",
		"emailAddress":      a.Email,
		"role":              role,
		"pendingOwner":      pending,
		"permissionDetails": []any{map[string]any{"permissionType": "file", "role": role, "inherited": inherited}},
	}
}

// holdersLocked returns every account with access to f, the owner first and
// the rest by email address.
func (e *Emulator) holdersLocked(f *file) []string {
	var holders []string
	for key := range e.state.Accounts {
		if key != f.Owner && e.roleOfLocked(f, key) != "" {
			holders = append(holders, key)
		}
	}
	slices.Sort(holders)
	return append([]string{f.Owner}, holders...)
}

// listPermissions is permissions.list: every account's effective
// permission, inherited ones included. It needs the writer role.
func (e *Emulator) listPermissions(w http.ResponseWriter, r *http.Request, c *call) {
	c.name = "drive.permissions.list"
	fields, err := fieldsOf(r, defaultPermissionList)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	f, role := e.lookupLocked(w, r.PathValue("id"), c.account)
	if f == nil {
		return
	}
	if rank(role) < rank(roleWriter) {
		driveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have sufficient permissions for this file.")
		return
	}
	permissions := []any{}
	for _, holder := range e.holdersLocked(f) {
		permissions = append(permissions, e.permissionResourceLocked(f, holder))
	}
	writeJSON(w, http.StatusOK, selectFields(map[string]any{"kind": "drive#permissionList", "permissions": permissions}, fields))
}

// createPermission is permissions.create for a user. Asking for access the
// account already has returns its existing permission unchanged, as Drive
// does for the owner's inherited permission.
func (e *Emulator) createPermission(w http.ResponseWriter, r *http.Request, c *call) {
	c.name = "drive.permissions.create"
	var request permissionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		driveError(w, http.StatusBadRequest, "badRequest", err.Error())
		return
	}
	fields, err := fieldsOf(r, defaultPermissionFields)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	if request.Type != "user" || rank(request.Role) == 0 || request.Role == roleOwner {
		driveError(w, http.StatusBadRequest, "invalidSharingRequest", "Only reader and writer permissions for a user can be created.")
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	f, role := e.lookupLocked(w, r.PathValue("id"), c.account)
	if f == nil {
		return
	}
	if rank(role) < rank(roleWriter) {
		driveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have sufficient permissions for this file.")
		return
	}
	target := strings.ToLower(request.EmailAddress)
	if e.state.Accounts[target] == nil {
		driveError(w, http.StatusBadRequest, "invalidSharingRequest", "Bad Request. User message: \"There is no Google account associated with this email address.\"")
		return
	}
	if rank(e.roleOfLocked(f, target)) < rank(request.Role) {
		f.setPermission(target, request.Role)
		e.saveLocked()
	}
	writeJSON(w, http.StatusOK, selectFields(e.permissionResourceLocked(f, target), fields))
}

// updatePermission is permissions.update. The owner may change an account's
// role or offer it ownership with pendingOwner; the pending owner accepts by
// updating its own permission to role owner with transferOwnership=true,
// after which the earlier owner is a writer.
func (e *Emulator) updatePermission(w http.ResponseWriter, r *http.Request, c *call) {
	c.name = "drive.permissions.update"
	var request permissionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		driveError(w, http.StatusBadRequest, "badRequest", err.Error())
		return
	}
	fields, err := fieldsOf(r, defaultPermissionFields)
	if err != nil {
		driveError(w, http.StatusBadRequest, "invalidParameter", "Invalid field selection: "+err.Error())
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	f, _ := e.lookupLocked(w, r.PathValue("id"), c.account)
	if f == nil {
		return
	}
	target := e.accountByPermissionIDLocked(r.PathValue("permissionID"))
	if target == "" || e.roleOfLocked(f, target) == "" {
		driveError(w, http.StatusNotFound, "notFound", "Permission not found: "+r.PathValue("permissionID")+".")
		return
	}
	refuse := func() {
		driveError(w, http.StatusForbidden, "insufficientFilePermissions", "The user does not have sufficient permissions for this file.")
	}
	switch {
	case request.Role == roleOwner:
		if r.URL.Query().Get("transferOwnership") != "true" {
			driveError(w, http.StatusForbidden, "forbidden", "The transferOwnership parameter must be enabled when the permission role is 'owner'.")
			return
		}
		direct := f.permission(target)
		if c.account != target || direct == nil || !direct.PendingOwner {
			refuse()
			return
		}
		earlier := f.Owner
		f.Owner = target
		f.removePermission(target)
		f.setPermission(earlier, roleWriter)
	case request.PendingOwner != nil:
		if c.account != f.Owner || target == f.Owner || e.roleOfLocked(f, target) != roleWriter {
			refuse()
			return
		}
		f.setPermission(target, roleWriter).PendingOwner = *request.PendingOwner
	case request.Role == roleReader || request.Role == roleWriter:
		if c.account != f.Owner || target == f.Owner {
			refuse()
			return
		}
		f.setPermission(target, request.Role)
	default:
		driveError(w, http.StatusBadRequest, "invalidSharingRequest", "Nothing to update.")
		return
	}
	e.saveLocked()
	writeJSON(w, http.StatusOK, selectFields(e.permissionResourceLocked(f, target), fields))
}
