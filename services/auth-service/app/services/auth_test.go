package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/models"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/store"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

/*
AuthService Test Cases:

1. TestAuthService_Register_Success
   - User doesn't exist (sql.ErrNoRows)
   - Password is hashed
   - User is created in database
   - Returns RegisterResponse with success message

2. TestAuthService_Register_DuplicateEmail
   - User already exists
   - Returns 409 Conflict error

3. TestAuthService_Register_DatabaseError_GetByEmail
   - Database error when checking email
   - Returns 500 Internal Server Error

4. TestAuthService_Register_DatabaseError_Create
   - Database error when creating user
   - Returns 500 Internal Server Error

5. TestAuthService_Register_PasswordHashing
   - Password is hashed with bcrypt
   - Hashed password is different from plain password
   - Hashed password can be verified

6. TestAuthService_Register_UserFields
   - User created with correct fields
   - Email, username, password_hash set correctly
   - is_email_verified set to false

7. TestAuthService_Register_MultipleCalls_DifferentPasswords
   - Same password produces different hashes (due to salt)

8. TestAuthService_Register_ContextCancellation
   - Handles cancelled context without panic

9. TestAuthService_Login_Success
   - Valid credentials generate access/refresh tokens

10. TestAuthService_Login_InvalidPassword
    - Returns unauthorized when password mismatch

11. TestAuthService_Login_UserNotFound
    - Returns not found when email missing

12. TestAuthService_Login_GenerateTokenErrors
    - Handles token generation failures

13. TestAuthService_ValidateToken_NotImplemented (placeholder)
    - TODO: add tests once implemented
*/

// mockUsersStore is a mock implementation of the Users store interface
type mockUsersStore struct {
	getByEmailFunc func(ctx context.Context, email string) (*models.User, error)
	createFunc     func(ctx context.Context, user *models.User) error
	getByIDFunc    func(ctx context.Context, id string) (*models.User, error)
	getAllFunc     func(ctx context.Context) ([]models.User, error)
	updateFunc     func(ctx context.Context, user *models.User) error
	deleteFunc     func(ctx context.Context, id string) error
}

type mockPublisher struct {
	lastEmail string
	lastURL   string
	callCount int
	err       error
}

func (m *mockPublisher) PublishEmailVerification(ctx context.Context, email string, verificationURL string) error {
	m.lastEmail = email
	m.lastURL = verificationURL
	m.callCount++
	return m.err
}

func (m *mockPublisher) PublishPasswordReset(ctx context.Context, email string, resetURL string) error {
	m.lastEmail = email
	m.lastURL = resetURL
	m.callCount++
	return m.err
}

func (m *mockUsersStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getByEmailFunc != nil {
		return m.getByEmailFunc(ctx, email)
	}
	return nil, sql.ErrNoRows
}

func (m *mockUsersStore) Create(ctx context.Context, user *models.User) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, user)
	}
	return nil
}

func (m *mockUsersStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockUsersStore) GetAll(ctx context.Context) ([]models.User, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx)
	}
	return nil, nil
}

func (m *mockUsersStore) Update(ctx context.Context, user *models.User) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, user)
	}
	return nil
}

func (m *mockUsersStore) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

// setupMockStorage creates a mock storage for testing
func setupMockStorage(mockUsers *mockUsersStore) store.Storage {
	return store.Storage{
		Users: mockUsers,
	}
}

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

