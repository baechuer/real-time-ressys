package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/errors"
	authmw "github.com/baechuer/real-time-ressys/services/auth-service/app/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Register Handler Test Cases:

1. TestRegisterHandler_Success
   - Valid registration request
   - Returns 201 Created with RegisterResponse

2. TestRegisterHandler_InvalidJSON
   - Malformed JSON body -> 400 INVALID_INPUT

3. TestRegisterHandler_MissingRequiredFields
   - Missing fields -> 400 INVALID_INPUT

4. TestRegisterHandler_InvalidEmail
   - Invalid email format -> 400 INVALID_INPUT

5. TestRegisterHandler_PasswordTooShort
   - Password < 8 chars -> 400 INVALID_INPUT

6. TestRegisterHandler_PasswordMissingRequirements
   - Missing upper/lower/number -> 400 INVALID_INPUT

7. TestRegisterHandler_UsernameTooShort
   - Username < 3 chars -> 400 INVALID_INPUT

8. TestRegisterHandler_UsernameInvalidFormat
   - Invalid chars sanitized then accepted

9. TestRegisterHandler_DuplicateEmail
   - Conflict from service -> 409 CONFLICT

10. TestRegisterHandler_DatabaseError
    - Internal error from service -> 500 INTERNAL_ERROR

11. TestRegisterHandler_EmailSanitization
    - Email lowercased/trimmed before service call

12. TestRegisterHandler_UsernameSanitization
    - Username sanitized before service call

Login Handler Test Cases:

1. TestLoginHandler_Success
   - Valid login -> 200 with tokens

2. TestLoginHandler_InvalidJSON
   - Malformed JSON -> 400 INVALID_INPUT

3. TestLoginHandler_MissingRequiredFields
   - Missing password -> 400 INVALID_INPUT

4. TestLoginHandler_InvalidEmail
   - Invalid email -> 400 INVALID_INPUT

5. TestLoginHandler_PasswordTooShort
   - Password < 8 chars -> 400 INVALID_INPUT

6. TestLoginHandler_PasswordMissingRequirements
   - Missing strength requirements -> 400 INVALID_INPUT

7. TestLoginHandler_ServiceUnauthorized
   - Service denies credentials -> 401 UNAUTHORIZED

8. TestLoginHandler_ServiceError
   - Service returns internal error -> 500 INTERNAL_ERROR
*/

// mockAuthService is a mock implementation for testing
type mockAuthService struct {
	registerFunc             func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError)
	loginFunc                func(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, *errors.AppError)
	refreshFunc              func(ctx context.Context, token string) (*dto.AuthResponse, *errors.AppError)
	logoutFunc               func(ctx context.Context, accessToken, refreshToken string) *errors.AppError
	verifyFunc               func(ctx context.Context, req dto.VerifyEmailRequest) *errors.AppError
	requestPasswordResetFunc func(ctx context.Context, userID int64) *errors.AppError
	resetPasswordFunc        func(ctx context.Context, req dto.ResetPasswordRequest) *errors.AppError
}

func (m *mockAuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, req)
	}
	return nil, errors.NewInternal("mock not configured")
}

func (m *mockAuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, *errors.AppError) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, req)
	}
	return nil, errors.NewInternal("mock not configured")
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) (int64, *errors.AppError) {
	return 0, errors.NewInternal("not implemented")
}

func (m *mockAuthService) RequestPasswordReset(ctx context.Context, userID int64) *errors.AppError {
	if m.requestPasswordResetFunc != nil {
		return m.requestPasswordResetFunc(ctx, userID)
	}
	return errors.NewInternal("mock not configured")
}

func (m *mockAuthService) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) *errors.AppError {
	if m.resetPasswordFunc != nil {
		return m.resetPasswordFunc(ctx, req)
	}
	return errors.NewInternal("mock not configured")
}

