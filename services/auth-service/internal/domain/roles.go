package domain

type Role string

const (
	// User can create and manage self created events
	RoleUser Role = "user"
	// Moderator can create and manage events created by other users
	RoleModerator Role = "moderator"
	// Admin users can conduct event management also other higher level priveleges like user management, banning users etc.
	RoleAdmin Role = "admin"
)

func IsValidRole(r string) bool {
	return r == string(RoleUser) || r == string(RoleModerator) || r == string(RoleAdmin)
}

// RoleRank: bigger => higher privilege
func RoleRank(r string) int {
	switch r {
	case string(RoleUser):
		return 1
	case string(RoleModerator):
		return 2
	case string(RoleAdmin):
		return 3
	default:
		return 0
	}
}
