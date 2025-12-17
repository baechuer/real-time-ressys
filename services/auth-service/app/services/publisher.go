package services

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

// getRequestIDFromContext extracts request ID from context (avoiding import cycle)
func getRequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

// EventPublisher defines the minimal interface AuthService needs to publish events.
type EventPublisher interface {
	PublishEmailVerification(ctx context.Context, email string, verificationURL string) error
	PublishPasswordReset(ctx context.Context, email string, resetURL string) error
}

// RabbitMQPublisher is a concrete implementation using RabbitMQ.
type RabbitMQPublisher struct {
	ch *amqp.Channel
}

func NewRabbitMQPublisher(ch *amqp.Channel) *RabbitMQPublisher {
	return &RabbitMQPublisher{ch: ch}
}

type emailVerificationMessage struct {
	Type            string `json:"type"`
	Email           string `json:"email"`
	VerificationURL string `json:"verification_url"`
}

// PublishEmailVerification publishes an email verification event to the auth.events exchange.
func (p *RabbitMQPublisher) PublishEmailVerification(ctx context.Context, email string, verificationURL string) error {
	msg := emailVerificationMessage{
		Type:            "email_verification",
		Email:           email,
		VerificationURL: verificationURL,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Extract request ID from context for distributed tracing
	requestID := getRequestIDFromContext(ctx)

	headers := make(amqp.Table)
	if requestID != "" {
		headers["X-Request-ID"] = requestID
		headers["X-Trace-ID"] = requestID // Also set trace ID for compatibility
	}

	return p.ch.PublishWithContext(
		ctx,
		"auth.events",        // exchange
		"email.verification", // routing key
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     headers,
		},
	)
}

type passwordResetMessage struct {
	Type     string `json:"type"`
	Email    string `json:"email"`
	ResetURL string `json:"reset_url"`
}

// PublishPasswordReset publishes a password reset event to the auth.events exchange.
func (p *RabbitMQPublisher) PublishPasswordReset(ctx context.Context, email string, resetURL string) error {
	msg := passwordResetMessage{
		Type:     "password_reset",
		Email:    email,
		ResetURL: resetURL,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Extract request ID from context for distributed tracing
	requestID := getRequestIDFromContext(ctx)

	headers := make(amqp.Table)
	if requestID != "" {
		headers["X-Request-ID"] = requestID
		headers["X-Trace-ID"] = requestID // Also set trace ID for compatibility
	}

	return p.ch.PublishWithContext(
		ctx,
		"auth.events",          // exchange
		"email.password_reset", // routing key
		false,                  // mandatory
		false,                  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     headers,
		},
	)
}