func (m *mockAuthService) Refresh(ctx context.Context, token string) (*dto.AuthResponse, *errors.AppError) {
	if m.refreshFunc != nil {
		return m.refreshFunc(ctx, token)
	}
	return nil, errors.NewInternal("mock not configured")
}

func (m *mockAuthService) Logout(ctx context.Context, accessToken, refreshToken string) *errors.AppError {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, accessToken, refreshToken)
	}
	return errors.NewInternal("mock not configured")
}

func (m *mockAuthService) VerifyEmail(ctx context.Context, req dto.VerifyEmailRequest) *errors.AppError {
	if m.verifyFunc != nil {
		return m.verifyFunc(ctx, req)
	}
	return errors.NewInternal("mock not configured")
}

// setupTestApp creates a test application with mock auth service
func setupTestApp(mockService *mockAuthService) *testApplication {
	return &testApplication{
		config: config{
			addr: ":8080",
		},
		mockAuthService: mockService,
	}
}

// testApplication wraps application for testing with mock service
type testApplication struct {
	config          config
	mockAuthService *mockAuthService
}

func (app *testApplication) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest

	// 1. Parse JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	// 2. Sanitize inputs (before validation)
	req.Email = sanitizeEmail(req.Email, 255)
	req.Username = sanitizeUsername(req.Username, 50)
	req.Password = sanitizeInput(req.Password, 128, true)

	// 3. Validate DTO
	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	// 4. Call mock service
	resp, appErr := app.mockAuthService.Register(r.Context(), req)
	if appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (app *testApplication) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	req.Email = sanitizeEmail(req.Email, 255)
	req.Password = sanitizeInput(req.Password, 128, true)

	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	resp, appErr := app.mockAuthService.Login(r.Context(), req)
	if appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (app *testApplication) requestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	if appErr := app.mockAuthService.RequestPasswordReset(r.Context(), userID); appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (app *testApplication) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}

	req.NewPassword = sanitizeInput(req.NewPassword, 128, true)

	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	if appErr := app.mockAuthService.ResetPassword(r.Context(), req); appErr != nil {
		writeErrorResponse(w, appErr)
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

	resp, appErr := app.mockAuthService.Refresh(r.Context(), c.Value)
	if appErr != nil {
		writeErrorResponse(w, appErr)
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

func (app *testApplication) logoutHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "missing or invalid authorization header", http.StatusUnauthorized)
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	c, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "missing refresh token", http.StatusUnauthorized)
		return
	}

	if appErr := app.mockAuthService.Logout(r.Context(), accessToken, c.Value); appErr != nil {
		writeErrorResponse(w, appErr)
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
		writeErrorResponse(w, errors.NewInvalidInput("invalid request body"))
		return
	}
	if err := validateRequest(&req); err != nil {
		writeErrorResponse(w, err)
		return
	}

	if appErr := app.mockAuthService.VerifyEmail(r.Context(), req); appErr != nil {
		writeErrorResponse(w, appErr)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// TestRegisterHandler_Success tests successful user registration
func TestRegisterHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		registerFunc: func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
			return &dto.RegisterResponse{
				Message: "User registered successfully",
			}, nil
		},
	}

	app := setupTestApp(mockService)

	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response dto.RegisterResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "User registered successfully", response.Message)
}

// TestRegisterHandler_InvalidJSON tests invalid JSON in request body
func TestRegisterHandler_InvalidJSON(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/register", bytes.NewBufferString(`{"email": "test@example.com"`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResp dto.ErrorResponse
	err = json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", errorResp.Error)
	assert.Equal(t, "INVALID_INPUT", errorResp.Code)
}

