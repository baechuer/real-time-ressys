package http_handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHealthHandler_ReturnsNonNil(t *testing.T) {
	h := NewHealthHandler()
	if h == nil {
		t.Fatalf("expected non-nil handler")
	}
}

func TestHealthz_ReturnsOKPlainText(t *testing.T) {
	h := NewHealthHandler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.Healthz(rr, req)

	res := rr.Result()
	defer res.Body.Close()

	// 1) status code
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	// 2) content-type header
	gotCT := res.Header.Get("Content-Type")
	wantCT := "text/plain; charset=utf-8"
	if gotCT != wantCT {
		t.Fatalf("expected Content-Type %q, got %q", wantCT, gotCT)
	}

	// 3) response body
	gotBody := rr.Body.String()
	wantBody := "ok"
	if gotBody != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, gotBody)
	}
}
