package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------- fakes ----------

// A tiny fake health handler for testing routing.
type fakeHealth struct{}

func (fakeHealth) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// fakeAuth implements AuthHandler. Each method writes a unique marker so tests
// can verify correct route dispatch.
type fakeAuth struct{}

func (fakeAuth) write(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(msg))
}

// Core auth
func (a fakeAuth) Register(w http.ResponseWriter, r *http.Request) { a.write(w, 200, "register") }
func (a fakeAuth) Login(w http.ResponseWriter, r *http.Request)    { a.write(w, 200, "login") }
func (a fakeAuth) Refresh(w http.ResponseWriter, r *http.Request)  { a.write(w, 200, "refresh") }
func (a fakeAuth) Logout(w http.ResponseWriter, r *http.Request)   { a.write(w, 200, "logout") }
func (a fakeAuth) Me(w http.ResponseWriter, r *http.Request)       { a.write(w, 200, "me") }
func (a fakeAuth) Admin(w http.ResponseWriter, r *http.Request)    { a.write(w, 200, "admin") }

// Moderation
func (a fakeAuth) BanUser(w http.ResponseWriter, r *http.Request)   { a.write(w, 200, "ban") }
func (a fakeAuth) UnbanUser(w http.ResponseWriter, r *http.Request) { a.write(w, 200, "unban") }
func (a fakeAuth) AdminSetUserRole(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "set_role")
}
func (a fakeAuth) AdminUserStatus(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "user_status")
}

// Email verification
func (a fakeAuth) VerifyEmailRequest(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "verify_email_request")
}
func (a fakeAuth) VerifyEmailConfirmGET(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "verify_email_confirm_get")
}
func (a fakeAuth) VerifyEmailConfirmPOST(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "verify_email_confirm_post")
}

// Password reset
func (a fakeAuth) PasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "pw_reset_request")
}
func (a fakeAuth) PasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "pw_reset_confirm")
}
func (a fakeAuth) PasswordResetValidate(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "pw_reset_validate")
}

// Account / session management
func (a fakeAuth) PasswordChange(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "pw_change")
}
func (a fakeAuth) AdminRevokeSessions(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "admin_revoke_sessions")
}
func (a fakeAuth) SessionsRevoke(w http.ResponseWriter, r *http.Request) {
	a.write(w, 200, "sessions_revoke")
}

// Optional
func (a fakeAuth) MeStatus(w http.ResponseWriter, r *http.Request) { a.write(w, 200, "me_status") }

// Middleware helper: no-op
func noopMW(next http.Handler) http.Handler { return next }

// Middleware helper: marker middleware that adds a header so we can assert it ran.
func headerMW(key, val string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(key, val)
			next.ServeHTTP(w, r)
		})
	}
}

// ---------- tests ----------

func TestNew_NilHealth_ReturnsError(t *testing.T) {
	_, err := New(Deps{
		Health: nil,
		Auth:   fakeAuth{},
		AuthMW: noopMW, AdminMW: noopMW, ModMW: noopMW,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestNew_NilAuth_ReturnsError(t *testing.T) {
	_, err := New(Deps{
		Health: fakeHealth{},
		Auth:   nil,
		AuthMW: noopMW, AdminMW: noopMW, ModMW: noopMW,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestNew_NilMiddlewares_ReturnError(t *testing.T) {
	_, err := New(Deps{
		Health: fakeHealth{},
		Auth:   fakeAuth{},
		AuthMW: nil, AdminMW: noopMW, ModMW: noopMW,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	_, err = New(Deps{
		Health: fakeHealth{},
		Auth:   fakeAuth{},
		AuthMW: noopMW, AdminMW: nil, ModMW: noopMW,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	_, err = New(Deps{
		Health: fakeHealth{},
		Auth:   fakeAuth{},
		AuthMW: noopMW, AdminMW: noopMW, ModMW: nil,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestNew_HealthzRoute_Works(t *testing.T) {
	h, err := New(Deps{
		Health: fakeHealth{},
		Auth:   fakeAuth{},
		AuthMW: noopMW, AdminMW: noopMW, ModMW: noopMW,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected CT %q, got %q", "text/plain; charset=utf-8", ct)
	}
}

func TestNew_LoginRoute_DispatchesToHandler(t *testing.T) {
	h, err := New(Deps{
		Health: fakeHealth{},
		Auth:   fakeAuth{},
		AuthMW: noopMW, AdminMW: noopMW, ModMW: noopMW,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/v1/login", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "login" {
		t.Fatalf("expected body %q, got %q", "login", rr.Body.String())
	}
}

func TestNew_MeRoute_UsesAuthMW(t *testing.T) {
	h, err := New(Deps{
		Health:  fakeHealth{},
		Auth:    fakeAuth{},
		AuthMW:  headerMW("X-AuthMW", "1"),
		AdminMW: noopMW,
		ModMW:   noopMW,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/v1/me", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-AuthMW") != "1" {
		t.Fatalf("expected AuthMW header set")
	}
	if rr.Body.String() != "me" {
		t.Fatalf("expected body %q, got %q", "me", rr.Body.String())
	}
}

func TestNew_AdminRoute_UsesAuthMWAndAdminMW(t *testing.T) {
	h, err := New(Deps{
		Health:  fakeHealth{},
		Auth:    fakeAuth{},
		AuthMW:  headerMW("X-AuthMW", "1"),
		AdminMW: headerMW("X-AdminMW", "1"),
		ModMW:   noopMW,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/v1/admin", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-AuthMW") != "1" {
		t.Fatalf("expected AuthMW header set")
	}
	if rr.Header().Get("X-AdminMW") != "1" {
		t.Fatalf("expected AdminMW header set")
	}
	if rr.Body.String() != "admin" {
		t.Fatalf("expected body %q, got %q", "admin", rr.Body.String())
	}
}

func TestNew_AdminSubroute_SetRole_UsesAuthMWAndAdminMW(t *testing.T) {
	h, err := New(Deps{
		Health:  fakeHealth{},
		Auth:    fakeAuth{},
		AuthMW:  headerMW("X-AuthMW", "1"),
		AdminMW: headerMW("X-AdminMW", "1"),
		ModMW:   noopMW,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/v1/admin/users/u-1/role", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-AuthMW") != "1" {
		t.Fatalf("expected AuthMW header set")
	}
	if rr.Header().Get("X-AdminMW") != "1" {
		t.Fatalf("expected AdminMW header set")
	}
	if rr.Body.String() != "set_role" {
		t.Fatalf("expected body %q, got %q", "set_role", rr.Body.String())
	}
}
