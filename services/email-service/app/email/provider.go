package email

import (
	"context"

	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
)

// Provider defines the interface for email sending providers
type Provider interface {
	SendEmail(ctx context.Context, email *models.Email) error
	Name() string
}

