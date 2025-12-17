package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	cfgPkg "github.com/baechuer/real-time-ressys/services/auth-service/app/config"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	authmw "github.com/baechuer/real-time-ressys/services/auth-service/app/middleware"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/models"
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
	"golang.org/x/crypto/bcrypt"
)

/*
Integration Test Cases:

Registration:
1) TestRegistrationFlow_Success
2) TestRegistrationFlow_DuplicateEmail
3) TestRegistrationFlow_InvalidInput
4) TestRegistrationFlow_DatabaseVerification
5) TestRegistrationFlow_MultipleUsers

Login:
6) TestLoginFlow_Success
7) TestLoginFlow_InvalidPassword
8) TestLoginFlow_UserNotFound
*/

// setupTestDatabase creates a PostgreSQL test container and returns connection string
func setupTestDatabase(t *testing.T) (string, func()) {
	ctx := context.Background()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if _, err := testcontainers.NewDockerClientWithOpts(ctx); err != nil {
		t.Skipf("Skipping integration test because Docker is unavailable: %v", err)
	}

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
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
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Cleanup function
	cleanup := func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return connStr, cleanup
}

// setupDatabaseSchema creates the users table in the test database
func setupDatabaseSchema(t *testing.T, db *sql.DB) {
	// Read and execute the migration SQL
	schemaSQL := `
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
	`

	_, err := db.Exec(schemaSQL)
	require.NoError(t, err, "Failed to create database schema")
}

// testApplication is a test version of the application struct
type testApplication struct {
	store       store.Storage
	authService *services.AuthService
	redisClient *redis.Client
}

// mount creates the HTTP router for testing
func (app *testApplication) mount() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/auth/v1", func(r chi.Router) {
		r.Get("/health", http.HandlerFunc(app.healthCheckHandler))
		r.Post("/register", http.HandlerFunc(app.registerHandler))
		r.Post("/login", http.HandlerFunc(app.loginHandler))
		r.Post("/logout", http.HandlerFunc(app.logoutHandler))
		r.Post("/refresh", http.HandlerFunc(app.refreshHandler))
		r.Post("/verify-email", http.HandlerFunc(app.verifyEmailHandler))
		r.Post("/reset-password", http.HandlerFunc(app.resetPasswordHandler))

		r.Group(func(pr chi.Router) {
			pr.Use(authmw.JWTAuth(app.redisClient))
			pr.Get("/me", http.HandlerFunc(app.meHandler))
			pr.Post("/request-password-reset", http.HandlerFunc(app.requestPasswordResetHandler))
		})
	})
	return r
}

