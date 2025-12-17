package providers

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
)

// SMTPProvider implements email.Provider interface

// SMTPProvider implements email sending via SMTP
type SMTPProvider struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
}

// NewSMTPProvider creates a new SMTP provider
func NewSMTPProvider(cfg *config.EmailConfig) (*SMTPProvider, error) {
	if cfg.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}
	if cfg.SMTPUsername == "" || cfg.SMTPPassword == "" {
		return nil, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD are required")
	}

	return &SMTPProvider{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
		from:     cfg.FromEmail,
		fromName: cfg.FromName,
	}, nil
}

// SendEmail sends an email via SMTP
func (p *SMTPProvider) SendEmail(ctx context.Context, email *models.Email) error {
	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	auth := smtp.PlainAuth("", p.username, p.password, p.host)

	fromHeader := fmt.Sprintf("%s <%s>", p.fromName, p.from)
	to := []string{email.To}

	msg := []byte(fmt.Sprintf("From: %s\r\n", fromHeader) +
		fmt.Sprintf("To: %s\r\n", email.To) +
		fmt.Sprintf("Subject: %s\r\n", email.Subject) +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" +
		email.Body + "\r\n")

	err := smtp.SendMail(addr, auth, p.from, to, msg)
	if err != nil {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	return nil
}

// Name returns the provider name
func (p *SMTPProvider) Name() string {
	return "smtp"
}