// TestAuthService_Register_Success tests successful user registration
func TestAuthService_Register_Success(t *testing.T) {
	var createdUser *models.User
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			// User doesn't exist
			return nil, sql.ErrNoRows
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			// Capture the user being created
			createdUser = user
			// Simulate database setting ID and CreatedAt
			user.ID = 1
			user.CreatedAt = time.Now()
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	resp, appErr := authService.Register(context.Background(), req)

	// Assertions
	require.Nil(t, appErr, "Should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, "User registered successfully", resp.Message)

	// Verify user was created with correct fields
	require.NotNil(t, createdUser, "User should be created")
	assert.Equal(t, "test@example.com", createdUser.Email)
	assert.Equal(t, "testuser", createdUser.Username)
	assert.False(t, createdUser.IsEmailVerified)
	assert.NotZero(t, createdUser.ID)
	assert.False(t, createdUser.CreatedAt.IsZero())

	// Verify password was hashed (not plain text)
	assert.NotEqual(t, "Password123", createdUser.PasswordHash)
	assert.True(t, len(createdUser.PasswordHash) > 0)
	assert.Contains(t, createdUser.PasswordHash, "$2a$") // bcrypt hash prefix

	// Verify password hash can be verified
	err := bcrypt.CompareHashAndPassword([]byte(createdUser.PasswordHash), []byte("Password123"))
	assert.NoError(t, err, "Password hash should be verifiable")
}

func TestAuthService_Register_StoresVerificationTokenAndPublishes(t *testing.T) {
	ctx := context.Background()
	var createdUser *models.User

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			createdUser = user
			user.ID = 42
			user.CreatedAt = time.Now()
			return nil
		},
	}

	publisher := &mockPublisher{}
	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, publisher)

	req := dto.RegisterRequest{
		Email:    "verify@example.com",
		Username: "verifyuser",
		Password: "Password123!",
	}

	resp, appErr := authService.Register(ctx, req)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	require.NotNil(t, createdUser)

	require.Equal(t, 1, publisher.callCount)
	assert.Equal(t, "verify@example.com", publisher.lastEmail)

	u, err := url.Parse(publisher.lastURL)
	require.NoError(t, err)
	rawToken := u.Query().Get("token")
	require.NotEmpty(t, rawToken)

	hashed := hashToken(rawToken)
	keys := redisClient.Keys(ctx, "email_verification:*").Val()
	require.Len(t, keys, 1)
	assert.Equal(t, "email_verification:"+hashed, keys[0])

	val, err := redisClient.Get(ctx, keys[0]).Result()
	require.NoError(t, err)

	var data emailVerificationData
	require.NoError(t, json.Unmarshal([]byte(val), &data))
	assert.Equal(t, int64(42), data.UserID)
	assert.Equal(t, "verify@example.com", data.Email)
}

// TestAuthService_Register_DuplicateEmail tests duplicate email registration
func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	existingUser := &models.User{
		ID:       1,
		Email:    "existing@example.com",
		Username: "existinguser",
	}

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			// User already exists
			return existingUser, nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.RegisterRequest{
		Email:    "existing@example.com",
		Username: "newuser",
		Password: "Password123",
	}

	resp, appErr := authService.Register(context.Background(), req)

	// Assertions
	assert.Nil(t, resp, "Response should be nil on error")
	require.NotNil(t, appErr, "Should return error")
	assert.Equal(t, "CONFLICT", string(appErr.Code))
	assert.Equal(t, "email already in use", appErr.Message)
	assert.Equal(t, 409, appErr.Status)
}

// TestAuthService_Register_DatabaseError_GetByEmail tests database error when checking email
func TestAuthService_Register_DatabaseError_GetByEmail(t *testing.T) {
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			// Database error (not sql.ErrNoRows)
			return nil, errors.New("connection refused")
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	resp, appErr := authService.Register(context.Background(), req)

	// Assertions
	assert.Nil(t, resp, "Response should be nil on error")
	require.NotNil(t, appErr, "Should return error")
	assert.Equal(t, "INTERNAL_ERROR", string(appErr.Code))
	assert.Contains(t, appErr.Message, "database error")
	assert.Equal(t, 500, appErr.Status)
}

// TestAuthService_Register_DatabaseError_Create tests database error when creating user
func TestAuthService_Register_DatabaseError_Create(t *testing.T) {
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			// User doesn't exist
			return nil, sql.ErrNoRows
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			// Database error when creating
			return errors.New("unique constraint violation")
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	resp, appErr := authService.Register(context.Background(), req)

	// Assertions
	assert.Nil(t, resp, "Response should be nil on error")
	require.NotNil(t, appErr, "Should return error")
	assert.Equal(t, "INTERNAL_ERROR", string(appErr.Code))
	assert.Contains(t, appErr.Message, "error creating user")
	assert.Equal(t, 500, appErr.Status)
}