// healthCheckHandler is a simple health check handler
func (app *testApplication) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// registerHandler handles user registration (copied from handlers/auth.go)
func (app *testApplication) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest

	// Parse JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeErrorResponse(w, "invalid request body", "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	// Sanitize inputs
	req.Email = sanitizeEmail(req.Email, 255)
	req.Username = sanitizeUsername(req.Username, 50)
	req.Password = sanitizeInput(req.Password, 128, true)

	// Validate DTO
	if err := app.validateRequest(&req); err != nil {
		app.writeErrorResponse(w, err.Error(), "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	// Call service
	resp, appErr := app.authService.Register(r.Context(), req)
	if appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// loginHandler handles user login (mirrors handlers/auth.go with cookie-based refresh token)
func (app *testApplication) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest

	// Parse request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeErrorResponse(w, "invalid request body", "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	// Sanitize inputs
	req.Email = sanitizeEmail(req.Email, 255)
	req.Password = sanitizeInput(req.Password, 128, true)

	// Validate DTO
	if err := app.validateRequest(&req); err != nil {
		app.writeErrorResponse(w, err.Error(), "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	// Call service
	resp, appErr := app.authService.Login(r.Context(), req)
	if appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

	// Set refresh token cookie (HttpOnly) and expose access token in header
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

	// Do not return refresh token in body for browser clients
	resp.RefreshToken = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// logoutHandler mirrors production logout: requires bearer token, blacklists it, clears refresh cookie.
func (app *testApplication) logoutHandler(w http.ResponseWriter, r *http.Request) {
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

func (app *testApplication) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyEmailRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeErrorResponse(w, "invalid request body", "INVALID_INPUT", http.StatusBadRequest)
		return
	}
	if err := app.validateRequest(&req); err != nil {
		app.writeErrorResponse(w, err.Error(), "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	if appErr := app.authService.VerifyEmail(r.Context(), req); appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (app *testApplication) requestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	if appErr := app.authService.RequestPasswordReset(r.Context(), userID); appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (app *testApplication) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.writeErrorResponse(w, "invalid request body", "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	req.NewPassword = sanitizeInput(req.NewPassword, 128, true)

	if err := app.validateRequest(&req); err != nil {
		app.writeErrorResponse(w, err.Error(), "INVALID_INPUT", http.StatusBadRequest)
		return
	}

	if appErr := app.authService.ResetPassword(r.Context(), req); appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (app *testApplication) refreshHandler(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		http.Error(w, "missing refresh token", http.StatusUnauthorized)
		return
	}

	resp, appErr := app.authService.Refresh(r.Context(), c.Value)
	if appErr != nil {
		app.writeErrorResponse(w, appErr.Message, string(appErr.Code), appErr.Status)
		return
	}

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

func (app *testApplication) meHandler(w http.ResponseWriter, r *http.Request) {
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

// writeErrorResponse writes an error response
func (app *testApplication) writeErrorResponse(w http.ResponseWriter, message, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// validateRequest validates a request DTO (simplified version)
func (app *testApplication) validateRequest(req interface{}) error {
	switch v := req.(type) {
	case *dto.RegisterRequest:
		if v.Email == "" {
			return fmt.Errorf("email is required")
		}
		if !strings.Contains(v.Email, "@") || !strings.Contains(v.Email, ".") {
			return fmt.Errorf("email must be a valid email address")
		}
		if v.Username == "" {
			return fmt.Errorf("username is required")
		}
		if len(v.Username) < 3 {
			return fmt.Errorf("username must be at least 3 characters")
		}
		if err := validatePassword(v.Password); err != nil {
			return err
		}
	case *dto.LoginRequest:
		if v.Email == "" {
			return fmt.Errorf("email is required")
		}
		if !strings.Contains(v.Email, "@") || !strings.Contains(v.Email, ".") {
			return fmt.Errorf("email must be a valid email address")
		}
		if err := validatePassword(v.Password); err != nil {
			return err
		}
	case *dto.VerifyEmailRequest:
		if v.Token == "" {
			return fmt.Errorf("token is required")
		}
	case *dto.ResetPasswordRequest:
		if v.Token == "" {
			return fmt.Errorf("token is required")
		}
		if v.NewPassword == "" {
			return fmt.Errorf("new_password is required")
		}
		if err := validatePassword(v.NewPassword); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid request type")
	}
	return nil
}

func validatePassword(pw string) error {
	if pw == "" {
		return fmt.Errorf("password is required")
	}
	if len(pw) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hasUpper, hasLower, hasNumber := false, false, false
	for _, c := range pw {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasNumber = true
		}
	}
	if !hasUpper || !hasLower || !hasNumber {
		return fmt.Errorf("password must contain at least one uppercase letter, one lowercase letter, and one number")
	}
	return nil
}

// sanitizeEmail sanitizes email input
func sanitizeEmail(email string, maxLength int) string {
	email = sanitizeInput(email, maxLength, false)
	return email
}

// sanitizeUsername sanitizes username input
func sanitizeUsername(username string, maxLength int) string {
	username = sanitizeInput(username, maxLength, false)
	return username
}

// sanitizeInput sanitizes user input
func sanitizeInput(input string, maxLength int, preserveSpecialChars bool) string {
	// Simple sanitization for integration tests
	if len(input) > maxLength && maxLength > 0 {
		return input[:maxLength]
	}
	return input
}

// setupTestApplication creates a full application stack for testing
func setupTestApplication(t *testing.T, db *sql.DB) *testApplication {
	t.Setenv("JWT_SECRET", "supersecret")

	storage := store.NewStorage(db)
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	authService := services.NewAuthService(storage, redisClient, nil)

	return &testApplication{
		store:       storage,
		authService: authService,
		redisClient: redisClient,
	}
}

// TestRegistrationFlow_Success tests successful user registration end-to-end
func TestRegistrationFlow_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Connect to database
	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err, "Failed to connect to test database")
	defer db.Close()

	// Setup database schema
	setupDatabaseSchema(t, db)

	// Setup application
	app := setupTestApplication(t, db)

	// Create HTTP request
	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	// Execute request
	handler := app.mount()
	handler.ServeHTTP(recorder, req)

	// Assert HTTP response
	assert.Equal(t, http.StatusCreated, recorder.Code, "Should return 201 Created")
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response dto.RegisterResponse
	err = json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err, "Should decode response JSON")
	assert.Equal(t, "User registered successfully", response.Message)

	// Verify user was created in database
	var dbUser struct {
		ID              int64
		Username        string
		Email           string
		PasswordHash    string
		IsEmailVerified bool
		CreatedAt       time.Time
	}

	err = db.QueryRow(
		"SELECT id, username, email, password_hash, is_email_verified, created_at FROM users WHERE email = $1",
		"test@example.com",
	).Scan(
		&dbUser.ID,
		&dbUser.Username,
		&dbUser.Email,
		&dbUser.PasswordHash,
		&dbUser.IsEmailVerified,
		&dbUser.CreatedAt,
	)

	require.NoError(t, err, "User should exist in database")
	assert.Equal(t, "testuser", dbUser.Username)
	assert.Equal(t, "test@example.com", dbUser.Email)
	assert.False(t, dbUser.IsEmailVerified, "Email should not be verified")
	assert.NotZero(t, dbUser.ID, "ID should be set")
	assert.False(t, dbUser.CreatedAt.IsZero(), "CreatedAt should be set")

	// Verify password was hashed (not plain text)
	assert.NotEqual(t, "Password123", dbUser.PasswordHash, "Password should be hashed")
	assert.Contains(t, dbUser.PasswordHash, "$2a$", "Should be bcrypt hash")

	// Verify password hash can verify the original password
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("Password123"))
	assert.NoError(t, err, "Password hash should verify original password")

	// Verify password hash does NOT verify wrong password
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("WrongPassword"))
	assert.Error(t, err, "Password hash should NOT verify wrong password")
}

// TestVerifyEmail_Success covers verification using a pre-seeded token in Redis.
func TestVerifyEmail_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)

	// Create user directly
	user := &models.User{
		Email:           "verify@example.com",
		Username:        "verifyuser",
		PasswordHash:    "hash",
		IsEmailVerified: false,
	}
	require.NoError(t, app.store.Users.Create(context.Background(), user))

	// Seed verification token in Redis
	raw := "rawtoken-integration"
	sum := sha256.Sum256([]byte(raw))
	hashed := hex.EncodeToString(sum[:])
	payload, _ := json.Marshal(map[string]interface{}{
		"user_id":    user.ID,
		"email":      user.Email,
		"created_at": time.Now().Unix(),
	})
	require.NoError(t, app.redisClient.Set(context.Background(), "email_verification:"+hashed, payload, time.Hour).Err())

	// Call verify-email endpoint
	body := map[string]string{"token": raw}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/v1/verify-email", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := app.mount()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		var errResp dto.ErrorResponse
		_ = json.NewDecoder(rec.Body).Decode(&errResp)
		t.Fatalf("expected 200, got %d: %+v", rec.Code, errResp)
	}

	_, err = app.redisClient.Get(context.Background(), "email_verification:"+hashed).Result()
	assert.Equal(t, redis.Nil, err)

	fetched, err := app.store.Users.GetByEmail(context.Background(), user.Email)
	require.NoError(t, err)
	assert.True(t, fetched.IsEmailVerified)
}

