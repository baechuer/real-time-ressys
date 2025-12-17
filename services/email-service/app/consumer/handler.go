package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/baechuer/real-time-ressys/services/email-service/app/email"
	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
	appErrors "github.com/baechuer/real-time-ressys/services/email-service/app/errors"
	"github.com/baechuer/real-time-ressys/services/email-service/app/logger"
	"github.com/baechuer/real-time-ressys/services/email-service/app/metrics"
	"github.com/baechuer/real-time-ressys/services/email-service/app/ratelimit"
	"github.com/baechuer/real-time-ressys/services/email-service/app/validation"
)

// Handler handles incoming RabbitMQ messages
type Handler struct {
	emailSender *email.Sender
	rateLimiter *ratelimit.RateLimiter
}

// NewHandler creates a new message handler
func NewHandler(emailSender *email.Sender, rateLimiter *ratelimit.RateLimiter) *Handler {
	return &Handler{
		emailSender: emailSender,
		rateLimiter: rateLimiter,
	}
}

// HandleEmailVerification handles email verification messages
func (h *Handler) HandleEmailVerification(ctx context.Context, msg *models.EmailVerificationMessage) error {
	startTime := time.Now()
	log := logger.GetLoggerFromContext(ctx)

	// Input validation
	if err := validation.ValidateEmail(msg.Email); err != nil {
		log.Error().Err(err).Msg("invalid email format")
		return appErrors.NewInvalidInput("invalid email format: " + err.Error())
	}

	if err := validation.ValidateURL(msg.VerificationURL); err != nil {
		log.Error().Err(err).Msg("invalid verification URL")
		return appErrors.NewInvalidInput("invalid verification URL: " + err.Error())
	}

	// Rate limiting - max 5 emails per email address per hour
	if h.rateLimiter != nil {
		if err := h.rateLimiter.Check(ctx, msg.Email, 5, time.Hour); err != nil {
			log.Warn().Err(err).Str("email", validation.SanitizeEmail(msg.Email)).Msg("rate limit exceeded")
			return appErrors.NewInvalidInput("rate limit exceeded: " + err.Error())
		}
	}

	// Log with sanitized PII
	sanitizedEmail := validation.SanitizeEmail(msg.Email)
	sanitizedURL := validation.SanitizeURL(msg.VerificationURL)
	log.Info().
		Str("email", sanitizedEmail).
		Str("url", sanitizedURL).
		Str("type", msg.Type).
		Msg("processing email verification message")

	// Get provider name for metrics
	providerName := "unknown"
	if h.emailSender != nil {
		providerName = h.emailSender.ProviderName()
	}

	// Send email
	err := h.emailSender.SendVerificationEmail(ctx, msg.Email, msg.VerificationURL)
	duration := time.Since(startTime)

	if err != nil {
		// Determine error type
		errorType := "unknown"
		if appErr, ok := err.(*appErrors.AppError); ok {
			errorType = string(appErr.Code)
		}
		
		log.Error().
			Err(err).
			Str("email", sanitizedEmail).
			Msg("failed to send verification email")
		
		metrics.RecordEmailFailed("email_verification", providerName, errorType)
		return err
	}

	log.Info().
		Str("email", sanitizedEmail).
		Msg("verification email sent successfully")

	metrics.RecordEmailSent("email_verification", providerName, duration)
	return nil
}

// HandlePasswordReset handles password reset messages
func (h *Handler) HandlePasswordReset(ctx context.Context, msg *models.PasswordResetMessage) error {
	startTime := time.Now()
	log := logger.GetLoggerFromContext(ctx)

	// Input validation
	if err := validation.ValidateEmail(msg.Email); err != nil {
		log.Error().Err(err).Msg("invalid email format")
		return appErrors.NewInvalidInput("invalid email format: " + err.Error())
	}

	if err := validation.ValidateURL(msg.ResetURL); err != nil {
		log.Error().Err(err).Msg("invalid reset URL")
		return appErrors.NewInvalidInput("invalid reset URL: " + err.Error())
	}

	// Rate limiting - max 3 password resets per email address per hour
	if h.rateLimiter != nil {
		if err := h.rateLimiter.Check(ctx, msg.Email, 3, time.Hour); err != nil {
			log.Warn().Err(err).Str("email", validation.SanitizeEmail(msg.Email)).Msg("rate limit exceeded")
			return appErrors.NewInvalidInput("rate limit exceeded: " + err.Error())
		}
	}

	// Log with sanitized PII
	sanitizedEmail := validation.SanitizeEmail(msg.Email)
	sanitizedURL := validation.SanitizeURL(msg.ResetURL)
	log.Info().
		Str("email", sanitizedEmail).
		Str("url", sanitizedURL).
		Str("type", msg.Type).
		Msg("processing password reset message")

	// Get provider name for metrics
	providerName := "unknown"
	if h.emailSender != nil {
		providerName = h.emailSender.ProviderName()
	}

	// Send email
	err := h.emailSender.SendPasswordResetEmail(ctx, msg.Email, msg.ResetURL)
	duration := time.Since(startTime)

	if err != nil {
		// Determine error type
		errorType := "unknown"
		if appErr, ok := err.(*appErrors.AppError); ok {
			errorType = string(appErr.Code)
		}
		
		log.Error().
			Err(err).
			Str("email", sanitizedEmail).
			Msg("failed to send password reset email")
		
		metrics.RecordEmailFailed("password_reset", providerName, errorType)
		return err
	}

	log.Info().
		Str("email", sanitizedEmail).
		Msg("password reset email sent successfully")

	metrics.RecordEmailSent("password_reset", providerName, duration)
	return nil
}

// ProcessMessage processes a raw message and routes to appropriate handler
func (h *Handler) ProcessMessage(ctx context.Context, body []byte) error {
	// Validate message body size
	if err := validation.ValidateMessageBodySize(body); err != nil {
		return appErrors.NewInvalidInput(err.Error())
	}

	// Try to parse as email verification
	var verificationMsg models.EmailVerificationMessage
	if err := json.Unmarshal(body, &verificationMsg); err == nil && verificationMsg.Type == "email_verification" {
		// Additional validation
		if verificationMsg.Email == "" {
			return appErrors.NewInvalidInput("email is required")
		}
		if verificationMsg.VerificationURL == "" {
			return appErrors.NewInvalidInput("verification_url is required")
		}
		return h.HandleEmailVerification(ctx, &verificationMsg)
	}

	// Try to parse as password reset
	var resetMsg models.PasswordResetMessage
	if err := json.Unmarshal(body, &resetMsg); err == nil && resetMsg.Type == "password_reset" {
		// Additional validation
		if resetMsg.Email == "" {
			return appErrors.NewInvalidInput("email is required")
		}
		if resetMsg.ResetURL == "" {
			return appErrors.NewInvalidInput("reset_url is required")
		}
		return h.HandlePasswordReset(ctx, &resetMsg)
	}

	return appErrors.NewInvalidInput("unknown message type")
}