// TestRegisterHandler_MissingRequiredFields tests missing required fields
func TestRegisterHandler_MissingRequiredFields(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.RegisterRequest{
		Email: "test@example.com",
		// Missing username and password
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResp dto.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", errorResp.Code)
	assert.Contains(t, errorResp.Error, "required")
}

// TestRegisterHandler_InvalidEmail tests invalid email format
func TestRegisterHandler_InvalidEmail(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.RegisterRequest{
		Email:    "not-an-email",
		Username: "testuser",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResp dto.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", errorResp.Code)
	assert.Contains(t, errorResp.Error, "email")
}

// TestRegisterHandler_PasswordTooShort tests password that's too short
func TestRegisterHandler_PasswordTooShort(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Pass1", // Too short (min 8)
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResp dto.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", errorResp.Code)
	assert.Contains(t, errorResp.Error, "Password")
	assert.Contains(t, errorResp.Error, "at least 8")
}

// TestRegisterHandler_PasswordMissingRequirements tests password missing strength requirements
func TestRegisterHandler_PasswordMissingRequirements(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	testCases := []struct {
		name     string
		password string
	}{
		{"Missing uppercase", "password123"},
		{"Missing lowercase", "PASSWORD123"},
		{"Missing number", "Password"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := dto.RegisterRequest{
				Email:    "test@example.com",
				Username: "testuser",
				Password: tc.password,
			}

			req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
			recorder := httptest.NewRecorder()

			app.registerHandler(recorder, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)

			var errorResp dto.ErrorResponse
			err := json.NewDecoder(recorder.Body).Decode(&errorResp)
			require.NoError(t, err)
			assert.Equal(t, "INVALID_INPUT", errorResp.Code)
			assert.Contains(t, errorResp.Error, "Password")
		})
	}
}

// TestRegisterHandler_UsernameTooShort tests username that's too short
func TestRegisterHandler_UsernameTooShort(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "ab", // Too short (min 3)
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var errorResp dto.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", errorResp.Code)
	assert.Contains(t, errorResp.Error, "Username")
	assert.Contains(t, errorResp.Error, "at least 3")
}

// TestRegisterHandler_UsernameInvalidFormat tests username with invalid characters
// Note: Invalid characters are sanitized (removed) before validation, so this should succeed
func TestRegisterHandler_UsernameInvalidFormat(t *testing.T) {
	var capturedRequest dto.RegisterRequest
	mockService := &mockAuthService{
		registerFunc: func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
			capturedRequest = req
			return &dto.RegisterResponse{Message: "User registered successfully"}, nil
		},
	}

	app := setupTestApp(mockService)

	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "user@name!", // Invalid characters will be sanitized
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	// Should succeed because sanitization removes invalid chars before validation
	assert.Equal(t, http.StatusCreated, recorder.Code)
	// Verify invalid characters were removed
	assert.Equal(t, "username", capturedRequest.Username)
}

// TestRegisterHandler_DuplicateEmail tests duplicate email registration
func TestRegisterHandler_DuplicateEmail(t *testing.T) {
	mockService := &mockAuthService{
		registerFunc: func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
			return nil, errors.NewConflict("email already in use")
		},
	}

	app := setupTestApp(mockService)

	reqBody := dto.RegisterRequest{
		Email:    "existing@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusConflict, recorder.Code)

	var errorResp dto.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "CONFLICT", errorResp.Code)
	assert.Equal(t, "email already in use", errorResp.Error)
}

// TestRegisterHandler_DatabaseError tests database error handling
func TestRegisterHandler_DatabaseError(t *testing.T) {
	mockService := &mockAuthService{
		registerFunc: func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
			return nil, errors.NewInternal("database error while checking email")
		},
	}

	app := setupTestApp(mockService)

	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	var errorResp dto.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&errorResp)
	require.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", errorResp.Code)
	assert.Contains(t, errorResp.Error, "database error")
}

// TestRegisterHandler_EmailSanitization tests email normalization (lowercase and trim)
func TestRegisterHandler_EmailSanitization(t *testing.T) {
	var capturedRequest dto.RegisterRequest
	mockService := &mockAuthService{
		registerFunc: func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
			capturedRequest = req
			return &dto.RegisterResponse{Message: "User registered successfully"}, nil
		},
	}

	app := setupTestApp(mockService)

	reqBody := dto.RegisterRequest{
		Email:    "  Test@Example.COM  ",
		Username: "testuser",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "test@example.com", capturedRequest.Email)
}

