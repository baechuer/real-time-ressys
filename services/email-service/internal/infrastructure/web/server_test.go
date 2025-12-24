package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func newTestEmailWeb(t *testing.T, authBase string) http.Handler {
	t.Helper()

	s := NewServer(Config{
		Addr:     ":0",
		AuthBase: strings.TrimRight(authBase, "/"),
		// RedisPool nil -> RL disabled regardless
		RedisPool: nil,
		RateLimit: RateLimitConfig{Enabled: false},
	}, zerolog.Nop())

	// we are in package web, so we can access s.srv.Handler
	return s.srv.Handler
}

// ---------- helpers ----------

func readAll(t *testing.T, r *http.Response) string {
	t.Helper()
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// ---------- page tests ----------

func TestVerifyPage_MissingToken_400(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("GET", "http://email.local/verify", nil)
	w := do(h, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestVerifyPage_WithToken_200_HTMLContainsAPICall(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("GET", "http://email.local/verify?token=abc", nil)
	w := do(h, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "fetch('/api/verify'") {
		t.Fatalf("expected html to call /api/verify, got: %s", body)
	}
	// token should appear in embedded JSON payload (escaped)
	if !strings.Contains(body, "abc") {
		t.Fatalf("expected token to appear in page html, got: %s", body)
	}
}

func TestResetPage_MissingToken_400(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("GET", "http://email.local/reset", nil)
	w := do(h, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestResetPage_WithToken_200_HTMLContainsValidateAndConfirm(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("GET", "http://email.local/reset?token=abc", nil)
	w := do(h, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "/api/reset/validate") {
		t.Fatalf("expected html to call /api/reset/validate, got: %s", body)
	}
	if !strings.Contains(body, "/api/reset/confirm") {
		t.Fatalf("expected html to call /api/reset/confirm, got: %s", body)
	}
	if !strings.Contains(body, "abc") {
		t.Fatalf("expected token to appear in page html, got: %s", body)
	}
}

// ---------- API proxy tests ----------

func TestAPIVerify_BadJSON_400(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("POST", "http://email.local/api/verify", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestAPIVerify_MissingToken_400(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("POST", "http://email.local/api/verify", strings.NewReader(`{"token":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestAPIVerify_ProxiesPOST_AndPassesThroughStatusBody(t *testing.T) {
	// fake auth-service
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/auth/v1/verify-email/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var m map[string]string
		_ = json.NewDecoder(r.Body).Decode(&m)
		if strings.TrimSpace(m["token"]) != "tok123" {
			t.Fatalf("expected token tok123, got %+v", m)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte("ok-verify"))
	}))
	defer auth.Close()

	h := newTestEmailWeb(t, auth.URL)

	req := httptest.NewRequest("POST", "http://email.local/api/verify", strings.NewReader(`{"token":"tok123"}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != 201 {
		t.Fatalf("expected 201 got %d (body=%q)", w.Code, w.Body.String())
	}
	if w.Body.String() != "ok-verify" {
		t.Fatalf("expected body passthrough, got %q", w.Body.String())
	}
}

func TestAPIResetValidate_ProxiesGET_WithQuery_AndPassesThrough(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/auth/v1/password/reset/validate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("token") != "t0" {
			t.Fatalf("expected token=t0, got %q", r.URL.RawQuery)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok-validate"))
	}))
	defer auth.Close()

	h := newTestEmailWeb(t, auth.URL)

	req := httptest.NewRequest("POST", "http://email.local/api/reset/validate", strings.NewReader(`{"token":"t0"}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d (body=%q)", w.Code, w.Body.String())
	}
	if w.Body.String() != "ok-validate" {
		t.Fatalf("expected body passthrough, got %q", w.Body.String())
	}
}

func TestAPIResetConfirm_MissingNewPassword_400(t *testing.T) {
	h := newTestEmailWeb(t, "http://example.com")

	req := httptest.NewRequest("POST", "http://email.local/api/reset/confirm", strings.NewReader(`{"token":"t0","new_password":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestAPIResetConfirm_ProxiesPOST_AndPassesThrough(t *testing.T) {
	// auth confirms it receives token + new_password
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/auth/v1/password/reset/confirm" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)

		// minimal JSON check (avoid struct coupling)
		if !bytes.Contains(b, []byte(`"token"`)) || !bytes.Contains(b, []byte(`"new_password"`)) {
			t.Fatalf("expected token+new_password json, got %s", string(b))
		}
		w.WriteHeader(204)
	}))
	defer auth.Close()

	h := newTestEmailWeb(t, auth.URL)

	req := httptest.NewRequest("POST", "http://email.local/api/reset/confirm", strings.NewReader(`{"token":"t0","new_password":"longpassword123"}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != 204 {
		t.Fatalf("expected 204 got %d (body=%q)", w.Code, w.Body.String())
	}
}

// ---------- proxy error handling ----------

func TestAPIVerify_AuthDown_Returns502(t *testing.T) {
	// use an invalid URL that will fail fast
	badAuth := &url.URL{Scheme: "http", Host: "127.0.0.1:1"} // port 1 should refuse
	h := newTestEmailWeb(t, badAuth.String())

	req := httptest.NewRequest("POST", "http://email.local/api/verify", strings.NewReader(`{"token":"tok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := do(h, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 got %d", w.Code)
	}
}

// ---------- sanity: server client timeout is finite (not a hard requirement, but good guard) ----------

func TestServer_ProxyClientHasTimeout(t *testing.T) {
	s := NewServer(Config{
		Addr:      ":0",
		AuthBase:  "http://example.com",
		RedisPool: nil,
		RateLimit: RateLimitConfig{
			Enabled: false,
		},
	}, zerolog.Nop())

	// must be non-zero to avoid hanging tests / production calls
	if s.client == nil || s.client.Timeout <= 0*time.Second {
		t.Fatalf("expected proxy client timeout > 0, got %+v", s.client)
	}
}
