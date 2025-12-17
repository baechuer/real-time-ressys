package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// jwtSecret is loaded lazily so we can validate it and avoid an empty secret.
var (
	jwtSecret     []byte
	secretLoadErr error
	secretOnce    sync.Once
)

func getJWTSecret() ([]byte, error) {
	secretOnce.Do(func() {
		val := os.Getenv("JWT_SECRET")
		if val == "" {
			secretLoadErr = fmt.Errorf("JWT_SECRET is not set")
			return
		}
		jwtSecret = []byte(val)
	})
	if secretLoadErr != nil {
		return nil, secretLoadErr
	}
	return jwtSecret, nil
}

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

const (
	blacklistPrefix = "blacklist:access_token:"
)

// AccessClaims represents JWT claims for access tokens.
type AccessClaims struct {
	UserID int64 `json:"sub"`
	RoleID int   `json:"role_id"`
	jwt.RegisteredClaims
}

// RefreshTokenData holds metadata stored in Redis (hashed token is the key).
type RefreshTokenData struct {
	UserID    int64  `json:"user_id"`
	RoleID    int    `json:"role_id"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
	DeviceID  string `json:"device_id,omitempty"`
}

// generateRefreshToken creates an opaque refresh token, hashes it, and stores it in Redis with TTL.
func generateRefreshToken(ctx context.Context, rdb *redis.Client, userID int64, roleID int) (string, error) {
	raw, err := randomToken()
	if err != nil {
		return "", err
	}

	hashed := hashToken(raw)
	now := time.Now()
	data := RefreshTokenData{
		UserID:    userID,
		RoleID:    roleID,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(refreshTokenTTL).Unix(),
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal refresh token data: %w", err)
	}

	key := refreshTokenKey(hashed)
	if err := rdb.Set(ctx, key, payload, refreshTokenTTL).Err(); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}

	return raw, nil
}

// validateRefreshToken verifies an opaque refresh token against Redis.
func validateRefreshToken(ctx context.Context, rdb *redis.Client, rawToken string) (*RefreshTokenData, error) {
	hashed := hashToken(rawToken)
	key := refreshTokenKey(hashed)

	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("refresh token not found")
		}
		return nil, fmt.Errorf("refresh token lookup failed: %w", err)
	}

	var data RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("refresh token decode failed: %w", err)
	}

	if time.Now().Unix() > data.ExpiresAt {
		// Clean up expired token
		_ = rdb.Del(ctx, key).Err()
		return nil, fmt.Errorf("refresh token expired")
	}
	return &data, nil
}

// rotateRefreshToken validates an existing refresh token, deletes it, and issues a new one.
func rotateRefreshToken(ctx context.Context, rdb *redis.Client, rawToken string) (string, *RefreshTokenData, error) {
	data, err := validateRefreshToken(ctx, rdb, rawToken)
	if err != nil {
		return "", nil, err
	}

	// Delete old token
	_ = rdb.Del(ctx, refreshTokenKey(hashToken(rawToken))).Err()

	newToken, err := generateRefreshToken(ctx, rdb, data.UserID, data.RoleID)
	if err != nil {
		return "", nil, err
	}
	return newToken, data, nil
}

func refreshTokenKey(hashed string) string {
	return fmt.Sprintf("refresh_token:%s", hashed)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateAccessToken signs a short-lived JWT with a unique JTI for blacklist support.
func GenerateAccessToken(userID int64, roleID int) (string, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return "", err
	}

	now := time.Now()
	jti, err := randomToken()
	if err != nil {
		return "", err
	}
	claims := AccessClaims{
		UserID: userID,
		RoleID: roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			ID:        jti,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateAccessToken parses and validates an access token, and checks blacklist in Redis.
func ValidateAccessToken(ctx context.Context, rdb *redis.Client, tokenStr string) (*AccessClaims, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check blacklist by JTI
	if claims.ID != "" {
		key := blacklistKey(claims.ID)
		exists, err := rdb.Exists(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("blacklist lookup failed: %w", err)
		}
		if exists > 0 {
			return nil, fmt.Errorf("token is revoked")
		}
	}

	return claims, nil
}

// BlacklistAccessToken stores the token JTI in Redis until its expiration.
func BlacklistAccessToken(ctx context.Context, rdb *redis.Client, tokenStr string) error {
	secret, err := getJWTSecret()
	if err != nil {
		return err
	}

	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return err
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return fmt.Errorf("invalid token claims")
	}

	if claims.ID == "" || claims.ExpiresAt == nil {
		return fmt.Errorf("token missing jti or exp")
	}

	expiry := time.Until(claims.ExpiresAt.Time)
	if expiry <= 0 {
		return nil // already expired, nothing to store
	}

	key := blacklistKey(claims.ID)
	if err := rdb.Set(ctx, key, "revoked", expiry).Err(); err != nil {
		return fmt.Errorf("blacklist write failed: %w", err)
	}
	return nil
}

func blacklistKey(jti string) string {
	return fmt.Sprintf("%s%s", blacklistPrefix, jti)
}

// ParseRefreshToken validates a refresh token against Redis and returns its data.
func ParseRefreshToken(ctx context.Context, rdb *redis.Client, rawToken string) (*RefreshTokenData, error) {
	return validateRefreshToken(ctx, rdb, rawToken)
}

// deleteRefreshToken removes an opaque refresh token from Redis.
func deleteRefreshToken(ctx context.Context, rdb *redis.Client, rawToken string) error {
	hashed := hashToken(rawToken)
	key := refreshTokenKey(hashed)
	return rdb.Del(ctx, key).Err()
}