// TestRegisterHandler_UsernameSanitization tests username sanitization (remove special chars, trim)
func TestRegisterHandler_UsernameSanitization(t *testing.T) {
	var capturedRequest dto.RegisterRequest
	mockService := &mockAuthService{
		registerFunc: func(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *errors.AppError) {
			capturedRequest = req
			return &dto.RegisterResponse{Message: "User registered successfully"}, nil
		},
	}

	app := setupTestApp(mockService)

	reqBody := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "  user@name!  ",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/register", reqBody)
	recorder := httptest.NewRecorder()

	app.registerHandler(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "username", capturedRequest.Username)
}

// ---- Login handler tests ----

func TestLoginHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		loginFunc: func(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, *errors.AppError) {
			return &dto.AuthResponse{AccessToken: "token", RefreshToken: "refresh"}, nil
		},
	}
	app := setupTestApp(mockService)

	reqBody := dto.LoginRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var resp dto.AuthResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&resp))
	assert.Equal(t, "token", resp.AccessToken)
	assert.Equal(t, "refresh", resp.RefreshToken)
}

func TestLoginHandler_InvalidJSON(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/login", bytes.NewBufferString(`{"email": "test@example.com"`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Equal(t, "invalid request body", errResp.Error)
}

func TestLoginHandler_MissingRequiredFields(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.LoginRequest{
		Email: "test@example.com",
		// missing password
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "Password")
}

func TestLoginHandler_InvalidEmail(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.LoginRequest{
		Email:    "invalid-email",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "Email")
}

func TestLoginHandler_PasswordTooShort(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.LoginRequest{
		Email:    "test@example.com",
		Password: "short1", // less than 8
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "Password")
	assert.Contains(t, errResp.Error, "at least 8")
}

func TestLoginHandler_PasswordMissingRequirements(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.LoginRequest{
		Email:    "test@example.com",
		Password: "passwordonly", // missing upper and number
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "Password")
}

func TestLoginHandler_ServiceUnauthorized(t *testing.T) {
	mockService := &mockAuthService{
		loginFunc: func(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, *errors.AppError) {
			return nil, errors.NewUnauthorized("invalid credentials")
		},
	}
	app := setupTestApp(mockService)

	reqBody := dto.LoginRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
	assert.Equal(t, "invalid credentials", errResp.Error)
}

func TestLoginHandler_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		loginFunc: func(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, *errors.AppError) {
			return nil, errors.NewInternal("database unavailable")
		},
	}
	app := setupTestApp(mockService)

	reqBody := dto.LoginRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/login", reqBody)
	recorder := httptest.NewRecorder()

	app.loginHandler(recorder, req)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&errResp))
	assert.Equal(t, "INTERNAL_ERROR", errResp.Code)
	assert.Contains(t, errResp.Error, "database unavailable")
}

// ---- Verify email handler tests ----

func TestVerifyEmailHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		verifyFunc: func(ctx context.Context, req dto.VerifyEmailRequest) *errors.AppError {
			assert.Equal(t, "rawtoken", req.Token)
			return nil
		},
	}
	app := setupTestApp(mockService)

	body := map[string]string{"token": "rawtoken"}
	req := createTestRequest(t, "POST", "/auth/v1/verify-email", body)
	rec := httptest.NewRecorder()

	app.verifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestVerifyEmailHandler_InvalidJSON(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/verify-email", bytes.NewBufferString(`{"token":`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	app.verifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
}

func TestVerifyEmailHandler_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		verifyFunc: func(ctx context.Context, req dto.VerifyEmailRequest) *errors.AppError {
			return errors.NewUnauthorized("invalid or expired verification token")
		},
	}
	app := setupTestApp(mockService)

	body := map[string]string{"token": "badtoken"}
	req := createTestRequest(t, "POST", "/auth/v1/verify-email", body)
	rec := httptest.NewRecorder()

	app.verifyEmailHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
	assert.Equal(t, "invalid or expired verification token", errResp.Error)
}

