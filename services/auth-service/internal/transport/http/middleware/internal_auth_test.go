package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalAuth(t *testing.T) {
	secret := "super-secret"
	mw := InternalAuth(secret)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	t.Run("Missing Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/internal/resource", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Wrong Secret", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/internal/resource", nil)
		req.Header.Set("X-Internal-Secret", "wrong-secret")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Correct Secret", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/internal/resource", nil)
		req.Header.Set("X-Internal-Secret", secret)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != "ok" {
			t.Errorf("expected body 'ok', got %q", w.Body.String())
		}
	})
}

func TestInternalAuth_Misconfigured(t *testing.T) {
	// If secret is empty, it should be a 500 error (fail closed)
	mw := InternalAuth("")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/internal/resource", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when secret is empty, got %d", w.Code)
	}
}