// TestRegistrationFlow_DuplicateEmail tests duplicate email registration
func TestRegistrationFlow_DuplicateEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)

	// Register first user
	reqBody1 := dto.RegisterRequest{
		Email:    "existing@example.com",
		Username: "user1",
		Password: "Password123",
	}

	bodyBytes1, err := json.Marshal(reqBody1)
	require.NoError(t, err)

	req1 := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes1))
	req1.Header.Set("Content-Type", "application/json")
	recorder1 := httptest.NewRecorder()

	handler := app.mount()
	handler.ServeHTTP(recorder1, req1)

	// First registration should succeed
	assert.Equal(t, http.StatusCreated, recorder1.Code)

	// Attempt to register with same email
	reqBody2 := dto.RegisterRequest{
		Email:    "existing@example.com",
		Username: "user2",
		Password: "Password456",
	}

	bodyBytes2, err := json.Marshal(reqBody2)
	require.NoError(t, err)

	req2 := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	recorder2 := httptest.NewRecorder()

	handler.ServeHTTP(recorder2, req2)

	// Second registration should fail with 409 Conflict
	assert.Equal(t, http.StatusConflict, recorder2.Code)

	var errorResp dto.ErrorResponse
	err = json.NewDecoder(recorder2.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "CONFLICT", errorResp.Code)
	assert.Equal(t, "email already in use", errorResp.Error)

	// Verify only one user exists in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", "existing@example.com").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have only one user with this email")
}

