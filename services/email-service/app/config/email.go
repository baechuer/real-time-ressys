package config

import (
	"os"
	"strconv"
)

// EmailConfig holds email provider configuration
type EmailConfig struct {
	Provider string

	// Common
	FromEmail string
	FromName  string

	// SendGrid
	SendGridAPIKey string

	// AWS SES
	AWSRegion        string
	AWSAccessKeyID   string
	AWSSecretKey     string

	// SMTP
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
}

// LoadEmailConfig loads email configuration from environment
func LoadEmailConfig() *EmailConfig {
	provider := GetString("EMAIL_PROVIDER", "smtp")

	cfg := &EmailConfig{
		Provider:  provider,
		FromEmail: GetString("EMAIL_FROM", GetString("SENDGRID_FROM_EMAIL", GetString("SES_FROM_EMAIL", GetString("SMTP_FROM_EMAIL", "noreply@example.com")))),
		FromName:  GetString("EMAIL_FROM_NAME", GetString("SENDGRID_FROM_NAME", "My App")),
	}

	switch provider {
	case "sendgrid":
		cfg.SendGridAPIKey = os.Getenv("SENDGRID_API_KEY")
	case "ses":
		cfg.AWSRegion = GetString("AWS_REGION", "us-east-1")
		cfg.AWSAccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		cfg.AWSSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	case "smtp":
		cfg.SMTPHost = GetString("SMTP_HOST", "smtp.gmail.com")
		portStr := os.Getenv("SMTP_PORT")
		if portStr == "" {
			cfg.SMTPPort = 587
		} else {
			if port, err := strconv.Atoi(portStr); err == nil {
				cfg.SMTPPort = port
			} else {
				cfg.SMTPPort = 587
			}
		}
		cfg.SMTPUsername = os.Getenv("SMTP_USERNAME")
		cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	}

	return cfg
}

