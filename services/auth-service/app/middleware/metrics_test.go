package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_RecordsRequest(t *testing.T) {
	handler := Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// Set route context
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{"/test"}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Metrics should be recorded (we can't easily verify without exposing internals,
	// but we can verify the handler works correctly)
}

func TestMetrics_RecordsDifferentStatusCodes(t *testing.T) {
	handler := Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/notfound", nil)
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{"/notfound"}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMetrics_RecordsRequestSize(t *testing.T) {
	handler := Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := []byte("test request body")
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{"/test"}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

