package email

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderVerificationTemplate(t *testing.T) {
	email := "test@example.com"
	url := "https://example.com/verify?token=abc123"

	html, err := RenderVerificationTemplate(email, url)
	require.NoError(t, err)

	// Check that HTML contains expected elements
	// Note: email parameter is used in template data but may not appear in rendered HTML
	assert.Contains(t, html, url)
	assert.Contains(t, html, "Verify Your Email Address")
	assert.Contains(t, html, "Verify Email")
	assert.Contains(t, html, "24 hours")
}

func TestRenderVerificationTemplate_ContainsURL(t *testing.T) {
	email := "user@test.com"
	url := "https://example.com/verify?token=xyz789"

	html, err := RenderVerificationTemplate(email, url)
	require.NoError(t, err)

	// URL should appear in both button link and text link
	assert.Contains(t, html, `href="`+url+`"`)
	assert.Contains(t, html, url)
}

func TestRenderVerificationTemplate_HTMLStructure(t *testing.T) {
	email := "test@example.com"
	url := "https://example.com/verify"

	html, err := RenderVerificationTemplate(email, url)
	require.NoError(t, err)

	// Should be valid HTML structure
	assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
	assert.Contains(t, html, "<html>")
	assert.Contains(t, html, "</html>")
	assert.Contains(t, html, "<body")
	assert.Contains(t, html, "</body>")
}

func TestRenderPasswordResetTemplate(t *testing.T) {
	email := "test@example.com"
	url := "https://example.com/reset?token=def456"

	html, err := RenderPasswordResetTemplate(email, url)
	require.NoError(t, err)

	// Check that HTML contains expected elements
	// Note: email parameter is used in template data but may not appear in rendered HTML
	assert.Contains(t, html, url)
	assert.Contains(t, html, "Reset Your Password")
	assert.Contains(t, html, "Reset Password")
	assert.Contains(t, html, "1 hour")
}

func TestRenderPasswordResetTemplate_ContainsURL(t *testing.T) {
	email := "user@test.com"
	url := "https://example.com/reset?token=abc123"

	html, err := RenderPasswordResetTemplate(email, url)
	require.NoError(t, err)

	// URL should appear in both button link and text link
	assert.Contains(t, html, `href="`+url+`"`)
	assert.Contains(t, html, url)
}

func TestRenderPasswordResetTemplate_HTMLStructure(t *testing.T) {
	email := "test@example.com"
	url := "https://example.com/reset"

	html, err := RenderPasswordResetTemplate(email, url)
	require.NoError(t, err)

	// Should be valid HTML structure
	assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
	assert.Contains(t, html, "<html>")
	assert.Contains(t, html, "</html>")
	assert.Contains(t, html, "<body")
	assert.Contains(t, html, "</body>")
}

func TestRenderVerificationTemplate_SpecialCharacters(t *testing.T) {
	email := "test+tag@example.com"
	url := "https://example.com/verify?token=abc&def=123"

	html, err := RenderVerificationTemplate(email, url)
	require.NoError(t, err)

	// Should handle special characters in URL (may be HTML escaped)
	// URL with & will be escaped to &amp; in HTML
	assert.True(t, strings.Contains(html, url) || strings.Contains(html, "abc&amp;def=123"))
}

func TestRenderPasswordResetTemplate_SpecialCharacters(t *testing.T) {
	email := "user+test@example.com"
	url := "https://example.com/reset?token=xyz&param=value"

	html, err := RenderPasswordResetTemplate(email, url)
	require.NoError(t, err)

	// Should handle special characters in URL (may be HTML escaped)
	// URL with & will be escaped to &amp; in HTML
	assert.True(t, strings.Contains(html, url) || strings.Contains(html, "xyz&amp;param=value"))
}

func TestRenderVerificationTemplate_EmptyInput(t *testing.T) {
	html, err := RenderVerificationTemplate("", "")
	require.NoError(t, err)

	// Should still generate valid HTML
	assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
}

func TestRenderPasswordResetTemplate_EmptyInput(t *testing.T) {
	html, err := RenderPasswordResetTemplate("", "")
	require.NoError(t, err)

	// Should still generate valid HTML
	assert.True(t, strings.HasPrefix(html, "<!DOCTYPE html>"))
}
