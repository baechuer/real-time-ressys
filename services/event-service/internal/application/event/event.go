package event

import (
	"context"
	"strings"
	"time"
)

const (
	EventVersion  = 1
	EventProducer = "event-service"
)

// DomainEventEnvelope is the stable contract for all domain events emitted by event-service.
// Consumers should rely on: version/producer/occurred_at + payload.
// trace_id is optional but strongly recommended for observability.
type DomainEventEnvelope[T any] struct {
	Version    int       `json:"version"`
	Producer   string    `json:"producer"`
	TraceID    string    `json:"trace_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	Payload    T         `json:"payload"`
}

// EventPublishedPayload is the business payload for routing key: event.published
type EventPublishedPayload struct {
	EventID   string    `json:"event_id"`
	OwnerID   string    `json:"owner_id"`
	City      string    `json:"city"`
	Category  string    `json:"category"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Capacity  int       `json:"capacity"`
	Status    string    `json:"status"`
}

// EventCanceledPayload is the business payload for routing key: event.canceled
type EventCanceledPayload struct {
	EventID   string    `json:"event_id"`
	OwnerID   string    `json:"owner_id"`
	City      string    `json:"city"`
	Category  string    `json:"category"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Capacity  int       `json:"capacity"`
	Status    string    `json:"status"`
}

// ---- trace id plumbing ----
// Minimal and decoupled: if transport layer stores a request id in context,
// we read it here. If not present, trace_id will be omitted.
//
// NOTE: You can later align this with chi middleware key if you want,
// but keeping it local avoids importing transport/middleware packages here.
type ctxKey string

const ctxRequestID ctxKey = "request_id"

// WithRequestID can be called by HTTP middleware to inject request_id into context.
func WithRequestID(ctx context.Context, id string) context.Context {
	id = strings.TrimSpace(id)
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxRequestID, id)
}

// TraceIDFromContext reads request_id if available.
func TraceIDFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxRequestID); v != nil {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