// TestRegistrationFlow_InvalidInput tests invalid request body
func TestRegistrationFlow_InvalidInput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)

	// Test with invalid email
	reqBody := dto.RegisterRequest{
		Email:    "invalid-email",
		Username: "testuser",
		Password: "Password123",
	}

	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler := app.mount()
	handler.ServeHTTP(recorder, req)

	// Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResp dto.ErrorResponse
	err = json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", errorResp.Code)
	assert.Contains(t, errorResp.Error, "email")

	// Verify no user was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "No user should be created with invalid input")
}

// TestRegistrationFlow_DatabaseVerification tests database state after registration
func TestRegistrationFlow_DatabaseVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)

	// Register user
	reqBody := dto.RegisterRequest{
		Email:    "verify@example.com",
		Username: "verifyuser",
		Password: "SecurePass123",
	}

	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler := app.mount()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)

	// Verify database state
	var dbUser struct {
		ID              int64
		Username        string
		Email           string
		PasswordHash    string
		IsEmailVerified bool
		CreatedAt       time.Time
	}

	err = db.QueryRow(
		"SELECT id, username, email, password_hash, is_email_verified, created_at FROM users WHERE email = $1",
		"verify@example.com",
	).Scan(
		&dbUser.ID,
		&dbUser.Username,
		&dbUser.Email,
		&dbUser.PasswordHash,
		&dbUser.IsEmailVerified,
		&dbUser.CreatedAt,
	)

	require.NoError(t, err)

	// Verify all fields
	assert.Greater(t, dbUser.ID, int64(0), "ID should be positive")
	assert.Equal(t, "verifyuser", dbUser.Username)
	assert.Equal(t, "verify@example.com", dbUser.Email)
	assert.False(t, dbUser.IsEmailVerified)
	assert.WithinDuration(t, time.Now(), dbUser.CreatedAt, 5*time.Second, "CreatedAt should be recent")

	// Verify password hash
	assert.NotEmpty(t, dbUser.PasswordHash)
	assert.NotEqual(t, "SecurePass123", dbUser.PasswordHash)
	assert.True(t, len(dbUser.PasswordHash) > 50, "Bcrypt hash should be long")

	// Verify password hash works
	err = bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte("SecurePass123"))
	assert.NoError(t, err)
}

