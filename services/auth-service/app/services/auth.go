package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/circuitbreaker"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	appErrors "github.com/baechuer/real-time-ressys/services/auth-service/app/errors"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/logger"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/models"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/store"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication business logic
type AuthService struct {
	store         store.Storage
	redisClient   *redis.Client
	publisher     EventPublisher
	redisBreaker  *circuitbreaker.CircuitBreaker
	rabbitBreaker *circuitbreaker.CircuitBreaker
}

// NewAuthService creates a new AuthService
func NewAuthService(store store.Storage, redisClient *redis.Client, publisher EventPublisher) *AuthService {
	// Initialize circuit breakers with reasonable defaults
	// 5 failures before opening, 30s reset timeout, 3 calls in half-open
	redisBreaker := circuitbreaker.NewCircuitBreaker(5, 30*time.Second, 3)
	rabbitBreaker := circuitbreaker.NewCircuitBreaker(5, 30*time.Second, 3)

	return &AuthService{
		store:         store,
		redisClient:   redisClient,
		publisher:     publisher,
		redisBreaker:  redisBreaker,
		rabbitBreaker: rabbitBreaker,
	}
}

// Register handles user registration
// Note: Input validation (format, length, etc.) is already done in the handler layer
// This function focuses on business logic validation and processing
// Returns a simple success message - user must login separately
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, *appErrors.AppError) {
	// Check if user already exists (business logic validation)
	existingUser, err := s.store.Users.GetByEmail(ctx, req.Email)

	// Case 1: User found (no error) - email is already in use
	if err == nil && existingUser != nil {
		return nil, appErrors.NewConflict("email already in use")
	}

	// Case 2: Error occurred - check what type of error
	if err != nil {
		// Case 2a: Any other error is a real database problem
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.NewInternal("database error while checking email")
		}
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, appErrors.NewInternal("error hashing password")
	}

	// Create user in database
	user := &models.User{
		Email:           req.Email,
		PasswordHash:    string(passwordHash),
		Username:        req.Username,
		IsEmailVerified: false,
	}
	err = s.store.Users.Create(ctx, user)
	if err != nil {
		return nil, appErrors.NewInternal("error creating user")
	}

	// Generate and store email verification token, then publish event.
	if s.publisher != nil {
		if err := s.issueEmailVerificationToken(ctx, user.ID, user.Email); err != nil {
			// Use structured logging with request ID
			log := getLoggerFromContext(ctx)
			log.Error().
				Err(err).
				Int64("user_id", user.ID).
				Str("email", user.Email).
				Msg("failed to issue email verification token")
			// Do not fail registration if email sending fails.
		}
	}

	// Return simple success message - user must login separately
	return &dto.RegisterResponse{
		Message: "User registered successfully",
	}, nil
}

type emailVerificationData struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
}

const emailVerificationTTL = 24 * time.Hour

func emailVerificationKey(hashed string) string {
	return "email_verification:" + hashed
}

// issueEmailVerificationToken creates an opaque token, stores it in Redis, and publishes an event.
func (s *AuthService) issueEmailVerificationToken(ctx context.Context, userID int64, email string) error {
	raw, err := randomToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	hashed := hashToken(raw)

	data := emailVerificationData{
		UserID:    userID,
		Email:     email,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal verification data: %w", err)
	}

	// Use circuit breaker for Redis operations
	err = s.redisBreaker.Call(ctx, func() error {
		return s.redisClient.Set(
			ctx,
			emailVerificationKey(hashed),
			payload,
			emailVerificationTTL,
		).Err()
	})
	if err != nil {
		return fmt.Errorf("store verification token: %w", err)
	}

	baseURL := os.Getenv("FRONTEND_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", baseURL, raw)

	if s.publisher != nil {
		// Use circuit breaker for RabbitMQ operations
		err = s.rabbitBreaker.Call(ctx, func() error {
			return s.publisher.PublishEmailVerification(ctx, email, verificationURL)
		})
		if err != nil {
			return fmt.Errorf("publish verification event: %w", err)
		}
	}

	return nil
}

// Login handles user login
// Note: Input validation (format, length, etc.) is already done in the handler layer
// This function focuses on authentication logic
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, *appErrors.AppError) {
	//Get user by email
	user, err := s.store.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.NewNotFound("user not found")
		}
		return nil, appErrors.NewInternal("error getting user by email")
	}
	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, appErrors.NewUnauthorized("invalid password")
		}
		return nil, appErrors.NewInternal("error verifying password")
	}
	// Generate tokens
	refreshToken, err := generateRefreshToken(ctx, s.redisClient, user.ID, user.RoleID)
	if err != nil {
		return nil, appErrors.NewInternal("error generating refresh token")
	}
	accessToken, err := GenerateAccessToken(user.ID, user.RoleID)
	if err != nil {
		return nil, appErrors.NewInternal("error generating access token")
	}
	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			ID:              user.ID,
			Username:        user.Username,
			Email:           user.Email,
			IsEmailVerified: user.IsEmailVerified,
			RoleID:          user.RoleID,
			CreatedAt:       user.CreatedAt.Format(time.RFC3339),
		},
	}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, token string) (int64, *appErrors.AppError) {
	claims, err := ValidateAccessToken(ctx, s.redisClient, token)
	if err != nil {
		return 0, appErrors.NewUnauthorized("invalid or expired token")
	}
	return claims.UserID, nil
}

