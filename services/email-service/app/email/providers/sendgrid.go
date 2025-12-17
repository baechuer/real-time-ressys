package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
)

// SendGrid API structures
type sendGridPersonalization struct {
	To []sendGridEmail `json:"to"`
}

type sendGridEmail struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendGridMessage struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridEmail             `json:"from"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SendGridProvider implements email sending via SendGrid API
type SendGridProvider struct {
	apiKey   string
	from     string
	fromName string
	client   *http.Client
}

// NewSendGridProvider creates a new SendGrid provider
func NewSendGridProvider(cfg *config.EmailConfig) (*SendGridProvider, error) {
	if cfg.SendGridAPIKey == "" {
		return nil, fmt.Errorf("SENDGRID_API_KEY is required")
	}

	return &SendGridProvider{
		apiKey:   cfg.SendGridAPIKey,
		from:     cfg.FromEmail,
		fromName: cfg.FromName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// SendEmail sends an email via SendGrid
func (p *SendGridProvider) SendEmail(ctx context.Context, email *models.Email) error {
	msg := sendGridMessage{
		Personalizations: []sendGridPersonalization{
			{
				To: []sendGridEmail{
					{Email: email.To},
				},
			},
		},
		From: sendGridEmail{
			Email: p.from,
			Name:  p.fromName,
		},
		Subject: email.Subject,
		Content: []sendGridContent{
			{
				Type:  "text/html",
				Value: email.Body,
			},
		},
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal SendGrid message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create SendGrid request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to SendGrid: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SendGrid API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Name returns the provider name
func (p *SendGridProvider) Name() string {
	return "sendgrid"
}