// TestRegistrationFlow_MultipleUsers tests registering multiple users
func TestRegistrationFlow_MultipleUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)

	handler := app.mount()

	// Register multiple users
	users := []dto.RegisterRequest{
		{Email: "user1@example.com", Username: "user1", Password: "Password123"},
		{Email: "user2@example.com", Username: "user2", Password: "Password456"},
		{Email: "user3@example.com", Username: "user3", Password: "Password789"},
	}

	for _, user := range users {
		bodyBytes, err := json.Marshal(user)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusCreated, recorder.Code, fmt.Sprintf("User %s should be registered", user.Email))
	}

	// Verify all users exist in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, len(users), count, "All users should be in database")

	// Verify each user can be retrieved
	for _, user := range users {
		var dbUser struct {
			ID       int64
			Username string
			Email    string
		}

		err = db.QueryRow(
			"SELECT id, username, email FROM users WHERE email = $1",
			user.Email,
		).Scan(&dbUser.ID, &dbUser.Username, &dbUser.Email)

		require.NoError(t, err, fmt.Sprintf("User %s should exist", user.Email))
		assert.Equal(t, user.Username, dbUser.Username)
		assert.Equal(t, user.Email, dbUser.Email)
	}
}

// TestLoginFlow_Success tests successful login after registration
func TestLoginFlow_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)

	handler := app.mount()

	// Register user
	registerReq := dto.RegisterRequest{
		Email:    "login@example.com",
		Username: "loginuser",
		Password: "Password123",
	}
	bodyReg, _ := json.Marshal(registerReq)
	reqReg := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyReg))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	handler.ServeHTTP(recReg, reqReg)
	require.Equal(t, http.StatusCreated, recReg.Code)

	// Login
	loginReq := dto.LoginRequest{
		Email:    "login@example.com",
		Password: "Password123",
	}
	bodyLogin, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLogin))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	handler.ServeHTTP(recLogin, reqLogin)

	require.Equal(t, http.StatusOK, recLogin.Code)
	var resp dto.AuthResponse
	require.NoError(t, json.NewDecoder(recLogin.Body).Decode(&resp))
	assert.NotEmpty(t, resp.AccessToken)
	assert.Empty(t, resp.RefreshToken) // refresh token is now set as HttpOnly cookie
	assert.Equal(t, "loginuser", resp.User.Username)
	assert.Equal(t, "login@example.com", resp.User.Email)

	// Verify Authorization header carries access token
	authHeader := recLogin.Header().Get("Authorization")
	assert.True(t, strings.HasPrefix(authHeader, "Bearer "))

	// Verify refresh token cookie set
	cookies := recLogin.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)
	assert.NotEmpty(t, refreshCookie.Value)

	// Logout using the access token and refresh cookie
	reqLogout := httptest.NewRequest("POST", "/auth/v1/logout", nil)
	reqLogout.Header.Set("Authorization", authHeader)
	reqLogout.AddCookie(refreshCookie)
	recLogout := httptest.NewRecorder()
	handler.ServeHTTP(recLogout, reqLogout)
	assert.Equal(t, http.StatusNoContent, recLogout.Code)

	// Access to protected with same token should now fail due to blacklist
	reqMe := httptest.NewRequest("GET", "/auth/v1/me", nil)
	reqMe.Header.Set("Authorization", authHeader)
	recMe := httptest.NewRecorder()
	handler.ServeHTTP(recMe, reqMe)
	assert.Equal(t, http.StatusUnauthorized, recMe.Code)

	// Refresh with the (now old) refresh cookie should fail because it was deleted on logout
	reqRefresh := httptest.NewRequest("POST", "/auth/v1/refresh", nil)
	reqRefresh.AddCookie(refreshCookie)
	recRefresh := httptest.NewRecorder()
	handler.ServeHTTP(recRefresh, reqRefresh)
	assert.Equal(t, http.StatusUnauthorized, recRefresh.Code)
}

// TestLoginFlow_InvalidPassword tests login with wrong password
func TestLoginFlow_InvalidPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	// Register user
	registerReq := dto.RegisterRequest{
		Email:    "wrongpass@example.com",
		Username: "wrongpass",
		Password: "Password123",
	}
	bodyReg, _ := json.Marshal(registerReq)
	reqReg := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyReg))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	handler.ServeHTTP(recReg, reqReg)
	require.Equal(t, http.StatusCreated, recReg.Code)

	// Login with wrong password
	loginReq := dto.LoginRequest{
		Email:    "wrongpass@example.com",
		Password: "WrongPass1",
	}
	bodyLogin, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLogin))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	handler.ServeHTTP(recLogin, reqLogin)

	assert.Equal(t, http.StatusUnauthorized, recLogin.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recLogin.Body).Decode(&errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
}