// TestAuthService_Register_PasswordHashing tests password hashing
func TestAuthService_Register_PasswordHashing(t *testing.T) {
	var capturedPasswordHash string
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			capturedPasswordHash = user.PasswordHash
			user.ID = 1
			user.CreatedAt = time.Now()
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "MySecurePassword123",
	}

	resp, appErr := authService.Register(context.Background(), req)

	// Assertions
	require.Nil(t, appErr, "Should not return error")
	require.NotNil(t, resp, "Response should not be nil")

	// Verify password was hashed
	assert.NotEqual(t, "MySecurePassword123", capturedPasswordHash, "Password should be hashed")
	assert.True(t, len(capturedPasswordHash) > 50, "Bcrypt hash should be long")
	assert.Contains(t, capturedPasswordHash, "$2a$", "Should be bcrypt hash")

	// Verify hash can verify the original password
	err := bcrypt.CompareHashAndPassword([]byte(capturedPasswordHash), []byte("MySecurePassword123"))
	assert.NoError(t, err, "Hash should verify original password")

	// Verify hash does NOT verify wrong password
	err = bcrypt.CompareHashAndPassword([]byte(capturedPasswordHash), []byte("WrongPassword"))
	assert.Error(t, err, "Hash should NOT verify wrong password")
}

// TestAuthService_Register_UserFields tests user fields are set correctly
func TestAuthService_Register_UserFields(t *testing.T) {
	var createdUser *models.User
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			createdUser = user
			user.ID = 42
			user.CreatedAt = time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.RegisterRequest{
		Email:    "user@example.com",
		Username: "myusername",
		Password: "SecurePass123",
	}

	resp, appErr := authService.Register(context.Background(), req)

	// Assertions
	require.Nil(t, appErr, "Should not return error")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, createdUser, "User should be created")

	// Verify all fields are set correctly
	assert.Equal(t, "user@example.com", createdUser.Email)
	assert.Equal(t, "myusername", createdUser.Username)
	assert.False(t, createdUser.IsEmailVerified, "is_email_verified should be false")
	assert.NotEmpty(t, createdUser.PasswordHash, "Password hash should be set")
	assert.NotEqual(t, "SecurePass123", createdUser.PasswordHash, "Password should be hashed")
}

// TestAuthService_Register_MultipleCalls_DifferentPasswords tests that different passwords produce different hashes
func TestAuthService_Register_MultipleCalls_DifferentPasswords(t *testing.T) {
	var hash1, hash2 string
	callCount := 0

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			callCount++
			if callCount == 1 {
				hash1 = user.PasswordHash
			} else {
				hash2 = user.PasswordHash
			}
			user.ID = int64(callCount)
			user.CreatedAt = time.Now()
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	// Register first user
	req1 := dto.RegisterRequest{
		Email:    "user1@example.com",
		Username: "user1",
		Password: "Password123",
	}
	_, appErr1 := authService.Register(context.Background(), req1)
	require.Nil(t, appErr1, "First registration should succeed")

	// Register second user with same password
	req2 := dto.RegisterRequest{
		Email:    "user2@example.com",
		Username: "user2",
		Password: "Password123",
	}
	_, appErr2 := authService.Register(context.Background(), req2)
	require.Nil(t, appErr2, "Second registration should succeed")

	// Verify hashes are different (bcrypt uses random salt)
	assert.NotEqual(t, hash1, hash2, "Same password should produce different hashes (due to salt)")

	// But both should verify the same password
	err := bcrypt.CompareHashAndPassword([]byte(hash1), []byte("Password123"))
	assert.NoError(t, err)
	err = bcrypt.CompareHashAndPassword([]byte(hash2), []byte("Password123"))
	assert.NoError(t, err)
}

// TestAuthService_Register_ContextCancellation tests context cancellation handling
func TestAuthService_Register_ContextCancellation(t *testing.T) {
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return nil, sql.ErrNoRows
			}
		},
		createFunc: func(ctx context.Context, user *models.User) error {
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	resp, appErr := authService.Register(ctx, req)

	// Should handle context cancellation gracefully
	// (In real scenario, this would propagate the error)
	// For now, we just verify it doesn't panic
	_ = resp
	_ = appErr
}

