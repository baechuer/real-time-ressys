package services

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
JWT services test cases:
1) GenerateAccessToken succeeds with valid secret
2) ValidateAccessToken rejects invalid signature
3) ValidateAccessToken rejects blacklisted JTI
4) BlacklistAccessToken stores JTI until expiry
5) generate/validate refresh token lifecycle (opaque)
6) ParseRefreshToken returns error on missing token
*/

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestGenerateAccessToken_Succeeds(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	token, err := GenerateAccessToken(10, 2)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestValidateAccessToken_InvalidSignature(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)

	// Warm secret cache with expected secret
	_, err := getJWTSecret()
	require.NoError(t, err)

	// Manually sign token with a different secret so signature won't verify
	badToken := jwt.NewWithClaims(jwt.SigningMethodHS256, AccessClaims{
		UserID: 1,
		RoleID: 1,
	})
	signed, err := badToken.SignedString([]byte("othersupersecret"))
	require.NoError(t, err)

	claims, err := ValidateAccessToken(context.Background(), rdb, signed)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestValidateAccessToken_Blacklisted(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)

	token, err := GenerateAccessToken(2, 3)
	require.NoError(t, err)

	err = BlacklistAccessToken(context.Background(), rdb, token)
	require.NoError(t, err)

	claims, err := ValidateAccessToken(context.Background(), rdb, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestBlacklistAccessToken_StoresJTI(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret")
	rdb := newTestRedis(t)

	token, err := GenerateAccessToken(3, 4)
	require.NoError(t, err)

	err = BlacklistAccessToken(context.Background(), rdb, token)
	require.NoError(t, err)

	// Ensure blacklist key exists
	claims, err := ValidateAccessToken(context.Background(), rdb, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestRefreshTokenLifecycle(t *testing.T) {
	t.Setenv("JWT_SECRET", "supersecret") // not used here but set for consistency
	rdb := newTestRedis(t)

	raw, err := generateRefreshToken(context.Background(), rdb, 5, 6)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	data, err := validateRefreshToken(context.Background(), rdb, raw)
	require.NoError(t, err)
	assert.Equal(t, int64(5), data.UserID)
	assert.Equal(t, 6, data.RoleID)

	newRaw, rotated, err := rotateRefreshToken(context.Background(), rdb, raw)
	require.NoError(t, err)
	assert.NotEqual(t, raw, newRaw)
	assert.Equal(t, data.UserID, rotated.UserID)

	// Old token should be gone
	_, err = validateRefreshToken(context.Background(), rdb, raw)
	assert.Error(t, err)
}

func TestValidateRefreshToken_Missing(t *testing.T) {
	rdb := newTestRedis(t)
	_, err := ParseRefreshToken(context.Background(), rdb, "non-existent")
	assert.Error(t, err)
}
