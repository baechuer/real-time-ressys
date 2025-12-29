package contracts

import "time"

type EventCanceledEmailPayload struct {
	EventID    string    `json:"event_id"`
	UserID     string    `json:"user_id"`
	PrevStatus string    `json:"prev_status"` // "active" | "waitlisted"
	Reason     string    `json:"reason"`      // "event_canceled"
	OccurredAt time.Time `json:"occurred_at"`

	TraceID     string `json:"trace_id,omitempty"`
	Producer    string `json:"producer"`     // "join-service"
	EventAction string `json:"event_action"` // "canceled"
}
