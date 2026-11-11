package rbac

// PermissionCache caches resolved permissions per role name.
type PermissionCache interface {
	// Get returns cached permissions for a role, or ok=false on cache miss or expiration.
	Get(roleName string) (permissions []string, ok bool)
	// Set stores permissions for a role name.
	Set(roleName string, permissions []string)
	// Invalidate removes a single role's cached permissions.
	Invalidate(roleName string)
	// InvalidateAll clears all cached permissions.
	InvalidateAll()
}
