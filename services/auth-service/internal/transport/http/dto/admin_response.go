package dto

type RevokeSessionsData struct {
	Status string `json:"status"` // "revoked"
	UserID string `json:"user_id"`
}
