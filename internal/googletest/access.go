package googletest

import "strings"

const (
	roleReader = "reader"
	roleWriter = "writer"
	roleOwner  = "owner"
)

func rank(role string) int {
	switch role {
	case roleReader:
		return 1
	case roleWriter:
		return 2
	case roleOwner:
		return 3
	default:
		return 0
	}
}

// roleOfLocked returns an account's effective role on f, or "" when it has
// no access: owner, its own permission on f, or the role it holds on a
// folder above f, where owning that folder counts as writer.
func (e *Emulator) roleOfLocked(f *file, account string) string {
	if f.Owner == account {
		return roleOwner
	}
	best := ""
	if p := f.permission(account); p != nil {
		best = p.Role
	}
	if inherited := e.inheritedRoleLocked(f, account); rank(inherited) > rank(best) {
		best = inherited
	}
	return best
}

// inheritedRoleLocked returns the role an account holds on f through the
// folders above it.
func (e *Emulator) inheritedRoleLocked(f *file, account string) string {
	best := ""
	for _, id := range f.Parents {
		parent := e.state.Files[id]
		if parent == nil {
			continue
		}
		role := e.roleOfLocked(parent, account)
		if role == roleOwner {
			role = roleWriter
		}
		if rank(role) > rank(best) {
			best = role
		}
	}
	return best
}

// capabilitiesLocked reports what an account may do with f, as Drive's
// capabilities field does: only the owner may trash or delete.
func (e *Emulator) capabilitiesLocked(f *file, account string) map[string]bool {
	role := e.roleOfLocked(f, account)
	return map[string]bool{
		"canModifyContent": rank(role) >= rank(roleWriter),
		"canShare":         rank(role) >= rank(roleWriter),
		"canTrash":         role == roleOwner,
		"canDelete":        role == roleOwner,
	}
}

func (f *file) permission(account string) *permission {
	for _, p := range f.Permissions {
		if p.Email == account {
			return p
		}
	}
	return nil
}

// setPermission gives account a direct role on f, replacing its earlier one.
func (f *file) setPermission(account, role string) *permission {
	if p := f.permission(account); p != nil {
		p.Role = role
		return p
	}
	p := &permission{Email: account, Role: role}
	f.Permissions = append(f.Permissions, p)
	return p
}

func (f *file) removePermission(account string) {
	kept := f.Permissions[:0]
	for _, p := range f.Permissions {
		if p.Email != account {
			kept = append(kept, p)
		}
	}
	f.Permissions = kept
}

// accountByPermissionIDLocked finds the account a permission ID belongs to.
func (e *Emulator) accountByPermissionIDLocked(id string) string {
	for key, a := range e.state.Accounts {
		if a.PermissionID == id {
			return key
		}
	}
	return ""
}

func sameEmail(a, b string) bool {
	return strings.EqualFold(a, b)
}