// ---- Password Reset Handler Tests ----

func TestRequestPasswordResetHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		requestPasswordResetFunc: func(ctx context.Context, userID int64) *errors.AppError {
			assert.Equal(t, int64(123), userID)
			return nil
		},
	}
	app := setupTestApp(mockService)

	req, err := http.NewRequest("POST", "/auth/v1/request-password-reset", nil)
	require.NoError(t, err)
	// Simulate JWT middleware setting userID in context (using exported test helper)
	ctx := context.WithValue(req.Context(), authmw.TestContextUserID(), int64(123))
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.requestPasswordResetHandler(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestRequestPasswordResetHandler_MissingAuthToken(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/request-password-reset", nil)
	require.NoError(t, err)
	// No userID in context (simulating missing JWT auth)

	rec := httptest.NewRecorder()
	app.requestPasswordResetHandler(rec, req)

	// Should fail because userID not in context
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequestPasswordResetHandler_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		requestPasswordResetFunc: func(ctx context.Context, userID int64) *errors.AppError {
			return errors.NewUnauthorized("email must be verified before password reset")
		},
	}
	app := setupTestApp(mockService)

	req, err := http.NewRequest("POST", "/auth/v1/request-password-reset", nil)
	require.NoError(t, err)
	ctx := context.WithValue(req.Context(), authmw.TestContextUserID(), int64(123))
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.requestPasswordResetHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
}

func TestResetPasswordHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		resetPasswordFunc: func(ctx context.Context, req dto.ResetPasswordRequest) *errors.AppError {
			assert.Equal(t, "valid-token", req.Token)
			assert.Equal(t, "NewPassword123", req.NewPassword)
			return nil
		},
	}
	app := setupTestApp(mockService)

	reqBody := dto.ResetPasswordRequest{
		Token:       "valid-token",
		NewPassword: "NewPassword123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/reset-password", reqBody)
	rec := httptest.NewRecorder()

	app.resetPasswordHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestResetPasswordHandler_InvalidJSON(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/reset-password", bytes.NewBufferString(`{"token":`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	app.resetPasswordHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
}

func TestResetPasswordHandler_MissingToken(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.ResetPasswordRequest{
		Token:       "",
		NewPassword: "NewPassword123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/reset-password", reqBody)
	rec := httptest.NewRecorder()

	app.resetPasswordHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "Token")
}

func TestResetPasswordHandler_MissingPassword(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.ResetPasswordRequest{
		Token:       "some-token",
		NewPassword: "",
	}

	req := createTestRequest(t, "POST", "/auth/v1/reset-password", reqBody)
	rec := httptest.NewRecorder()

	app.resetPasswordHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "NewPassword")
}

func TestResetPasswordHandler_PasswordTooShort(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	reqBody := dto.ResetPasswordRequest{
		Token:       "some-token",
		NewPassword: "Short1", // Too short
	}

	req := createTestRequest(t, "POST", "/auth/v1/reset-password", reqBody)
	rec := httptest.NewRecorder()

	app.resetPasswordHandler(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "INVALID_INPUT", errResp.Code)
	assert.Contains(t, errResp.Error, "Password")
}

func TestResetPasswordHandler_PasswordMissingRequirements(t *testing.T) {
	testCases := []struct {
		name     string
		password string
	}{
		{"Missing uppercase", "password123"},
		{"Missing lowercase", "PASSWORD123"},
		{"Missing number", "Password"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(&mockAuthService{})

			reqBody := dto.ResetPasswordRequest{
				Token:       "some-token",
				NewPassword: tc.password,
			}

			req := createTestRequest(t, "POST", "/auth/v1/reset-password", reqBody)
			rec := httptest.NewRecorder()

			app.resetPasswordHandler(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			var errResp dto.ErrorResponse
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
			assert.Equal(t, "INVALID_INPUT", errResp.Code)
		})
	}
}

