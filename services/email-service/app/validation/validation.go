package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxEmailLength is the maximum length for an email address (RFC 5321)
	MaxEmailLength = 254
	// MaxURLLength is a reasonable maximum for URLs
	MaxURLLength = 2048
	// MaxMessageBodySize is the maximum size for message body in bytes
	MaxMessageBodySize = 1024 * 1024 // 1MB
)

var (
	// emailRegex is a simplified email validation regex
	// For production, consider using a more robust library like github.com/go-playground/validator
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// ValidateEmail validates an email address format
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	if len(email) > MaxEmailLength {
		return fmt.Errorf("email exceeds maximum length of %d characters", MaxEmailLength)
	}

	// Check for basic format
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	// Additional checks
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid email format")
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Validate local part
	if len(localPart) == 0 || len(localPart) > 64 {
		return fmt.Errorf("invalid email local part")
	}

	// Validate domain part
	if len(domainPart) == 0 || len(domainPart) > 253 {
		return fmt.Errorf("invalid email domain part")
	}

	// Check for consecutive dots
	if strings.Contains(email, "..") {
		return fmt.Errorf("invalid email format: consecutive dots not allowed")
	}

	return nil
}

// ValidateURL validates a URL for safety and format
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL is required")
	}

	if len(urlStr) > MaxURLLength {
		return fmt.Errorf("URL exceeds maximum length of %d characters", MaxURLLength)
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got: %s", parsedURL.Scheme)
	}

	// Require a host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Check for dangerous patterns (basic check)
	lowerURL := strings.ToLower(urlStr)
	dangerousPatterns := []string{
		"javascript:",
		"data:",
		"vbscript:",
		"file:",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerURL, pattern) {
			return fmt.Errorf("URL contains potentially dangerous pattern: %s", pattern)
		}
	}

	return nil
}

// ValidateMessageBodySize validates the size of a message body
func ValidateMessageBodySize(body []byte) error {
	if len(body) > MaxMessageBodySize {
		return fmt.Errorf("message body exceeds maximum size of %d bytes", MaxMessageBodySize)
	}
	return nil
}

// SanitizeEmail sanitizes an email address for logging (masks PII)
func SanitizeEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		// If we can't parse, return masked version
		if len(email) > 3 {
			return email[:2] + "***@" + "***"
		}
		return "***"
	}

	localPart := parts[0]
	domainPart := parts[1]

	// Mask local part (keep first 2 chars, mask rest)
	var maskedLocal string
	if len(localPart) > 2 {
		maskedLocal = localPart[:2] + "***"
	} else {
		maskedLocal = "***"
	}

	// Mask domain (keep first part of domain, mask rest)
	domainParts := strings.Split(domainPart, ".")
	var maskedDomain string
	if len(domainParts) > 0 {
		firstPart := domainParts[0]
		if len(firstPart) > 2 {
			maskedDomain = firstPart[:2] + "***." + strings.Join(domainParts[1:], ".")
		} else {
			maskedDomain = "***." + strings.Join(domainParts[1:], ".")
		}
	} else {
		maskedDomain = "***"
	}

	return maskedLocal + "@" + maskedDomain
}

// SanitizeURL sanitizes a URL for logging (removes sensitive query params)
func SanitizeURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, return masked version
		if len(urlStr) > 50 {
			return urlStr[:50] + "***"
		}
		return "***"
	}

	// Keep scheme, host, path
	// Mask query parameters
	sanitized := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)

	if parsedURL.RawQuery != "" {
		// Count query params but don't show values
		query := parsedURL.Query()
		if len(query) > 0 {
			sanitized += "?***"
		}
	}

	return sanitized
}

// ValidateStringLength validates string length with UTF-8 awareness
func ValidateStringLength(s string, maxLen int) error {
	if utf8.RuneCountInString(s) > maxLen {
		return fmt.Errorf("string exceeds maximum length of %d characters", maxLen)
	}
	return nil
}