func TestAuthService_VerifyEmail_Success(t *testing.T) {
	ctx := context.Background()
	redisClient := newTestRedisClient(t)

	rawToken := "rawtoken"
	hashed := hashToken(rawToken)
	payload, _ := json.Marshal(emailVerificationData{
		UserID:    7,
		Email:     "verify@example.com",
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, redisClient.Set(ctx, emailVerificationKey(hashed), payload, time.Hour).Err())

	updateCalled := 0
	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:              7,
				Email:           email,
				IsEmailVerified: false,
			}, nil
		},
		updateFunc: func(ctx context.Context, user *models.User) error {
			updateCalled++
			assert.True(t, user.IsEmailVerified)
			return nil
		},
	}

	authSvc := NewAuthService(setupMockStorage(mockUsers), redisClient, nil)

	appErr := authSvc.VerifyEmail(ctx, dto.VerifyEmailRequest{Token: rawToken})
	require.Nil(t, appErr)
	assert.Equal(t, 1, updateCalled)

	// Token should be deleted after successful verification
	_, err := redisClient.Get(ctx, emailVerificationKey(hashed)).Result()
	assert.Equal(t, redis.Nil, err)
}

func TestAuthService_VerifyEmail_InvalidToken(t *testing.T) {
	redisClient := newTestRedisClient(t)
	authSvc := NewAuthService(store.Storage{}, redisClient, nil)

	appErr := authSvc.VerifyEmail(context.Background(), dto.VerifyEmailRequest{Token: "does-not-exist"})
	require.NotNil(t, appErr)
	assert.Equal(t, "invalid or expired verification token", appErr.Message)
}

func TestAuthService_Logout_BlacklistsAccessTokenAndDeletesRefresh(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	authSvc := NewAuthService(store.Storage{}, rdb, nil)

	// Generate tokens
	access, err := GenerateAccessToken(123, 1)
	require.NoError(t, err)
	refresh, err := generateRefreshToken(context.Background(), rdb, 123, 1)
	require.NoError(t, err)

	// Validate works before logout
	_, err = ValidateAccessToken(context.Background(), rdb, access)
	require.NoError(t, err)

	// Logout
	errApp := authSvc.Logout(context.Background(), access, refresh)
	require.Nil(t, errApp)

	// Access token should be revoked
	_, err = ValidateAccessToken(context.Background(), rdb, access)
	assert.Error(t, err)

	// Refresh token should be deleted
	_, err = validateRefreshToken(context.Background(), rdb, refresh)
	assert.Error(t, err)
}

func TestAuthService_Refresh_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	authSvc := NewAuthService(store.Storage{}, rdb, nil)

	refresh, err := generateRefreshToken(context.Background(), rdb, 5, 2)
	require.NoError(t, err)

	resp, appErr := authSvc.Refresh(context.Background(), refresh)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
}

func TestAuthService_Refresh_Invalid(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	authSvc := NewAuthService(store.Storage{}, rdb, nil)

	resp, appErr := authSvc.Refresh(context.Background(), "bogus")
	assert.Nil(t, resp)
	require.NotNil(t, appErr)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}

// ---- Password Reset Tests ----

func TestAuthService_RequestPasswordReset_Success(t *testing.T) {
	ctx := context.Background()
	userID := int64(42)
	userEmail := "user@example.com"

	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			assert.Equal(t, "42", id)
			return &models.User{
				ID:              userID,
				Email:           userEmail,
				IsEmailVerified: true,
			}, nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	publisher := &mockPublisher{}
	authService := NewAuthService(storage, redisClient, publisher)

	appErr := authService.RequestPasswordReset(ctx, userID)

	require.Nil(t, appErr)
	assert.Equal(t, 1, publisher.callCount)
	assert.Equal(t, userEmail, publisher.lastEmail)
	assert.Contains(t, publisher.lastURL, "reset-password?token=")

	// Verify token was stored in Redis
	u, err := url.Parse(publisher.lastURL)
	require.NoError(t, err)
	rawToken := u.Query().Get("token")
	require.NotEmpty(t, rawToken)

	hashed := hashToken(rawToken)
	key := passwordResetKey(hashed)
	val, err := redisClient.Get(ctx, key).Result()
	require.NoError(t, err)

	var data passwordResetData
	err = json.Unmarshal([]byte(val), &data)
	require.NoError(t, err)
	assert.Equal(t, userID, data.UserID)
	assert.Equal(t, userEmail, data.Email)
}

