package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders_AllHeadersSet(t *testing.T) {
	t.Setenv("SECURITY_HEADERS_ENABLED", "true")
	os.Unsetenv("ENVIRONMENT") // Not production

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
	assert.Equal(t, "", rec.Header().Get("Strict-Transport-Security")) // Not in production
}

func TestSecurityHeaders_HSTSInProduction(t *testing.T) {
	t.Setenv("SECURITY_HEADERS_ENABLED", "true")
	t.Setenv("ENVIRONMENT", "production")

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	hsts := rec.Header().Get("Strict-Transport-Security")
	assert.Contains(t, hsts, "max-age=31536000")
	assert.Contains(t, hsts, "includeSubDomains")
	assert.Contains(t, hsts, "preload")
}

func TestSecurityHeaders_Disabled(t *testing.T) {
	t.Setenv("SECURITY_HEADERS_ENABLED", "false")

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("X-XSS-Protection"))
	assert.Empty(t, rec.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, rec.Header().Get("X-Frame-Options"))
}

func TestSecurityHeaders_DefaultEnabled(t *testing.T) {
	os.Unsetenv("SECURITY_HEADERS_ENABLED")

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Should be enabled by default
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

func TestSecurityHeaders_CSPContent(t *testing.T) {
	t.Setenv("SECURITY_HEADERS_ENABLED", "true")

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self'")
	assert.Contains(t, csp, "style-src 'self' 'unsafe-inline'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "base-uri 'self'")
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	t.Setenv("SECURITY_HEADERS_ENABLED", "true")

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	pp := rec.Header().Get("Permissions-Policy")
	assert.Contains(t, pp, "geolocation=()")
	assert.Contains(t, pp, "microphone=()")
	assert.Contains(t, pp, "camera=()")
}

func TestSecurityHeaders_XPoweredByNotSet(t *testing.T) {
	t.Setenv("SECURITY_HEADERS_ENABLED", "true")

	handler := SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// X-Powered-By should not be set by our middleware
	// (Note: handlers can still set it, but we don't set it by default)
	assert.Empty(t, rec.Header().Get("X-Powered-By"))
}