// TestRefreshFlow_Success tests refresh flow
func TestRefreshFlow_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	// Register and login to obtain refresh cookie
	registerReq := dto.RegisterRequest{
		Email:    "refresh@example.com",
		Username: "refreshuser",
		Password: "Password123",
	}
	bodyReg, _ := json.Marshal(registerReq)
	reqReg := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyReg))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	handler.ServeHTTP(recReg, reqReg)
	require.Equal(t, http.StatusCreated, recReg.Code)

	loginReq := dto.LoginRequest{
		Email:    "refresh@example.com",
		Password: "Password123",
	}
	bodyLogin, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLogin))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	handler.ServeHTTP(recLogin, reqLogin)
	require.Equal(t, http.StatusOK, recLogin.Code)

	var refreshCookie *http.Cookie
	for _, c := range recLogin.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	require.NotNil(t, refreshCookie)

	// Call refresh
	reqRefresh := httptest.NewRequest("POST", "/auth/v1/refresh", nil)
	reqRefresh.AddCookie(refreshCookie)
	recRefresh := httptest.NewRecorder()
	handler.ServeHTTP(recRefresh, reqRefresh)
	require.Equal(t, http.StatusOK, recRefresh.Code)

	var resp dto.AuthResponse
	require.NoError(t, json.NewDecoder(recRefresh.Body).Decode(&resp))
	assert.NotEmpty(t, resp.AccessToken)
	assert.Empty(t, resp.RefreshToken)

	// New refresh cookie set
	var newRefresh *http.Cookie
	for _, c := range recRefresh.Result().Cookies() {
		if c.Name == "refresh_token" {
			newRefresh = c
			break
		}
	}
	require.NotNil(t, newRefresh)
	assert.NotEqual(t, refreshCookie.Value, newRefresh.Value)

	// Authorization header present
	authHeader := recRefresh.Header().Get("Authorization")
	assert.True(t, strings.HasPrefix(authHeader, "Bearer "))
}

// TestRefreshFlow_InvalidRefresh tests refresh with invalid token
func TestRefreshFlow_InvalidRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	// Attempt refresh without valid cookie
	reqRefresh := httptest.NewRequest("POST", "/auth/v1/refresh", nil)
	recRefresh := httptest.NewRecorder()
	handler.ServeHTTP(recRefresh, reqRefresh)
	assert.Equal(t, http.StatusUnauthorized, recRefresh.Code)
}

// TestLoginFlow_UserNotFound tests login when user email does not exist
func TestLoginFlow_UserNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	loginReq := dto.LoginRequest{
		Email:    "missing@example.com",
		Password: "Password123",
	}
	bodyLogin, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLogin))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	handler.ServeHTTP(recLogin, reqLogin)

	assert.Equal(t, http.StatusNotFound, recLogin.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recLogin.Body).Decode(&errResp))
	assert.Equal(t, "NOT_FOUND", errResp.Code)
}

