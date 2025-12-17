package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://app.example.com")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Authorization", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_NoOriginHeader(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No Origin header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightRequest(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Equal(t, "3600", rec.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_PreflightRequest_DisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should still return 204, but without CORS headers
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardSubdomain(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*.example.com")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		name   string
		origin string
		allowed bool
	}{
		{"app subdomain", "https://app.example.com", true},
		{"www subdomain", "https://www.example.com", true},
		{"api subdomain", "https://api.example.com", true},
		{"different domain", "https://example.org", false},
		{"root domain doesn't match", "https://example.com", false}, // Wildcard doesn't match root domain
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tc.allowed {
				assert.Equal(t, tc.origin, rec.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORS_AllowAllOrigins_Development(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	os.Unsetenv("CORS_ALLOWED_ORIGINS") // Not set = defaults to *

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://any-origin.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ExplicitWildcard(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://any-origin.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_Disabled(t *testing.T) {
	t.Setenv("CORS_ENABLED", "false")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "test response", rec.Body.String())
}

func TestCORS_MultipleOrigins(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,https://app.example.com,https://staging.example.com")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		origin   string
		allowed  bool
	}{
		{"http://localhost:3000", true},
		{"https://app.example.com", true},
		{"https://staging.example.com", true},
		{"https://evil.com", false},
	}

	for _, tc := range testCases {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tc.allowed {
				assert.Equal(t, tc.origin, rec.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORS_WhitespaceInOrigins(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:3000 , https://app.example.com ")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ExposedHeaders(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Authorization", "Bearer token123")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Authorization", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCORS_PreflightWithCredentials(t *testing.T) {
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestIsCORSEnabled_DefaultTrue(t *testing.T) {
	os.Unsetenv("CORS_ENABLED")
	// Note: This tests the function directly, but it's not exported
	// We test it indirectly through CORS() behavior
	t.Setenv("CORS_ENABLED", "")
	
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should be enabled by default (allows all origins when CORS_ALLOWED_ORIGINS not set)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestIsCORSEnabled_ExplicitFalse(t *testing.T) {
	t.Setenv("CORS_ENABLED", "false")

	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestIsCORSEnabled_CaseInsensitive(t *testing.T) {
	testCases := []struct {
		value   string
		enabled bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"", true}, // Default
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			if tc.value != "" {
				t.Setenv("CORS_ENABLED", tc.value)
			} else {
				os.Unsetenv("CORS_ENABLED")
			}

			handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tc.enabled {
				// When enabled, should have CORS headers (if origin allowed or wildcard)
				// We set CORS_ALLOWED_ORIGINS to allow this origin
				t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
				handler2 := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				rec2 := httptest.NewRecorder()
				handler2.ServeHTTP(rec2, req)
				assert.NotEmpty(t, rec2.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

