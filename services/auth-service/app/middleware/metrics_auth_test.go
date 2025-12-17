package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsAuth_IPWhitelist_Allowed(t *testing.T) {
	t.Setenv("METRICS_ALLOWED_IPS", "127.0.0.1,192.168.1.1")

	handler := MetricsAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsAuth_IPWhitelist_Blocked(t *testing.T) {
	t.Setenv("METRICS_ALLOWED_IPS", "127.0.0.1")

	handler := MetricsAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMetricsAuth_APIKey_Valid(t *testing.T) {
	t.Setenv("METRICS_API_KEY", "secret-key-123")
	os.Unsetenv("METRICS_ALLOWED_IPS")

	handler := MetricsAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("X-Metrics-API-Key", "secret-key-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsAuth_APIKey_Invalid(t *testing.T) {
	t.Setenv("METRICS_API_KEY", "secret-key-123")
	os.Unsetenv("METRICS_ALLOWED_IPS")

	handler := MetricsAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("X-Metrics-API-Key", "wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMetricsAuth_APIKey_QueryParam(t *testing.T) {
	t.Setenv("METRICS_API_KEY", "secret-key-123")
	os.Unsetenv("METRICS_ALLOWED_IPS")

	handler := MetricsAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics?api_key=secret-key-123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsAuth_NoConfig_AllowsAccess(t *testing.T) {
	os.Unsetenv("METRICS_ALLOWED_IPS")
	os.Unsetenv("METRICS_API_KEY")
	os.Unsetenv("METRICS_REQUIRE_AUTH")

	handler := MetricsAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should allow access if no protection configured (development mode)
	assert.Equal(t, http.StatusOK, rec.Code)
}
