package email

import (
	"context"
	"fmt"
	"time"

	"github.com/baechuer/real-time-ressys/services/email-service/app/circuitbreaker"
	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	"github.com/baechuer/real-time-ressys/services/email-service/app/email/providers"
	appErrors "github.com/baechuer/real-time-ressys/services/email-service/app/errors"
	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
)

// Sender handles email sending using the configured provider
type Sender struct {
	provider       Provider
	config         *config.EmailConfig
	circuitBreaker *circuitbreaker.CircuitBreaker
}

// NewSender creates a new email sender with the configured provider
func NewSender(cfg *config.EmailConfig) (*Sender, error) {
	var provider Provider
	var err error

	switch cfg.Provider {
	case "sendgrid":
		provider, err = providers.NewSendGridProvider(cfg)
	case "ses":
		provider, err = providers.NewSESProvider(cfg)
	case "smtp":
		provider, err = providers.NewSMTPProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported email provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize email provider: %w", err)
	}

	// Initialize circuit breaker
	// 5 failures before opening, 30s reset timeout, 2 calls in half-open
	cb := circuitbreaker.NewCircuitBreaker(5, 30*time.Second, 2)

	return &Sender{
		provider:       provider,
		config:         cfg,
		circuitBreaker: cb,
	}, nil
}

// SendVerificationEmail sends an email verification email
func (s *Sender) SendVerificationEmail(ctx context.Context, email, url string) error {
	subject := "Verify your email address"
	body, err := RenderVerificationTemplate(email, url)
	if err != nil {
		return appErrors.NewInternal("failed to render verification template")
	}

	emailModel := &models.Email{
		To:       email,
		Subject:  subject,
		Body:     body,
		From:     s.config.FromEmail,
		FromName: s.config.FromName,
	}

	// Use circuit breaker to protect against provider failures
	err = s.circuitBreaker.Call(ctx, func() error {
		return s.provider.SendEmail(ctx, emailModel)
	})
	if err != nil {
		return appErrors.NewEmailProviderError("failed to send verification email", err)
	}

	return nil
}

// SendPasswordResetEmail sends a password reset email
func (s *Sender) SendPasswordResetEmail(ctx context.Context, email, url string) error {
	subject := "Reset your password"
	body, err := RenderPasswordResetTemplate(email, url)
	if err != nil {
		return appErrors.NewInternal("failed to render password reset template")
	}

	emailModel := &models.Email{
		To:       email,
		Subject:  subject,
		Body:     body,
		From:     s.config.FromEmail,
		FromName: s.config.FromName,
	}

	// Use circuit breaker to protect against provider failures
	err = s.circuitBreaker.Call(ctx, func() error {
		return s.provider.SendEmail(ctx, emailModel)
	})
	if err != nil {
		return appErrors.NewEmailProviderError("failed to send password reset email", err)
	}

	return nil
}

// ProviderName returns the name of the email provider
func (s *Sender) ProviderName() string {
	if s.provider == nil {
		return "unknown"
	}
	return s.provider.Name()
}

// CheckHealth checks the health of the email provider
func (s *Sender) CheckHealth(ctx context.Context) error {
	// For now, just check if provider is initialized
	// In the future, we could send a test email or ping the provider API
	if s.provider == nil {
		return fmt.Errorf("email provider not initialized")
	}
	return nil
}