// ValidateRefreshToken validates an opaque refresh token against Redis.
func (s *AuthService) ValidateRefreshToken(ctx context.Context, token string) (*RefreshTokenData, *appErrors.AppError) {
	data, err := ParseRefreshToken(ctx, s.redisClient, token)
	if err != nil {
		return nil, appErrors.NewUnauthorized("invalid or expired refresh token")
	}
	return data, nil
}

// Refresh rotates a refresh token and issues a new access token (and new refresh).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*dto.AuthResponse, *appErrors.AppError) {
	if refreshToken == "" {
		return nil, appErrors.NewUnauthorized("missing refresh token")
	}

	newRefresh, data, err := rotateRefreshToken(ctx, s.redisClient, refreshToken)
	if err != nil {
		return nil, appErrors.NewUnauthorized("invalid or expired refresh token")
	}

	access, err := GenerateAccessToken(data.UserID, data.RoleID)
	if err != nil {
		return nil, appErrors.NewInternal("error generating access token")
	}

	return &dto.AuthResponse{
		AccessToken:  access,
		RefreshToken: newRefresh,
		User: dto.UserResponse{
			ID:     data.UserID,
			RoleID: data.RoleID,
		},
	}, nil
}

// VerifyEmail validates an email verification token, marks the user as verified, and invalidates the token.
func (s *AuthService) VerifyEmail(ctx context.Context, req dto.VerifyEmailRequest) *appErrors.AppError {
	if req.Token == "" {
		return appErrors.NewInvalidInput("missing verification token")
	}

	hashed := hashToken(req.Token)
	key := emailVerificationKey(hashed)

	var val string
	var err error
	err = s.redisBreaker.Call(ctx, func() error {
		var cbErr error
		val, cbErr = s.redisClient.Get(ctx, key).Result()
		return cbErr
	})
	if err != nil {
		if err == redis.Nil {
			return appErrors.NewUnauthorized("invalid or expired verification token")
		}
		return appErrors.NewInternal("failed to look up verification token")
	}

	var data emailVerificationData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return appErrors.NewInternal("failed to decode verification token data")
	}

	// Load user and mark as verified.
	user, err := s.store.Users.GetByEmail(ctx, data.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.NewNotFound("user not found for verification")
		}
		return appErrors.NewInternal("failed to load user for verification")
	}

	if user.IsEmailVerified {
		// Already verified; clean up token and return success.
		_ = s.redisBreaker.Call(ctx, func() error {
			return s.redisClient.Del(ctx, key).Err()
		})
		return nil
	}

	user.IsEmailVerified = true
	if err := s.store.Users.Update(ctx, user); err != nil {
		return appErrors.NewInternal("failed to update user verification status")
	}

	// One-time use token: delete it after successful verification.
	_ = s.redisBreaker.Call(ctx, func() error {
		return s.redisClient.Del(ctx, key).Err()
	})

	return nil
}

// Logout invalidates the access token (blacklist) and deletes the refresh token if provided.
func (s *AuthService) Logout(ctx context.Context, accessToken string, refreshToken string) *appErrors.AppError {
	if accessToken == "" {
		return appErrors.NewUnauthorized("missing access token")
	}

	// Validate first to ensure signature/exp are OK; then blacklist.
	if _, err := ValidateAccessToken(ctx, s.redisClient, accessToken); err != nil {
		return appErrors.NewUnauthorized("invalid or expired token")
	}
	if err := BlacklistAccessToken(ctx, s.redisClient, accessToken); err != nil {
		return appErrors.NewInternal("failed to blacklist token")
	}

	if refreshToken != "" {
		_ = deleteRefreshToken(ctx, s.redisClient, refreshToken)
	}
	return nil
}

type passwordResetData struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
}

const passwordResetTTL = 1 * time.Hour

func passwordResetKey(hashed string) string {
	return "password_reset:" + hashed
}

