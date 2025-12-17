package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"runtime"

	"github.com/alicebob/miniredis/v2"
	cfgPkg "github.com/baechuer/real-time-ressys/services/auth-service/app/config"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	authmw "github.com/baechuer/real-time-ressys/services/auth-service/app/middleware"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/services"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

/*
E2E Test Cases:
1) Register → Login → Protected (/me) flow succeeds with real Postgres + Redis
2) Protected endpoint rejects missing token
*/

type e2eApplication struct {
	authService *services.AuthService
	redisClient *redis.Client
}

func (app *e2eApplication) mount() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/auth/v1", func(r chi.Router) {
		r.Post("/register", http.HandlerFunc(app.registerHandler))
		r.Post("/login", http.HandlerFunc(app.loginHandler))
		r.Post("/logout", http.HandlerFunc(app.logoutHandler))
		r.Group(func(pr chi.Router) {
			pr.Use(authmw.JWTAuth(app.redisClient))
			pr.Get("/me", http.HandlerFunc(app.meHandler))
		})
	})
	return r
}

func (app *e2eApplication) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeErrorResponse(w, "invalid request body", "INVALID_INPUT", http.StatusBadRequest)
		return
	}
	req.Email = sanitizeEmail(req.Email, 255)
	req.Username = sanitizeUsername(req.Username, 50)
	req.Password = sanitizeInput(req.Password, 128, true)
	if err := validateRequest(&req); err != nil {
		app.writeErrorResponse(w, err.Error(), "INVALID_INPUT", http.StatusBadRequest)
		return
	}
	resp, appErr := app.authService.Register(r.Context(), req)
	if appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (app *e2eApplication) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeErrorResponse(w, "invalid request body", "INVALID_INPUT", http.StatusBadRequest)
		return
	}
	req.Email = sanitizeEmail(req.Email, 255)
	req.Password = sanitizeInput(req.Password, 128, true)
	if err := validateRequest(&req); err != nil {
		app.writeErrorResponse(w, err.Error(), "INVALID_INPUT", http.StatusBadRequest)
		return
	}
	resp, appErr := app.authService.Login(r.Context(), req)
	if appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

	// Set refresh cookie and Authorization header; omit refresh token in body.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.RefreshToken,
		Path:     "/auth/v1",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
	w.Header().Set("Authorization", "Bearer "+resp.AccessToken)
	resp.RefreshToken = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (app *e2eApplication) logoutHandler(w http.ResponseWriter, r *http.Request) {
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
		app.writeErrorResponse(w, err.Message, string(err.Code), err.Status)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/v1",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (app *e2eApplication) meHandler(w http.ResponseWriter, r *http.Request) {
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

func (app *e2eApplication) writeErrorResponse(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// Minimal validation for tests (matches integration tests)
func validateRequest(req interface{}) error {
	switch v := req.(type) {
	case *dto.RegisterRequest:
		if v.Email == "" || !strings.Contains(v.Email, "@") || !strings.Contains(v.Email, ".") {
			return assert.AnError
		}
		if len(v.Username) < 3 {
			return assert.AnError
		}
		if len(v.Password) < 8 {
			return assert.AnError
		}
	case *dto.LoginRequest:
		if v.Email == "" || !strings.Contains(v.Email, "@") || !strings.Contains(v.Email, ".") {
			return assert.AnError
		}
		if len(v.Password) < 8 {
			return assert.AnError
		}
	default:
		return assert.AnError
	}
	return nil
}

// Sanitizers copied from integration tests (simplified)
func sanitizeInput(input string, maxLength int, preserveSpecialChars bool) string {
	input = strings.TrimSpace(input)
	if maxLength > 0 && len(input) > maxLength {
		input = input[:maxLength]
	}
	return input
}

func sanitizeEmail(email string, maxLength int) string {
	email = sanitizeInput(email, maxLength, false)
	return strings.ToLower(email)
}

func sanitizeUsername(username string, maxLength int) string {
	username = sanitizeInput(username, maxLength, false)
	return username
}

func setupE2E(t *testing.T) (*e2eApplication, func()) {
	t.Helper()
	ctx := context.Background()

	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("Skipping e2e test on Windows without Docker support")
	}
	if _, err := testcontainers.NewDockerClientWithOpts(ctx); err != nil {
		t.Skipf("Skipping e2e test because Docker is unavailable: %v", err)
	}

	// Postgres container
	pg, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:17"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)

	// Schema
	_, err = db.Exec(`
CREATE EXTENSION IF NOT EXISTS citext;
CREATE TABLE IF NOT EXISTS roles (
	id SERIAL PRIMARY KEY,
	name VARCHAR(50) NOT NULL UNIQUE,
	description TEXT,
	created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO roles (name, description) VALUES
	('user', 'Regular user'),
	('moderator', 'Content moderator'),
	('admin', 'Administrator')
ON CONFLICT (name) DO NOTHING;
CREATE TABLE IF NOT EXISTS users (
	id BIGSERIAL PRIMARY KEY,
	username VARCHAR(255) NOT NULL UNIQUE,
	email citext NOT NULL UNIQUE,
	password_hash VARCHAR(255) NOT NULL,
	is_email_verified BOOLEAN NOT NULL DEFAULT FALSE,
	role_id INTEGER NOT NULL DEFAULT 1 REFERENCES roles(id) ON DELETE RESTRICT,
	created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	require.NoError(t, err)

	// Redis
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Services
	store := store.NewStorage(db)
	authSvc := services.NewAuthService(store, redisClient, nil)

	app := &e2eApplication{
		authService: authSvc,
		redisClient: redisClient,
	}

	cleanup := func() {
		db.Close()
		redisClient.Close()
		_ = pg.Terminate(ctx)
	}

	return app, cleanup
}

func TestAuthE2E_RegisterLoginProtected(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")

	app, cleanup := setupE2E(t)
	defer cleanup()
	router := app.mount()

	// Register
	regBody := dto.RegisterRequest{
		Email:    "e2e@example.com",
		Username: "e2euser",
		Password: "Password123",
	}
	bodyBytes, _ := json.Marshal(regBody)
	req := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Login
	loginReq := dto.LoginRequest{
		Email:    "e2e@example.com",
		Password: "Password123",
	}
	loginBytes, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(loginBytes))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)
	require.Equal(t, http.StatusOK, recLogin.Code)

	var authResp dto.AuthResponse
	require.NoError(t, json.NewDecoder(recLogin.Body).Decode(&authResp))
	assert.NotEmpty(t, authResp.AccessToken)
	assert.Empty(t, authResp.RefreshToken)

	// Refresh cookie should be set
	var refreshCookie *http.Cookie
	for _, c := range recLogin.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)
	assert.NotEmpty(t, refreshCookie.Value)

	// Authorization header should carry access token
	authHeader := recLogin.Header().Get("Authorization")
	assert.True(t, strings.HasPrefix(authHeader, "Bearer "))

	// Refresh to get a new access token and cookie
	reqRefresh := httptest.NewRequest("POST", "/auth/v1/refresh", nil)
	reqRefresh.AddCookie(refreshCookie)
	recRefresh := httptest.NewRecorder()
	router.ServeHTTP(recRefresh, reqRefresh)
	require.Equal(t, http.StatusOK, recRefresh.Code)

	var refreshResp dto.AuthResponse
	require.NoError(t, json.NewDecoder(recRefresh.Body).Decode(&refreshResp))
	assert.NotEmpty(t, refreshResp.AccessToken)
	assert.Empty(t, refreshResp.RefreshToken)

	var newRefreshCookie *http.Cookie
	for _, c := range recRefresh.Result().Cookies() {
		if c.Name == "refresh_token" {
			newRefreshCookie = c
			break
		}
	}
	require.NotNil(t, newRefreshCookie)
	assert.NotEqual(t, refreshCookie.Value, newRefreshCookie.Value)

	// Authorization header on refresh response
	authHeaderRefresh := recRefresh.Header().Get("Authorization")
	assert.True(t, strings.HasPrefix(authHeaderRefresh, "Bearer "))

	// Logout
	reqLogout := httptest.NewRequest("POST", "/auth/v1/logout", nil)
	reqLogout.Header.Set("Authorization", authHeaderRefresh)
	reqLogout.AddCookie(newRefreshCookie)
	recLogout := httptest.NewRecorder()
	router.ServeHTTP(recLogout, reqLogout)
	assert.Equal(t, http.StatusNoContent, recLogout.Code)

	// Protected access should now be unauthorized with same token
	reqMe := httptest.NewRequest("GET", "/auth/v1/me", nil)
	reqMe.Header.Set("Authorization", authHeaderRefresh)
	recMe := httptest.NewRecorder()
	router.ServeHTTP(recMe, reqMe)
	assert.Equal(t, http.StatusUnauthorized, recMe.Code)
}

func TestAuthE2E_ProtectedMissingToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")

	app, cleanup := setupE2E(t)
	defer cleanup()
	router := app.mount()

	req := httptest.NewRequest("GET", "/auth/v1/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

