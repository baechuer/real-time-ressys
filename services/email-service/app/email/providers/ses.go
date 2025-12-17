package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
)

// SESProvider implements email.Provider interface

// SESProvider implements email sending via AWS SES
type SESProvider struct {
	region        string
	accessKeyID   string
	secretKey     string
	from          string
	fromName      string
	client        *http.Client
	serviceName   string
	signingMethod string
}

// NewSESProvider creates a new AWS SES provider
func NewSESProvider(cfg *config.EmailConfig) (*SESProvider, error) {
	if cfg.AWSRegion == "" {
		return nil, fmt.Errorf("AWS_REGION is required")
	}
	if cfg.AWSAccessKeyID == "" || cfg.AWSSecretKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	return &SESProvider{
		region:      cfg.AWSRegion,
		accessKeyID: cfg.AWSAccessKeyID,
		secretKey:   cfg.AWSSecretKey,
		from:        cfg.FromEmail,
		fromName:    cfg.FromName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		serviceName:   "ses",
		signingMethod: "AWS4-HMAC-SHA256",
	}, nil
}

// SendEmail sends an email via AWS SES
func (p *SESProvider) SendEmail(ctx context.Context, email *models.Email) error {
	// Build SES API request
	endpoint := fmt.Sprintf("https://email.%s.amazonaws.com", p.region)

	// Create form data
	formData := url.Values{}
	formData.Set("Action", "SendEmail")
	formData.Set("Source", fmt.Sprintf("%s <%s>", p.fromName, p.from))
	formData.Set("Destination.ToAddresses.member.1", email.To)
	formData.Set("Message.Subject.Data", email.Subject)
	formData.Set("Message.Subject.Charset", "UTF-8")
	formData.Set("Message.Body.Html.Data", email.Body)
	formData.Set("Message.Body.Html.Charset", "UTF-8")

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create SES request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Sign the request using AWS Signature Version 4
	if err := p.signRequest(req, formData.Encode()); err != nil {
		return fmt.Errorf("failed to sign SES request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to SES: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SES API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// signRequest signs the request using AWS Signature Version 4
func (p *SESProvider) signRequest(req *http.Request, payload string) error {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	// Create canonical request
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("host:%s\n", req.URL.Host)
	canonicalHeaders += fmt.Sprintf("x-amz-date:%s\n", amzDate)
	signedHeaders := "host;x-amz-date"

	hasher := sha256.New()
	hasher.Write([]byte(payload))
	payloadHash := fmt.Sprintf("%x", hasher.Sum(nil))

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash)

	// Create string to sign
	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, p.region, p.serviceName)

	hasher = sha256.New()
	hasher.Write([]byte(canonicalRequest))
	canonicalRequestHash := fmt.Sprintf("%x", hasher.Sum(nil))

	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		canonicalRequestHash)

	// Calculate signature
	kDate := p.hmacSHA256([]byte("AWS4"+p.secretKey), dateStamp)
	kRegion := p.hmacSHA256(kDate, p.region)
	kService := p.hmacSHA256(kRegion, p.serviceName)
	kSigning := p.hmacSHA256(kService, "aws4_request")
	signature := fmt.Sprintf("%x", p.hmacSHA256(kSigning, stringToSign))

	// Add authorization header
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		p.accessKeyID,
		credentialScope,
		signedHeaders,
		signature)

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Authorization", authorization)

	return nil
}

func (p *SESProvider) hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// Name returns the provider name
func (p *SESProvider) Name() string {
	return "ses"
}