// TestPasswordResetFlow_FullFlow tests the complete password reset flow end-to-end
func TestPasswordResetFlow_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	// 1. Register and verify email
	registerReq := dto.RegisterRequest{
		Email:    "reset@example.com",
		Username: "resetuser",
		Password: "OldPassword123",
	}
	bodyRegister, _ := json.Marshal(registerReq)
	reqRegister := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyRegister))
	reqRegister.Header.Set("Content-Type", "application/json")
	recRegister := httptest.NewRecorder()
	handler.ServeHTTP(recRegister, reqRegister)
	assert.Equal(t, http.StatusCreated, recRegister.Code)

	// Get user ID and create verification token manually (for testing)
	var dbUser struct {
		ID int64
	}
	err = db.QueryRow("SELECT id FROM users WHERE email = $1", "reset@example.com").Scan(&dbUser.ID)
	require.NoError(t, err)

	// Manually create verification token (simulating what service does)
	rawVerifyToken := "rawtoken-reset-integration"
	sum := sha256.Sum256([]byte(rawVerifyToken))
	hashedVerify := hex.EncodeToString(sum[:])
	verifyPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":    dbUser.ID,
		"email":      "reset@example.com",
		"created_at": time.Now().Unix(),
	})
	require.NoError(t, app.redisClient.Set(context.Background(), "email_verification:"+hashedVerify, verifyPayload, time.Hour).Err())

	// Manually verify email (simulate user clicking link)
	verifyReq := dto.VerifyEmailRequest{Token: rawVerifyToken}
	bodyVerify, _ := json.Marshal(verifyReq)
	reqVerify := httptest.NewRequest("POST", "/auth/v1/verify-email", bytes.NewBuffer(bodyVerify))
	reqVerify.Header.Set("Content-Type", "application/json")
	recVerify := httptest.NewRecorder()
	handler.ServeHTTP(recVerify, reqVerify)
	assert.Equal(t, http.StatusOK, recVerify.Code)

	// 2. Login to get access token
	loginReq := dto.LoginRequest{
		Email:    "reset@example.com",
		Password: "OldPassword123",
	}
	bodyLogin, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLogin))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	handler.ServeHTTP(recLogin, reqLogin)
	assert.Equal(t, http.StatusOK, recLogin.Code)

	var loginResp dto.AuthResponse
	json.NewDecoder(recLogin.Body).Decode(&loginResp)
	accessToken := loginResp.AccessToken
	require.NotEmpty(t, accessToken)

	// 3. Request password reset (protected endpoint)
	reqReset := httptest.NewRequest("POST", "/auth/v1/request-password-reset", nil)
	reqReset.Header.Set("Authorization", "Bearer "+accessToken)
	recReset := httptest.NewRecorder()
	handler.ServeHTTP(recReset, reqReset)
	assert.Equal(t, http.StatusAccepted, recReset.Code)

	// 4. Get user ID for token creation (reuse dbUser from above)
	err = db.QueryRow("SELECT id FROM users WHERE email = $1", "reset@example.com").Scan(&dbUser.ID)
	require.NoError(t, err)

	// Manually create reset token (simulating what service does)
	rawResetToken := "rawresettoken-integration"
	sumReset := sha256.Sum256([]byte(rawResetToken))
	hashedReset := hex.EncodeToString(sumReset[:])
	resetPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":    dbUser.ID,
		"email":      "reset@example.com",
		"created_at": time.Now().Unix(),
	})
	require.NoError(t, app.redisClient.Set(context.Background(), "password_reset:"+hashedReset, resetPayload, time.Hour).Err())
	resetToken := rawResetToken

	// 5. Reset password using token
	resetPasswordReq := dto.ResetPasswordRequest{
		Token:       resetToken,
		NewPassword: "NewPassword123",
	}
	bodyReset, _ := json.Marshal(resetPasswordReq)
	reqResetPassword := httptest.NewRequest("POST", "/auth/v1/reset-password", bytes.NewBuffer(bodyReset))
	reqResetPassword.Header.Set("Content-Type", "application/json")
	recResetPassword := httptest.NewRecorder()
	handler.ServeHTTP(recResetPassword, reqResetPassword)
	assert.Equal(t, http.StatusOK, recResetPassword.Code)

	// 6. Verify token was deleted (one-time use)
	_, err = app.redisClient.Get(context.Background(), "password_reset:"+hashedReset).Result()
	require.Error(t, err)
	assert.ErrorIs(t, err, redis.Nil)

	// 7. Verify old password no longer works
	loginReqOld := dto.LoginRequest{
		Email:    "reset@example.com",
		Password: "OldPassword123",
	}
	bodyLoginOld, _ := json.Marshal(loginReqOld)
	reqLoginOld := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLoginOld))
	reqLoginOld.Header.Set("Content-Type", "application/json")
	recLoginOld := httptest.NewRecorder()
	handler.ServeHTTP(recLoginOld, reqLoginOld)
	assert.Equal(t, http.StatusUnauthorized, recLoginOld.Code)

	// 8. Verify new password works
	loginReqNew := dto.LoginRequest{
		Email:    "reset@example.com",
		Password: "NewPassword123",
	}
	bodyLoginNew, _ := json.Marshal(loginReqNew)
	reqLoginNew := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLoginNew))
	reqLoginNew.Header.Set("Content-Type", "application/json")
	recLoginNew := httptest.NewRecorder()
	handler.ServeHTTP(recLoginNew, reqLoginNew)
	assert.Equal(t, http.StatusOK, recLoginNew.Code)
}

