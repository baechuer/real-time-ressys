package dto

type SetUserRoleData struct {
	Status string `json:"status"` // "role_updated"
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
