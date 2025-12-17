package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRequestIDTracing_AddsRequestIDToHeader(t *testing.T) {
	handler := RequestIDTracing()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// Set a request ID using chi middleware
	ctx := context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "test-request-id-123", rec.Header().Get("X-Request-ID"))
}

func TestRequestIDTracing_GeneratesRequestIDIfMissing(t *testing.T) {
	handler := RequestIDTracing()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No request ID set

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should generate a request ID
	requestID := rec.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID)
}

func TestRequestIDTracing_AddsToContext(t *testing.T) {
	var capturedRequestID string

	handler := RequestIDTracing()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), middleware.RequestIDKey, "test-id-456")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "test-id-456", capturedRequestID)
}

func TestGetRequestIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request_id", "test-123")

	requestID := GetRequestIDFromContext(ctx)
	assert.Equal(t, "test-123", requestID)
}

func TestGetRequestIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	requestID := GetRequestIDFromContext(ctx)
	assert.Empty(t, requestID)
}
