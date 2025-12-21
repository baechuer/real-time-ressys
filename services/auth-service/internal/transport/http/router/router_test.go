package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A tiny fake health handler for testing routing.
type fakeHealth struct{}

func (fakeHealth) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func TestNew_NilHealth_ReturnsError(t *testing.T) {
	_, err := New(Deps{
		Health: nil,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestNew_HealthzRoute_Works(t *testing.T) {
	h, err := New(Deps{
		Health: fakeHealth{},
		Auth:   nil, // auth not wired yet
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if h == nil {
		t.Fatalf("expected non-nil handler")
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected Content-Type %q, got %q", "text/plain; charset=utf-8", ct)
	}
}

func TestNew_AuthRoutes_NotRegisteredYet_404(t *testing.T) {
	h, err := New(Deps{
		Health: fakeHealth{},
		Auth:   nil,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/v1/login", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
