package router

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type HealthHandler interface {
	Healthz(w http.ResponseWriter, r *http.Request)
}

type AuthHandler interface {
	// Core auth
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
	Refresh(w http.ResponseWriter, r *http.Request)
	Logout(w http.ResponseWriter, r *http.Request)
	Me(w http.ResponseWriter, r *http.Request)
	Admin(w http.ResponseWriter, r *http.Request)
	// Moderation (account-level)
	BanUser(w http.ResponseWriter, r *http.Request)
	UnbanUser(w http.ResponseWriter, r *http.Request)
	AdminSetUserRole(w http.ResponseWriter, r *http.Request)
	AdminUserStatus(w http.ResponseWriter, r *http.Request)

	// Email verification
	VerifyEmailRequest(w http.ResponseWriter, r *http.Request)
	VerifyEmailConfirmGET(w http.ResponseWriter, r *http.Request)
	VerifyEmailConfirmPOST(w http.ResponseWriter, r *http.Request)

	// Password reset
	PasswordResetRequest(w http.ResponseWriter, r *http.Request)
	PasswordResetConfirm(w http.ResponseWriter, r *http.Request)
	PasswordResetValidate(w http.ResponseWriter, r *http.Request)

	// Account / session management
	PasswordChange(w http.ResponseWriter, r *http.Request)
	AdminRevokeSessions(w http.ResponseWriter, r *http.Request)
	SessionsRevoke(w http.ResponseWriter, r *http.Request)

	// Optional
	MeStatus(w http.ResponseWriter, r *http.Request)
}

type Deps struct {
	Health HealthHandler
	Auth   AuthHandler

	AuthMW  func(http.Handler) http.Handler
	ModMW   func(http.Handler) http.Handler
	AdminMW func(http.Handler) http.Handler

	// ---- Rate limit middlewares (optional; can be nil) ----
	RLRegister func(http.Handler) http.Handler
	RLLogin    func(http.Handler) http.Handler
	RLRefresh  func(http.Handler) http.Handler
	RLLogout   func(http.Handler) http.Handler

	RLVerifyEmailRequest   func(http.Handler) http.Handler
	RLPasswordResetRequest func(http.Handler) http.Handler

	RLPasswordChange func(http.Handler) http.Handler
	RLSessionsRevoke func(http.Handler) http.Handler
	RLModActions     func(http.Handler) http.Handler
	RLAdminActions   func(http.Handler) http.Handler
}

func New(deps Deps) (http.Handler, error) {
	if deps.Health == nil {
		return nil, fmt.Errorf("nil Health handler")
	}
	if deps.Auth == nil {
		return nil, fmt.Errorf("nil Auth handler")
	}
	if deps.AuthMW == nil {
		return nil, fmt.Errorf("nil Auth middleware")
	}
	if deps.AdminMW == nil {
		return nil, fmt.Errorf("nil Admin middleware")
	}
	if deps.ModMW == nil {
		return nil, fmt.Errorf("nil Mod middleware")
	}

	r := chi.NewRouter()
	r.Get("/healthz", deps.Health.Healthz)

	r.Route("/auth/v1", func(r chi.Router) {
		// --- Core auth ---
		if deps.RLRegister != nil {
			r.With(deps.RLRegister).Post("/register", deps.Auth.Register)
		} else {
			r.Post("/register", deps.Auth.Register)
		}

		if deps.RLLogin != nil {
			r.With(deps.RLLogin).Post("/login", deps.Auth.Login)
		} else {
			r.Post("/login", deps.Auth.Login)
		}

		if deps.RLRefresh != nil {
			r.With(deps.RLRefresh).Post("/refresh", deps.Auth.Refresh)
		} else {
			r.Post("/refresh", deps.Auth.Refresh)
		}

		if deps.RLLogout != nil {
			r.With(deps.RLLogout).Post("/logout", deps.Auth.Logout)
		} else {
			r.Post("/logout", deps.Auth.Logout)
		}

		r.With(deps.AuthMW).Get("/me", deps.Auth.Me)
		r.With(deps.AuthMW).Get("/me/status", deps.Auth.MeStatus)

		// You already mount admin below; keeping this is fine
		r.With(deps.AuthMW, deps.AdminMW).Get("/admin", deps.Auth.Admin)

		// --- Moderation (account-level) ---
		r.Route("/mod", func(r chi.Router) {
			r.Use(deps.AuthMW)
			r.Use(deps.ModMW)
			if deps.RLModActions != nil {
				r.Use(deps.RLModActions)
			}

			r.Post("/users/{id}/ban", deps.Auth.BanUser)
			r.Post("/users/{id}/unban", deps.Auth.UnbanUser)
		})

		// --- Admin (privileged) ---
		r.Route("/admin", func(r chi.Router) {
			r.Use(deps.AuthMW)
			r.Use(deps.AdminMW)
			if deps.RLAdminActions != nil {
				r.Use(deps.RLAdminActions)
			}

			r.Get("/", deps.Auth.Admin)
			r.Post("/users/{id}/role", deps.Auth.AdminSetUserRole)
			r.Get("/users/{id}/status", deps.Auth.AdminUserStatus)
			r.Post("/users/{id}/sessions/revoke", deps.Auth.AdminRevokeSessions)
		})

		// --- Email verification ---
		if deps.RLVerifyEmailRequest != nil {
			r.With(deps.RLVerifyEmailRequest).Post("/verify-email/request", deps.Auth.VerifyEmailRequest)
		} else {
			r.Post("/verify-email/request", deps.Auth.VerifyEmailRequest)
		}
		//r.Get("/verify-email/confirm", deps.Auth.VerifyEmailConfirmGET) // ?token=...
		r.Post("/verify-email/confirm", deps.Auth.VerifyEmailConfirmPOST)

		// --- Password reset ---
		if deps.RLPasswordResetRequest != nil {
			r.With(deps.RLPasswordResetRequest).Post("/password/reset/request", deps.Auth.PasswordResetRequest)
		} else {
			r.Post("/password/reset/request", deps.Auth.PasswordResetRequest)
		}
		r.Post("/password/reset/confirm", deps.Auth.PasswordResetConfirm)
		r.Get("/password/reset/validate", deps.Auth.PasswordResetValidate) // ?token=...

		// --- Account / session management ---
		if deps.RLPasswordChange != nil {
			r.With(deps.AuthMW, deps.RLPasswordChange).Post("/password/change", deps.Auth.PasswordChange)
		} else {
			r.With(deps.AuthMW).Post("/password/change", deps.Auth.PasswordChange)
		}

		if deps.RLSessionsRevoke != nil {
			r.With(deps.AuthMW, deps.RLSessionsRevoke).Post("/sessions/revoke", deps.Auth.SessionsRevoke)
		} else {
			r.With(deps.AuthMW).Post("/sessions/revoke", deps.Auth.SessionsRevoke)
		}
	})

	return r, nil
}