// RequestPasswordReset generates a password reset token for an authenticated user, stores it in Redis, and publishes an event.
func (s *AuthService) RequestPasswordReset(ctx context.Context, userID int64) *appErrors.AppError {
	// Load user to get email
	user, err := s.store.Users.GetByID(ctx, fmt.Sprintf("%d", userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.NewNotFound("user not found")
		}
		return appErrors.NewInternal("failed to load user")
	}

	if !user.IsEmailVerified {
		return appErrors.NewUnauthorized("email must be verified before password reset")
	}

	raw, err := randomToken()
	if err != nil {
		return appErrors.NewInternal("failed to generate reset token")
	}
	hashed := hashToken(raw)

	data := passwordResetData{
		UserID:    userID,
		Email:     user.Email,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return appErrors.NewInternal("failed to encode reset token data")
	}

	if err := s.redisClient.Set(
		ctx,
		passwordResetKey(hashed),
		payload,
		passwordResetTTL,
	).Err(); err != nil {
		return appErrors.NewInternal("failed to store reset token")
	}

	baseURL := os.Getenv("FRONTEND_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, raw)

	if s.publisher != nil {
		if err := s.publisher.PublishPasswordReset(ctx, user.Email, resetURL); err != nil {
			// Use structured logging with request ID
			log := getLoggerFromContext(ctx)
			log.Error().
				Err(err).
				Int64("user_id", userID).
				Str("email", user.Email).
				Msg("failed to publish password reset event")
		}
	}

	return nil
}

// ForgotPassword handles unauthenticated password reset requests (for users who forgot their password).
// It looks up the user by email and sends a password reset link if the user exists and email is verified.
// This endpoint does not reveal whether an email exists in the system (security best practice).
func (s *AuthService) ForgotPassword(ctx context.Context, req dto.ForgotPasswordRequest) *appErrors.AppError {
	// Look up user by email
	user, err := s.store.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		// Don't reveal if email exists - always return success for security
		// This prevents email enumeration attacks
		if errors.Is(err, sql.ErrNoRows) {
			// User doesn't exist, but return success anyway
			return nil
		}
		// Real database error - log but still return success to user
		log := getLoggerFromContext(ctx)
		log.Error().
			Err(err).
			Str("email", req.Email).
			Msg("database error while looking up user for password reset")
		return nil
	}

	// Only send reset email if email is verified
	if !user.IsEmailVerified {
		// Don't reveal that email exists but is unverified - return success
		return nil
	}

	// Generate reset token
	raw, err := randomToken()
	if err != nil {
		log := getLoggerFromContext(ctx)
		log.Error().
			Err(err).
			Int64("user_id", user.ID).
			Str("email", req.Email).
			Msg("failed to generate password reset token")
		return nil // Don't reveal error to user
	}
	hashed := hashToken(raw)

	data := passwordResetData{
		UserID:    user.ID,
		Email:     user.Email,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		log := getLoggerFromContext(ctx)
		log.Error().
			Err(err).
			Int64("user_id", user.ID).
			Msg("failed to encode password reset token data")
		return nil
	}

	// Store token in Redis with circuit breaker
	err = s.redisBreaker.Call(ctx, func() error {
		return s.redisClient.Set(
			ctx,
			passwordResetKey(hashed),
			payload,
			passwordResetTTL,
		).Err()
	})
	if err != nil {
		log := getLoggerFromContext(ctx)
		log.Error().
			Err(err).
			Int64("user_id", user.ID).
			Msg("failed to store password reset token")
		return nil
	}

	// Build reset URL
	baseURL := os.Getenv("FRONTEND_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, raw)

	// Publish password reset event with circuit breaker
	if s.publisher != nil {
		err = s.rabbitBreaker.Call(ctx, func() error {
			return s.publisher.PublishPasswordReset(ctx, user.Email, resetURL)
		})
		if err != nil {
			log := getLoggerFromContext(ctx)
			log.Error().
				Err(err).
				Int64("user_id", user.ID).
				Str("email", user.Email).
				Msg("failed to publish password reset event")
		}
	}

	// Always return success (don't reveal if email exists)
	return nil
}

// getLoggerFromContext retrieves logger from context or returns global logger
func getLoggerFromContext(ctx context.Context) zerolog.Logger {
	if log := zerolog.Ctx(ctx); log.GetLevel() != zerolog.Disabled {
		return *log
	}
	// Fallback to global logger
	return logger.Logger
}

// ResetPassword validates a password reset token and updates the user's password.
func (s *AuthService) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest) *appErrors.AppError {
	if req.Token == "" {
		return appErrors.NewInvalidInput("missing reset token")
	}
	if req.NewPassword == "" {
		return appErrors.NewInvalidInput("missing new password")
	}

	hashed := hashToken(req.Token)
	key := passwordResetKey(hashed)

	var val string
	var err error
	err = s.redisBreaker.Call(ctx, func() error {
		var cbErr error
		val, cbErr = s.redisClient.Get(ctx, key).Result()
		return cbErr
	})
	if err != nil {
		if err == redis.Nil {
			return appErrors.NewUnauthorized("invalid or expired reset token")
		}
		return appErrors.NewInternal("failed to look up reset token")
	}

	var data passwordResetData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return appErrors.NewInternal("failed to decode reset token data")
	}

	// Load user
	user, err := s.store.Users.GetByID(ctx, fmt.Sprintf("%d", data.UserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appErrors.NewNotFound("user not found")
		}
		return appErrors.NewInternal("failed to load user")
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return appErrors.NewInternal("failed to hash new password")
	}

	// Update password
	user.PasswordHash = string(newPasswordHash)
	if err := s.store.Users.Update(ctx, user); err != nil {
		return appErrors.NewInternal("failed to update password")
	}

	// One-time use token: delete it after successful reset
	_ = s.redisBreaker.Call(ctx, func() error {
		return s.redisClient.Del(ctx, key).Err()
	})

	return nil
}
