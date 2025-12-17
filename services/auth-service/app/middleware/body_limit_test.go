package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBodyLimit_WithinLimit(t *testing.T) {
	handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBuffer(make([]byte, 512)) // 512 bytes, under limit
	req := httptest.NewRequest("POST", "/test", body)
	req.ContentLength = 512
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimit_ExceedsLimit(t *testing.T) {
	handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBuffer(make([]byte, 2048)) // 2048 bytes, exceeds 1024 limit
	req := httptest.NewRequest("POST", "/test", body)
	req.ContentLength = 2048
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Contains(t, rec.Body.String(), "PAYLOAD_TOO_LARGE")
	assert.Contains(t, rec.Body.String(), "Request body too large")
}

func TestBodyLimit_ExactLimit(t *testing.T) {
	limit := int64(1024)
	handler := BodyLimit(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBuffer(make([]byte, int(limit))) // Exactly at limit
	req := httptest.NewRequest("POST", "/test", body)
	req.ContentLength = limit
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimit_ZeroLimitUsesDefault(t *testing.T) {
	handler := BodyLimit(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Default should be 1MB, so 512KB should pass
	body := bytes.NewBuffer(make([]byte, 512*1024))
	req := httptest.NewRequest("POST", "/test", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimit_NegativeLimitUsesDefault(t *testing.T) {
	handler := BodyLimit(-1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBuffer(make([]byte, 512*1024))
	req := httptest.NewRequest("POST", "/test", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimitFromEnv_Default(t *testing.T) {
	os.Unsetenv("REQUEST_BODY_MAX_SIZE")

	handler := BodyLimitFromEnv()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Default is 1MB, so 512KB should pass
	body := bytes.NewBuffer(make([]byte, 512*1024))
	req := httptest.NewRequest("POST", "/test", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimitFromEnv_CustomLimit(t *testing.T) {
	customLimit := int64(2048)
	t.Setenv("REQUEST_BODY_MAX_SIZE", strconv.FormatInt(customLimit, 10))

	handler := BodyLimitFromEnv()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should accept 1KB (under 2KB limit)
	body := bytes.NewBuffer(make([]byte, 1024))
	req := httptest.NewRequest("POST", "/test", body)
	req.ContentLength = 1024
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimitFromEnv_CustomLimitExceeded(t *testing.T) {
	customLimit := int64(2048)
	t.Setenv("REQUEST_BODY_MAX_SIZE", strconv.FormatInt(customLimit, 10))

	handler := BodyLimitFromEnv()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should reject 3KB (over 2KB limit)
	body := bytes.NewBuffer(make([]byte, 3072))
	req := httptest.NewRequest("POST", "/test", body)
	req.ContentLength = 3072
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestBodyLimitFromEnv_InvalidValueUsesDefault(t *testing.T) {
	t.Setenv("REQUEST_BODY_MAX_SIZE", "invalid")

	handler := BodyLimitFromEnv()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should use default 1MB
	body := bytes.NewBuffer(make([]byte, 512*1024))
	req := httptest.NewRequest("POST", "/test", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimitFromEnv_ZeroValueUsesDefault(t *testing.T) {
	t.Setenv("REQUEST_BODY_MAX_SIZE", "0")

	handler := BodyLimitFromEnv()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should use default 1MB
	body := bytes.NewBuffer(make([]byte, 512*1024))
	req := httptest.NewRequest("POST", "/test", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimit_NoContentLength(t *testing.T) {
	handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewBuffer(make([]byte, 512))
	req := httptest.NewRequest("POST", "/test", body)
	// No Content-Length header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should still work, MaxBytesReader will handle it
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBodyLimit_GETRequest(t *testing.T) {
	handler := BodyLimit(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// GET requests typically don't have bodies, should pass
	assert.Equal(t, http.StatusOK, rec.Code)
}
