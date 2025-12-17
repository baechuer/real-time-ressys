package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/docs"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/errors"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/logger"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/metrics"
	authmw "github.com/baechuer/real-time-ressys/services/auth-service/app/middleware"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/services"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

type application struct {
	config      config
	store       store.Storage
	authService *services.AuthService
	redisClient *redis.Client
	db          interface {
		PingContext(ctx context.Context) error
		Close() error
	}
	rabbitConn interface {
		IsClosed() bool
		Close() error
	}
	rabbitCh interface {
		IsClosed() bool
		Close() error
	}
}

type dbConfig struct {
	addr         string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

type config struct {
	addr string
	db   dbConfig
}

func (app *application) mount() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(authmw.RequestIDTracing()) // Propagate request ID to logger and context
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Metrics middleware - record HTTP metrics
	r.Use(authmw.Metrics())

	// Security headers - must be early to protect all responses
	r.Use(authmw.SecurityHeaders())

	// CORS middleware - must be early in the chain to handle preflight requests
	r.Use(authmw.CORS())

	// Request body size limit - prevent DoS attacks
	r.Use(authmw.BodyLimitFromEnv())

	//Set a timeout value on the request context (ctx), that will signal
	//through ctx.Done() that the request has time out and further
	//processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	loginLimit := authmw.RouteLimit{Name: "login", Capacity: 5, Window: time.Minute}
	registerLimit := authmw.RouteLimit{Name: "register", Capacity: 10, Window: 5 * time.Minute}
	refreshLimit := authmw.RouteLimit{Name: "refresh", Capacity: 30, Window: 5 * time.Minute}
	logoutLimit := authmw.RouteLimit{Name: "logout", Capacity: 30, Window: 5 * time.Minute}
	verifyEmailLimit := authmw.RouteLimit{Name: "verifyEmail", Capacity: 5, Window: time.Minute}
	protectedLimit := authmw.RouteLimit{Name: "protected", Capacity: 120, Window: time.Minute}
	healthCheckLimit := authmw.RouteLimit{Name: "healthCheck", Capacity: 20, Window: time.Minute}
	r.Route("/auth/v1", func(r chi.Router) {
		r.With(authmw.RateLimit(app.redisClient, healthCheckLimit, authmw.PrincipalIP())).Get("/health", http.HandlerFunc(app.healthCheckHandler))

		// Prometheus metrics endpoint - PROTECTED (IP whitelist, API key, or admin auth)
		r.With(authmw.MetricsAuth()).Get("/metrics", metrics.MetricsHandler().ServeHTTP)

		// OpenAPI documentation endpoint
		r.Get("/openapi.json", app.openAPIHandler)

		// Authentication endpoints
		r.With(authmw.RateLimit(app.redisClient, registerLimit, authmw.PrincipalIP())).Post("/register", http.HandlerFunc(app.registerHandler))
		r.With(authmw.RateLimit(app.redisClient, loginLimit, authmw.PrincipalIP())).Post("/login", http.HandlerFunc(app.loginHandler))
		r.With(authmw.RateLimit(app.redisClient, logoutLimit, authmw.PrincipalIP())).Post("/logout", http.HandlerFunc(app.logoutHandler))
		r.With(authmw.RateLimit(app.redisClient, refreshLimit, authmw.PrincipalIP())).Post("/refresh", http.HandlerFunc(app.refreshHandler))
		r.With(authmw.RateLimit(app.redisClient, verifyEmailLimit, authmw.PrincipalIP())).Post("/verify-email", http.HandlerFunc(app.verifyEmailHandler))
		passwordResetLimit := authmw.RouteLimit{Name: "passwordReset", Capacity: 5, Window: time.Minute}
		r.With(authmw.RateLimit(app.redisClient, passwordResetLimit, authmw.PrincipalIP())).Post("/reset-password", http.HandlerFunc(app.resetPasswordHandler))
		forgotPasswordLimit := authmw.RouteLimit{Name: "forgotPassword", Capacity: 3, Window: time.Hour}
		r.With(authmw.RateLimit(app.redisClient, forgotPasswordLimit, authmw.PrincipalIP())).Post("/forgot-password", http.HandlerFunc(app.forgotPasswordHandler))
		// Protected endpoints
		r.Group(func(pr chi.Router) {
			pr.Use(authmw.JWTAuth(app.redisClient))
			pr.Use(authmw.RateLimit(app.redisClient, protectedLimit, authmw.PrincipalUserOrIP()))
			pr.Get("/me", http.HandlerFunc(app.meHandler))
			pr.Get("/admin", http.HandlerFunc(app.adminOnlyHandler))
			pr.Post("/request-password-reset", http.HandlerFunc(app.requestPasswordResetHandler))
		})

	})
	return r
}