func TestResetPasswordHandler_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		resetPasswordFunc: func(ctx context.Context, req dto.ResetPasswordRequest) *errors.AppError {
			return errors.NewUnauthorized("invalid or expired reset token")
		},
	}
	app := setupTestApp(mockService)

	reqBody := dto.ResetPasswordRequest{
		Token:       "invalid-token",
		NewPassword: "NewPassword123",
	}

	req := createTestRequest(t, "POST", "/auth/v1/reset-password", reqBody)
	rec := httptest.NewRecorder()

	app.resetPasswordHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
}

// ---- Refresh Handler Tests ----

func TestRefreshHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		refreshFunc: func(ctx context.Context, token string) (*dto.AuthResponse, *errors.AppError) {
			assert.Equal(t, "valid-refresh-token", token)
			return &dto.AuthResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
			}, nil
		},
	}
	app := setupTestApp(mockService)

	req, err := http.NewRequest("POST", "/auth/v1/refresh", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "valid-refresh-token",
	})

	rec := httptest.NewRecorder()
	app.refreshHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Authorization"), "Bearer new-access-token")

	var resp dto.AuthResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "new-access-token", resp.AccessToken)
	assert.Empty(t, resp.RefreshToken) // Should be empty in response body
}

func TestRefreshHandler_MissingCookie(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/refresh", nil)
	require.NoError(t, err)
	// No cookie

	rec := httptest.NewRecorder()
	app.refreshHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing refresh token")
}

func TestRefreshHandler_EmptyCookie(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/refresh", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "", // Empty value
	})

	rec := httptest.NewRecorder()
	app.refreshHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefreshHandler_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		refreshFunc: func(ctx context.Context, token string) (*dto.AuthResponse, *errors.AppError) {
			return nil, errors.NewUnauthorized("invalid refresh token")
		},
	}
	app := setupTestApp(mockService)

	req, err := http.NewRequest("POST", "/auth/v1/refresh", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "invalid-token",
	})

	rec := httptest.NewRecorder()
	app.refreshHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp.Code)
}

// ---- Logout Handler Tests ----

func TestLogoutHandler_Success(t *testing.T) {
	mockService := &mockAuthService{
		logoutFunc: func(ctx context.Context, accessToken, refreshToken string) *errors.AppError {
			assert.Equal(t, "valid-access-token", accessToken)
			assert.Equal(t, "valid-refresh-token", refreshToken)
			return nil
		},
	}
	app := setupTestApp(mockService)

	req, err := http.NewRequest("POST", "/auth/v1/logout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer valid-access-token")
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "valid-refresh-token",
	})

	rec := httptest.NewRecorder()
	app.logoutHandler(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestLogoutHandler_MissingAuthHeader(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/logout", nil)
	require.NoError(t, err)
	// No Authorization header

	rec := httptest.NewRecorder()
	app.logoutHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogoutHandler_InvalidAuthHeader(t *testing.T) {
	app := setupTestApp(&mockAuthService{})

	req, err := http.NewRequest("POST", "/auth/v1/logout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "InvalidFormat")

	rec := httptest.NewRecorder()
	app.logoutHandler(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogoutHandler_ServiceError(t *testing.T) {
	mockService := &mockAuthService{
		logoutFunc: func(ctx context.Context, accessToken, refreshToken string) *errors.AppError {
			return errors.NewInternal("logout failed")
		},
	}
	app := setupTestApp(mockService)

	req, err := http.NewRequest("POST", "/auth/v1/logout", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token")
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "refresh",
	})

	rec := httptest.NewRecorder()
	app.logoutHandler(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var errResp dto.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errResp))
	assert.Equal(t, "INTERNAL_ERROR", errResp.Code)
}

// Helper function to create test request
func createTestRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("Failed to encode body: %v", err)
		}
	}

	req, err := http.NewRequest(method, path, &buf)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}