// TestPasswordResetFlow_UnverifiedEmail tests that unverified users cannot request password reset
func TestPasswordResetFlow_UnverifiedEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	// Register user (not verified)
	registerReq := dto.RegisterRequest{
		Email:    "unverified@example.com",
		Username: "unverified",
		Password: "Password123",
	}
	bodyRegister, _ := json.Marshal(registerReq)
	reqRegister := httptest.NewRequest("POST", "/auth/v1/register", bytes.NewBuffer(bodyRegister))
	reqRegister.Header.Set("Content-Type", "application/json")
	recRegister := httptest.NewRecorder()
	handler.ServeHTTP(recRegister, reqRegister)
	assert.Equal(t, http.StatusCreated, recRegister.Code)

	// Login to get token
	loginReq := dto.LoginRequest{
		Email:    "unverified@example.com",
		Password: "Password123",
	}
	bodyLogin, _ := json.Marshal(loginReq)
	reqLogin := httptest.NewRequest("POST", "/auth/v1/login", bytes.NewBuffer(bodyLogin))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	handler.ServeHTTP(recLogin, reqLogin)
	assert.Equal(t, http.StatusOK, recLogin.Code)

	var loginResp dto.AuthResponse
	json.NewDecoder(recLogin.Body).Decode(&loginResp)
	accessToken := loginResp.AccessToken

	// Try to request password reset (should fail - email not verified)
	reqReset := httptest.NewRequest("POST", "/auth/v1/request-password-reset", nil)
	reqReset.Header.Set("Authorization", "Bearer "+accessToken)
	recReset := httptest.NewRecorder()
	handler.ServeHTTP(recReset, reqReset)
	assert.Equal(t, http.StatusUnauthorized, recReset.Code)

	var errResp dto.ErrorResponse
	json.NewDecoder(recReset.Body).Decode(&errResp)
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
	assert.Contains(t, errResp.Error, "email must be verified")
}

// TestPasswordResetFlow_InvalidToken tests reset with invalid token
func TestPasswordResetFlow_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	connStr, cleanup := setupTestDatabase(t)
	defer cleanup()

	db, err := cfgPkg.NewDB(connStr, 10, 5, "15m")
	require.NoError(t, err)
	defer db.Close()

	setupDatabaseSchema(t, db)
	app := setupTestApplication(t, db)
	handler := app.mount()

	resetReq := dto.ResetPasswordRequest{
		Token:       "invalid-token-12345",
		NewPassword: "NewPassword123",
	}
	bodyReset, _ := json.Marshal(resetReq)
	reqReset := httptest.NewRequest("POST", "/auth/v1/reset-password", bytes.NewBuffer(bodyReset))
	reqReset.Header.Set("Content-Type", "application/json")
	recReset := httptest.NewRecorder()
	handler.ServeHTTP(recReset, reqReset)

	assert.Equal(t, http.StatusUnauthorized, recReset.Code)
	var errResp dto.ErrorResponse
	json.NewDecoder(recReset.Body).Decode(&errResp)
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
	assert.Contains(t, errResp.Error, "invalid or expired")
}

// Helper function to extract raw token from Redis key (for testing)
func extractTokenFromRedisKey(key string) string {
	// Key format: "password_reset:<hashed>" or "email_verification:<hashed>"
	// We need to brute force or use a different approach
	// For testing, we'll store the raw token separately or use a test helper
	// This is a simplified version - in real scenario, token comes from email
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return ""
	}
	// We can't reverse the hash, so for integration tests we'll need a different approach
	// Let's use the service method to generate and retrieve token
	return ""
}