func (app *application) run(mux http.Handler) error {
	srv := http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}
	logger.Logger.Info().Str("addr", app.config.addr).Msg("Starting server")
	return srv.ListenAndServe()
}

// runWithGracefulShutdown starts the server with graceful shutdown support.
// It handles SIGTERM and SIGINT signals, allowing in-flight requests to complete
// before shutting down connections.
func (app *application) runWithGracefulShutdown(
	mux http.Handler,
	db interface{ Close() error },
	redisClient interface{ Close() error },
	rabbitConn interface{ Close() error },
	rabbitCh interface{ Close() error },
) error {
	srv := &http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Logger.Info().Str("addr", app.config.addr).Msg("Starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Wait for interrupt signal or server error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		logger.Logger.Info().Str("signal", sig.String()).Msg("Received signal, starting graceful shutdown")
	}

	// Graceful shutdown with timeout
	shutdownTimeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown server (stops accepting new connections, waits for in-flight requests)
	if err := srv.Shutdown(ctx); err != nil {
		logger.Logger.Error().Err(err).Msg("Server forced to shutdown")
		return err
	}

	logger.Logger.Info().Msg("Server gracefully stopped")

	// Close connections in order
	logger.Logger.Info().Msg("Closing database connection")
	if err := db.Close(); err != nil {
		logger.Logger.Error().Err(err).Msg("Error closing database")
	}

	logger.Logger.Info().Msg("Closing Redis connection")
	if err := redisClient.Close(); err != nil {
		logger.Logger.Error().Err(err).Msg("Error closing Redis")
	}

	logger.Logger.Info().Msg("Closing RabbitMQ channel")
	if err := rabbitCh.Close(); err != nil {
		logger.Logger.Error().Err(err).Msg("Error closing RabbitMQ channel")
	}

	logger.Logger.Info().Msg("Closing RabbitMQ connection")
	if err := rabbitConn.Close(); err != nil {
		logger.Logger.Error().Err(err).Msg("Error closing RabbitMQ connection")
	}

	logger.Logger.Info().Msg("Graceful shutdown completed")
	return nil
}

// registerHandler handles user registration
func (app *application) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest

	// 1. Parse JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	// 2. Sanitize inputs (before validation)
	req.Email = sanitizeEmail(req.Email, 255)
	req.Username = sanitizeUsername(req.Username, 50)
	// Password should NOT be sanitized (preserve special characters for password strength)
	// Only trim and limit length
	req.Password = sanitizeInput(req.Password, 128, true)

	// 3. Validate DTO
	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	// 4. Call service (already validated and sanitized)
	resp, appErr := app.authService.Register(r.Context(), req)
	if appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// loginHandler handles user login
