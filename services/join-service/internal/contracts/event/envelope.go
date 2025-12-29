// PATH: services/join-service/internal/contracts/event/envelope.go
package event

import "time"

// DomainEventEnvelope is the canonical envelope consumed across services.
// NOTE: message_id is optional for backward compatibility.
type DomainEventEnvelope[T any] struct {
	Version    int       `json:"version"`
	Producer   string    `json:"producer"`
	TraceID    string    `json:"trace_id,omitempty"`
	MessageID  string    `json:"message_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	Payload    T         `json:"payload"`
}

// EventPublishedPayload / EventUpdatedPayload
// Keep fields tolerant: extra fields from producer are ignored by json.Unmarshal.
type EventPublishedPayload struct {
	EventID  string `json:"event_id"`
	Capacity *int   `json:"capacity,omitempty"` // pointer so we can detect missing
	Status   string `json:"status,omitempty"`   // e.g. published/canceled
}

type EventUpdatedPayload = EventPublishedPayload

// EventCanceledPayload
// Accept both event_id and legacy id for robustness.
type EventCanceledPayload struct {
	EventID string `json:"event_id,omitempty"`
	ID      string `json:"id,omitempty"`     // legacy / older producer
	Status  string `json:"status,omitempty"` // optional
	Reason  string `json:"reason,omitempty"` // optional
}
