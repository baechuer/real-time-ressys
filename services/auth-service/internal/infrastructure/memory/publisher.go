package memory

import (
	"context"
	"log"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
)

type NoopPublisher struct{}

func NewNoopPublisher() *NoopPublisher { return &NoopPublisher{} }

func (p *NoopPublisher) PublishVerifyEmail(ctx context.Context, evt auth.VerifyEmailEvent) error {
	log.Printf("[noop-pub] verify email: user_id=%s email=%s url=%s", evt.UserID, evt.Email, evt.URL)
	return nil
}

func (p *NoopPublisher) PublishPasswordReset(ctx context.Context, evt auth.PasswordResetEvent) error {
	log.Printf("[noop-pub] password reset: user_id=%s email=%s url=%s", evt.UserID, evt.Email, evt.URL)
	return nil
}