func TestAuthService_RequestPasswordReset_UnverifiedEmail(t *testing.T) {
	ctx := context.Background()
	userID := int64(42)

	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return &models.User{
				ID:              userID,
				Email:           "user@example.com",
				IsEmailVerified: false, // Not verified
			}, nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	appErr := authService.RequestPasswordReset(ctx, userID)

	require.NotNil(t, appErr)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
	assert.Contains(t, appErr.Message, "email must be verified")
}

func TestAuthService_RequestPasswordReset_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userID := int64(999)

	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	appErr := authService.RequestPasswordReset(ctx, userID)

	require.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", string(appErr.Code))
}

func TestAuthService_RequestPasswordReset_PublishesToRabbitMQ(t *testing.T) {
	ctx := context.Background()
	userID := int64(42)

	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return &models.User{
				ID:              userID,
				Email:           "test@example.com",
				IsEmailVerified: true,
			}, nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	publisher := &mockPublisher{}
	authService := NewAuthService(storage, redisClient, publisher)

	appErr := authService.RequestPasswordReset(ctx, userID)

	require.Nil(t, appErr)
	assert.Equal(t, 1, publisher.callCount)
	assert.Equal(t, "test@example.com", publisher.lastEmail)
	assert.Contains(t, publisher.lastURL, "reset-password")
}

func TestAuthService_ResetPassword_Success(t *testing.T) {
	ctx := context.Background()
	userID := int64(42)
	userEmail := "user@example.com"
	oldPasswordHash := "$2a$10$oldhash"
	newPassword := "NewPassword123"

	// Setup: Store reset token in Redis
	rawToken, err := randomToken()
	require.NoError(t, err)
	hashed := hashToken(rawToken)
	key := passwordResetKey(hashed)

	data := passwordResetData{
		UserID:    userID,
		Email:     userEmail,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(data)
	require.NoError(t, err)

	redisClient := newTestRedisClient(t)
	err = redisClient.Set(ctx, key, payload, passwordResetTTL).Err()
	require.NoError(t, err)

	// Setup: Mock user store
	var updatedUser *models.User
	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return &models.User{
				ID:           userID,
				Email:        userEmail,
				PasswordHash: oldPasswordHash,
			}, nil
		},
		updateFunc: func(ctx context.Context, user *models.User) error {
			updatedUser = user
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       rawToken,
		NewPassword: newPassword,
	}

	appErr := authService.ResetPassword(ctx, req)

	require.Nil(t, appErr)
	require.NotNil(t, updatedUser)
	assert.NotEqual(t, oldPasswordHash, updatedUser.PasswordHash)
	assert.Contains(t, updatedUser.PasswordHash, "$2a$") // bcrypt hash

	// Verify password can be verified
	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte(newPassword))
	assert.NoError(t, err)

	// Verify token was deleted (one-time use)
	_, err = redisClient.Get(ctx, key).Result()
	assert.Equal(t, redis.Nil, err)
}

func TestAuthService_ResetPassword_InvalidToken(t *testing.T) {
	ctx := context.Background()
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(store.Storage{}, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       "invalid-token",
		NewPassword: "NewPassword123",
	}

	appErr := authService.ResetPassword(ctx, req)

	require.NotNil(t, appErr)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
	assert.Contains(t, appErr.Message, "invalid or expired")
}

func TestAuthService_ResetPassword_ExpiredToken(t *testing.T) {
	ctx := context.Background()

	// Create token but don't store it (simulating expiration)
	rawToken, err := randomToken()
	require.NoError(t, err)

	redisClient := newTestRedisClient(t)
	authService := NewAuthService(store.Storage{}, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       rawToken,
		NewPassword: "NewPassword123",
	}

	appErr := authService.ResetPassword(ctx, req)

	require.NotNil(t, appErr)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}

func TestAuthService_ResetPassword_MissingToken(t *testing.T) {
	ctx := context.Background()
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(store.Storage{}, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       "",
		NewPassword: "NewPassword123",
	}

	appErr := authService.ResetPassword(ctx, req)

	require.NotNil(t, appErr)
	assert.Equal(t, "INVALID_INPUT", string(appErr.Code))
	assert.Contains(t, appErr.Message, "missing reset token")
}

