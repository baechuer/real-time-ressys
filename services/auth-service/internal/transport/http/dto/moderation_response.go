package dto

// BanUserData is returned by ban endpoint.
type BanUserData struct {
	Status string `json:"status"` // "banned"
	UserID string `json:"user_id"`
}

// UnbanUserData is returned by unban endpoint.
type UnbanUserData struct {
	Status string `json:"status"` // "unbanned"
	UserID string `json:"user_id"`
}
