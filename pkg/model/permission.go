package model

// Permission represents a granular authorization capability.
type Permission string

const (
	// Page designs (barcode label printing)
	PermDesignRead   Permission = "design:read"
	PermDesignCreate Permission = "design:create"
	PermDesignUpdate Permission = "design:update"
	PermDesignDelete Permission = "design:delete"

	// Label layout presets
	PermPresetRead   Permission = "preset:read"
	PermPresetCreate Permission = "preset:create"
	PermPresetUpdate Permission = "preset:update"
	PermPresetDelete Permission = "preset:delete"

	// Accurate Online integration
	// read = status/databases/items/sources/documents
	// create = login (establish session)
	// update = select database (mutate session)
	// delete = logout (destroy session)
	PermAccurateRead   Permission = "accurate:read"
	PermAccurateCreate Permission = "accurate:create"
	PermAccurateUpdate Permission = "accurate:update"
	PermAccurateDelete Permission = "accurate:delete"

	// User management (admin)
	PermUserRead   Permission = "user:read"
	PermUserCreate Permission = "user:create" // approve guest
	PermUserUpdate Permission = "user:update" // change role
	PermUserDelete Permission = "user:delete"

	// Role management (admin)
	PermRoleRead   Permission = "role:read"
	PermRoleCreate Permission = "role:create"
	PermRoleUpdate Permission = "role:update"
	PermRoleDelete Permission = "role:delete"

	// System diagnostics
	PermSystemRead Permission = "system:read"
)

// AllPermissions is the canonical registry used for validation and UI display.
var AllPermissions = []Permission{
	PermDesignRead, PermDesignCreate, PermDesignUpdate, PermDesignDelete,
	PermPresetRead, PermPresetCreate, PermPresetUpdate, PermPresetDelete,
	PermAccurateRead, PermAccurateCreate, PermAccurateUpdate, PermAccurateDelete,
	PermUserRead, PermUserCreate, PermUserUpdate, PermUserDelete,
	PermRoleRead, PermRoleCreate, PermRoleUpdate, PermRoleDelete,
	PermSystemRead,
}

// ValidPermission checks if a string is in the registry.
func ValidPermission(s string) bool {
	for _, p := range AllPermissions {
		if string(p) == s {
			return true
		}
	}
	return false
}