func TestAuthService_ResetPassword_MissingPassword(t *testing.T) {
	ctx := context.Background()
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(store.Storage{}, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       "some-token",
		NewPassword: "",
	}

	appErr := authService.ResetPassword(ctx, req)

	require.NotNil(t, appErr)
	assert.Equal(t, "INVALID_INPUT", string(appErr.Code))
	assert.Contains(t, appErr.Message, "missing new password")
}

func TestAuthService_ResetPassword_TokenOneTimeUse(t *testing.T) {
	ctx := context.Background()
	userID := int64(42)

	// Setup: Store reset token
	rawToken, err := randomToken()
	require.NoError(t, err)
	hashed := hashToken(rawToken)
	key := passwordResetKey(hashed)

	data := passwordResetData{
		UserID:    userID,
		Email:     "user@example.com",
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(data)
	require.NoError(t, err)

	redisClient := newTestRedisClient(t)
	err = redisClient.Set(ctx, key, payload, passwordResetTTL).Err()
	require.NoError(t, err)

	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return &models.User{
				ID:           userID,
				Email:        "user@example.com",
				PasswordHash: "$2a$10$old",
			}, nil
		},
		updateFunc: func(ctx context.Context, user *models.User) error {
			return nil
		},
	}

	storage := setupMockStorage(mockUsers)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       rawToken,
		NewPassword: "NewPassword123",
	}

	// First use - should succeed
	appErr := authService.ResetPassword(ctx, req)
	require.Nil(t, appErr)

	// Second use - should fail (token deleted)
	appErr = authService.ResetPassword(ctx, req)
	require.NotNil(t, appErr)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}

func TestAuthService_ResetPassword_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userID := int64(999)

	// Setup: Store reset token
	rawToken, err := randomToken()
	require.NoError(t, err)
	hashed := hashToken(rawToken)
	key := passwordResetKey(hashed)

	data := passwordResetData{
		UserID:    userID,
		Email:     "nonexistent@example.com",
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(data)
	require.NoError(t, err)

	redisClient := newTestRedisClient(t)
	err = redisClient.Set(ctx, key, payload, passwordResetTTL).Err()
	require.NoError(t, err)

	mockUsers := &mockUsersStore{
		getByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
	}

	storage := setupMockStorage(mockUsers)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.ResetPasswordRequest{
		Token:       rawToken,
		NewPassword: "NewPassword123",
	}

	appErr := authService.ResetPassword(ctx, req)

	require.NotNil(t, appErr)
	assert.Equal(t, "NOT_FOUND", string(appErr.Code))
}

// ---- Login Tests ----

func TestAuthService_Login_Success(t *testing.T) {
	ctx := context.Background()
	userEmail := "login@example.com"
	userPassword := "Password123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			assert.Equal(t, userEmail, email)
			return &models.User{
				ID:           42,
				Email:        userEmail,
				PasswordHash: string(hashedPassword),
				RoleID:       1,
			}, nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	t.Setenv("JWT_SECRET", "test-secret-key")

	authService := NewAuthService(storage, redisClient, nil)

	req := dto.LoginRequest{
		Email:    userEmail,
		Password: userPassword,
	}

	resp, appErr := authService.Login(ctx, req)

	require.Nil(t, appErr)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, int64(42), resp.User.ID)
	assert.Equal(t, userEmail, resp.User.Email)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	ctx := context.Background()

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, sql.ErrNoRows
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.LoginRequest{
		Email:    "notfound@example.com",
		Password: "Password123",
	}

	resp, appErr := authService.Login(ctx, req)

	require.NotNil(t, appErr)
	assert.Nil(t, resp)
	assert.Equal(t, "NOT_FOUND", string(appErr.Code))
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	ctx := context.Background()
	userEmail := "user@example.com"
	correctPassword := "CorrectPassword123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return &models.User{
				ID:           42,
				Email:        userEmail,
				PasswordHash: string(hashedPassword),
			}, nil
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.LoginRequest{
		Email:    userEmail,
		Password: "WrongPassword123",
	}

	resp, appErr := authService.Login(ctx, req)

	require.NotNil(t, appErr)
	assert.Nil(t, resp)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
	assert.Contains(t, appErr.Message, "invalid password")
}

