package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadEmailConfig_DefaultSMTP(t *testing.T) {
	// Clear email-related env vars
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("EMAIL_FROM")
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
	}()

	os.Unsetenv("EMAIL_PROVIDER")

	cfg := LoadEmailConfig()

	assert.Equal(t, "smtp", cfg.Provider)
	assert.NotEmpty(t, cfg.FromEmail)
	assert.NotEmpty(t, cfg.FromName)
}

func TestLoadEmailConfig_SMTP(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("EMAIL_FROM")
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USERNAME")
		os.Unsetenv("SMTP_PASSWORD")
	}()

	os.Setenv("EMAIL_PROVIDER", "smtp")
	os.Setenv("EMAIL_FROM", "test@example.com")
	os.Setenv("SMTP_HOST", "smtp.test.com")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_USERNAME", "user")
	os.Setenv("SMTP_PASSWORD", "pass")

	cfg := LoadEmailConfig()

	assert.Equal(t, "smtp", cfg.Provider)
	assert.Equal(t, "test@example.com", cfg.FromEmail)
	assert.Equal(t, "smtp.test.com", cfg.SMTPHost)
	assert.Equal(t, 587, cfg.SMTPPort)
	assert.Equal(t, "user", cfg.SMTPUsername)
	assert.Equal(t, "pass", cfg.SMTPPassword)
}

func TestLoadEmailConfig_SMTP_DefaultPort(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("SMTP_PORT")
	}()

	os.Setenv("EMAIL_PROVIDER", "smtp")
	os.Unsetenv("SMTP_PORT")

	cfg := LoadEmailConfig()

	assert.Equal(t, "smtp", cfg.Provider)
	assert.Equal(t, 587, cfg.SMTPPort) // Default port
}

func TestLoadEmailConfig_SMTP_InvalidPort(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("SMTP_PORT")
	}()

	os.Setenv("EMAIL_PROVIDER", "smtp")
	os.Setenv("SMTP_PORT", "invalid")

	cfg := LoadEmailConfig()

	assert.Equal(t, "smtp", cfg.Provider)
	assert.Equal(t, 587, cfg.SMTPPort) // Should default to 587 on invalid port
}

func TestLoadEmailConfig_SendGrid(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("SENDGRID_API_KEY")
		os.Unsetenv("SENDGRID_FROM_EMAIL")
	}()

	os.Setenv("EMAIL_PROVIDER", "sendgrid")
	os.Setenv("SENDGRID_API_KEY", "SG.test-key")
	os.Setenv("SENDGRID_FROM_EMAIL", "sendgrid@example.com")

	cfg := LoadEmailConfig()

	assert.Equal(t, "sendgrid", cfg.Provider)
	assert.Equal(t, "SG.test-key", cfg.SendGridAPIKey)
	assert.Equal(t, "sendgrid@example.com", cfg.FromEmail)
}

func TestLoadEmailConfig_SES(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("SES_FROM_EMAIL")
	}()

	os.Setenv("EMAIL_PROVIDER", "ses")
	os.Setenv("AWS_REGION", "us-west-2")
	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("SES_FROM_EMAIL", "ses@example.com")

	cfg := LoadEmailConfig()

	assert.Equal(t, "ses", cfg.Provider)
	assert.Equal(t, "us-west-2", cfg.AWSRegion)
	assert.Equal(t, "test-key", cfg.AWSAccessKeyID)
	assert.Equal(t, "test-secret", cfg.AWSSecretKey)
	assert.Equal(t, "ses@example.com", cfg.FromEmail)
}

func TestLoadEmailConfig_SES_DefaultRegion(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("AWS_REGION")
	}()

	os.Setenv("EMAIL_PROVIDER", "ses")
	os.Unsetenv("AWS_REGION")

	cfg := LoadEmailConfig()

	assert.Equal(t, "ses", cfg.Provider)
	assert.Equal(t, "us-east-1", cfg.AWSRegion) // Default region
}

func TestLoadEmailConfig_FromEmailFallback(t *testing.T) {
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("EMAIL_FROM")
		os.Unsetenv("SENDGRID_FROM_EMAIL")
		os.Unsetenv("SES_FROM_EMAIL")
		os.Unsetenv("SMTP_FROM_EMAIL")
	}()

	os.Setenv("EMAIL_PROVIDER", "smtp")
	os.Unsetenv("EMAIL_FROM")
	os.Unsetenv("SMTP_FROM_EMAIL")

	cfg := LoadEmailConfig()

	// Should have a default email
	assert.NotEmpty(t, cfg.FromEmail)
}