func (app *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest

	// 1. Parse request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	// 2. Sanitize inputs (before validation)
	req.Email = sanitizeEmail(req.Email, 255)
	// Password should NOT be sanitized (preserve special characters)
	// Only trim and limit length
	req.Password = sanitizeInput(req.Password, 128, true)

	// 3. Validate DTO
	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	// 4. Call service layer (already validated and sanitized)
	resp, appErr := app.authService.Login(r.Context(), req)
	if appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	// Set refresh token cookie for browser clients (HttpOnly to protect from XSS).
	// Secure should be true in production (when served over HTTPS).
	// SameSite=StrictMode helps protect against CSRF attacks.
	secureCookie := os.Getenv("ENVIRONMENT") == "production" || os.Getenv("COOKIE_SECURE") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/auth/v1",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	// Expose access token in Authorization header for convenience.
	w.Header().Set("Authorization", "Bearer "+resp.AccessToken)

	// Do not return refresh token in body for browser clients.
	resp.RefreshToken = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// refreshHandler issues a new access token and rotates the refresh token (cookie-based).
func (app *application) refreshHandler(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		http.Error(w, "missing refresh token", http.StatusUnauthorized)
		return
	}

	resp, appErr := app.authService.Refresh(r.Context(), c.Value)
	if appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	// Set new refresh token cookie
	secureCookie := os.Getenv("ENVIRONMENT") == "production" || os.Getenv("COOKIE_SECURE") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/auth/v1",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
	// Expose new access token in header
	w.Header().Set("Authorization", "Bearer "+resp.AccessToken)
	resp.RefreshToken = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// verifyEmailHandler handles email verification using an opaque token.
func (app *application) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyEmailRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	if appErr := app.authService.VerifyEmail(r.Context(), req); appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// meHandler demonstrates a protected route and context injection.
func (app *application) meHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}
	roleID, _ := authmw.RoleIDFromContext(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id": userID,
		"role_id": roleID,
	})
}

// logoutHandler handles user logout
func (app *application) logoutHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "missing or invalid authorization header", http.StatusUnauthorized)
		return
	}
	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	var refreshToken string
	if c, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = c.Value
	}

	if err := app.authService.Logout(r.Context(), accessToken, refreshToken); err != nil {
		writeErrorResponse(w, err)
		return
	}

	// Clear refresh token cookie
	secureCookie := os.Getenv("ENVIRONMENT") == "production" || os.Getenv("COOKIE_SECURE") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/v1",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// forgotPasswordHandler handles unauthenticated password reset requests (for users who forgot their password).
func (app *application) forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.ForgotPasswordRequest

	// 1. Parse JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	// 2. Sanitize email
	req.Email = sanitizeEmail(req.Email, 255)

	// 3. Validate DTO
	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	// 4. Call service (always returns success for security - doesn't reveal if email exists)
	if appErr := app.authService.ForgotPassword(r.Context(), req); appErr != nil {
		// Even if there's an error, return success to prevent email enumeration
		// Errors are logged but not exposed to user
	}

	// Always return success (202 Accepted) - don't reveal if email exists
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If an account with that email exists and is verified, a password reset link has been sent.",
	})
}

// requestPasswordResetHandler handles password reset request (protected - requires auth token).
func (app *application) requestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	if appErr := app.authService.RequestPasswordReset(r.Context(), userID); appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// resetPasswordHandler handles password reset using token from email.
func (app *application) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	// Sanitize password (preserve special characters)
	req.NewPassword = sanitizeInput(req.NewPassword, 128, true)

	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	if appErr := app.authService.ResetPassword(r.Context(), req); appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// adminOnlyHandler shows optional role-based enforcement for protected routes.
func (app *application) adminOnlyHandler(w http.ResponseWriter, r *http.Request) {
	_, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}
	roleID, _ := authmw.RoleIDFromContext(r.Context())
	if !roleAllowed(roleID, []int{3}) { // only admin (role_id = 3)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "admin access granted",
	})
}

func roleAllowed(roleID int, allowed []int) bool {
	for _, v := range allowed {
		if roleID == v {
			return true
		}
	}
	return false
}

// openAPIHandler returns the OpenAPI specification
func (app *application) openAPIHandler(w http.ResponseWriter, r *http.Request) {
	docs.OpenAPIHandler(w, r)
}

// writeErrorResponse writes an error response in a consistent format
func writeErrorResponse(w http.ResponseWriter, appErr *errors.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Status)

	errResp := dto.ErrorResponse{
		Error: appErr.Message,
		Code:  string(appErr.Code),
	}

	json.NewEncoder(w).Encode(errResp)
}