func TestAuthService_Login_DatabaseError(t *testing.T) {
	ctx := context.Background()

	mockUsers := &mockUsersStore{
		getByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return nil, errors.New("database connection failed")
		},
	}

	storage := setupMockStorage(mockUsers)
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(storage, redisClient, nil)

	req := dto.LoginRequest{
		Email:    "user@example.com",
		Password: "Password123",
	}

	resp, appErr := authService.Login(ctx, req)

	require.NotNil(t, appErr)
	assert.Nil(t, resp)
	assert.Equal(t, "INTERNAL_ERROR", string(appErr.Code))
}

// ---- ValidateToken Tests ----

func TestAuthService_ValidateToken_Success(t *testing.T) {
	ctx := context.Background()
	t.Setenv("JWT_SECRET", "test-secret-key")

	redisClient := newTestRedisClient(t)
	userID := int64(123)
	roleID := 1

	accessToken, err := GenerateAccessToken(userID, roleID)
	require.NoError(t, err)

	authService := NewAuthService(store.Storage{}, redisClient, nil)

	validatedUserID, appErr := authService.ValidateToken(ctx, accessToken)

	require.Nil(t, appErr)
	assert.Equal(t, userID, validatedUserID)
}

func TestAuthService_ValidateToken_InvalidToken(t *testing.T) {
	ctx := context.Background()
	t.Setenv("JWT_SECRET", "test-secret-key")

	redisClient := newTestRedisClient(t)
	authService := NewAuthService(store.Storage{}, redisClient, nil)

	validatedUserID, appErr := authService.ValidateToken(ctx, "invalid-token")

	require.NotNil(t, appErr)
	assert.Equal(t, int64(0), validatedUserID)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}

func TestAuthService_ValidateToken_RevokedToken(t *testing.T) {
	ctx := context.Background()
	t.Setenv("JWT_SECRET", "test-secret-key")

	redisClient := newTestRedisClient(t)
	userID := int64(123)
	roleID := 1

	accessToken, err := GenerateAccessToken(userID, roleID)
	require.NoError(t, err)

	// Revoke the token
	err = BlacklistAccessToken(ctx, redisClient, accessToken)
	require.NoError(t, err)

	authService := NewAuthService(store.Storage{}, redisClient, nil)

	validatedUserID, appErr := authService.ValidateToken(ctx, accessToken)

	require.NotNil(t, appErr)
	assert.Equal(t, int64(0), validatedUserID)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}

// ---- ValidateRefreshToken Tests ----

func TestAuthService_ValidateRefreshToken_Success(t *testing.T) {
	ctx := context.Background()
	t.Setenv("JWT_SECRET", "test-secret-key")

	redisClient := newTestRedisClient(t)
	userID := int64(456)
	roleID := 2

	refreshToken, err := generateRefreshToken(ctx, redisClient, userID, roleID)
	require.NoError(t, err)

	authService := NewAuthService(store.Storage{}, redisClient, nil)

	data, appErr := authService.ValidateRefreshToken(ctx, refreshToken)

	require.Nil(t, appErr)
	require.NotNil(t, data)
	assert.Equal(t, userID, data.UserID)
	assert.Equal(t, roleID, data.RoleID)
}

func TestAuthService_ValidateRefreshToken_InvalidToken(t *testing.T) {
	ctx := context.Background()
	redisClient := newTestRedisClient(t)
	authService := NewAuthService(store.Storage{}, redisClient, nil)

	data, appErr := authService.ValidateRefreshToken(ctx, "invalid-refresh-token")

	require.NotNil(t, appErr)
	assert.Nil(t, data)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}

func TestAuthService_ValidateRefreshToken_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	t.Setenv("JWT_SECRET", "test-secret-key")

	redisClient := newTestRedisClient(t)
	userID := int64(789)
	roleID := 1

	refreshToken, err := generateRefreshToken(ctx, redisClient, userID, roleID)
	require.NoError(t, err)

	// Manually expire the token by deleting it from Redis
	hashed := hashToken(refreshToken)
	key := refreshTokenKey(hashed)
	redisClient.Del(ctx, key)

	authService := NewAuthService(store.Storage{}, redisClient, nil)

	data, appErr := authService.ValidateRefreshToken(ctx, refreshToken)

	require.NotNil(t, appErr)
	assert.Nil(t, data)
	assert.Equal(t, "UNAUTHORIZED", string(appErr.Code))
}
