package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// RedisSessionStore implements auth.SessionStore using Redis with per-user versioning:
// - Refresh token is opaque (random).
// - Redis stores: rt:<token> -> "<uid>:<ver>" with TTL
// - Redis stores: rtver:<uid> -> <ver> (integer, no TTL by default)
// - RevokeAll increments rtver:<uid>
// - Validation checks token's ver == current rtver:<uid>
type RedisSessionStore struct {
	rdb *goredis.Client

	rtPrefix    string
	rtverPrefix string

	// optional hardening knobs
	tokenBytes int // entropy bytes for opaque token
}

func NewRedisSessionStore(c *Client) *RedisSessionStore {
	var rdb *goredis.Client
	if c != nil {
		rdb = c.rdb
	}
	return &RedisSessionStore{
		rdb:         rdb,
		rtPrefix:    "rt:",
		rtverPrefix: "rtver:",
		tokenBytes:  32, // 256-bit
	}
}

func (s *RedisSessionStore) CreateRefreshToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", domain.ErrMissingField("user_id")
	}
	if s.rdb == nil {
		// If you prefer fail-closed, return an unavailable error here.
		return "", errors.New("redis session store not configured")
	}

	ver, err := s.getUserRTVer(ctx, userID)
	if err != nil {
		return "", err
	}

	token, err := s.newOpaqueToken()
	if err != nil {
		return "", err
	}

	val := fmt.Sprintf("%s:%d", userID, ver)
	if err := s.rdb.Set(ctx, s.rtPrefix+token, val, ttl).Err(); err != nil {
		return "", err
	}

	return token, nil
}

func (s *RedisSessionStore) RotateRefreshToken(ctx context.Context, oldToken string, ttl time.Duration) (string, error) {
	oldToken = strings.TrimSpace(oldToken)
	if oldToken == "" {
		return "", domain.ErrRefreshTokenInvalid()
	}
	if s.rdb == nil {
		return "", errors.New("redis session store not configured")
	}

	newToken, err := s.newOpaqueToken()
	if err != nil {
		return "", err
	}

	// Atomic "move": GET old -> DEL old -> SET new with TTL
	// Returns the old value (uid:ver) if existed, otherwise nil.
	const lua = `
local v = redis.call("GET", KEYS[1])
if not v then
  return nil
end
redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[2], v, "PX", ARGV[1])
return v
`
	ttlms := ttl.Milliseconds()
	if ttlms <= 0 {
		ttlms = int64((7 * 24 * time.Hour).Milliseconds())
	}

	res, err := s.rdb.Eval(ctx, lua, []string{s.rtPrefix + oldToken, s.rtPrefix + newToken}, ttlms).Result()
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", domain.ErrRefreshTokenInvalid()
	}

	val, ok := res.(string)
	if !ok || strings.TrimSpace(val) == "" {
		return "", domain.ErrRefreshTokenInvalid()
	}

	uid, tokVer, err := parseUIDVer(val)
	if err != nil {
		return "", domain.ErrRefreshTokenInvalid()
	}

	// Check user current version; if revoked in-between, treat as invalid.
	curVer, err := s.getUserRTVer(ctx, uid)
	if err != nil {
		return "", err
	}
	if tokVer != curVer {
		// token was from an older generation; invalidate the new token too (best effort)
		_ = s.rdb.Del(ctx, s.rtPrefix+newToken).Err()
		return "", domain.ErrRefreshTokenInvalid()
	}

	return newToken, nil
}

func (s *RedisSessionStore) RevokeRefreshToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		// idempotent
		return nil
	}
	if s.rdb == nil {
		return errors.New("redis session store not configured")
	}
	_ = s.rdb.Del(ctx, s.rtPrefix+token).Err()
	return nil
}

func (s *RedisSessionStore) RevokeAll(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return domain.ErrMissingField("user_id")
	}
	if s.rdb == nil {
		return errors.New("redis session store not configured")
	}
	// bump version; all existing tokens with old ver become invalid
	if err := s.rdb.Incr(ctx, s.rtverPrefix+userID).Err(); err != nil {
		return err
	}
	return nil
}

func (s *RedisSessionStore) GetUserIDByRefreshToken(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", domain.ErrRefreshTokenInvalid()
	}
	if s.rdb == nil {
		return "", errors.New("redis session store not configured")
	}

	val, err := s.rdb.Get(ctx, s.rtPrefix+token).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", domain.ErrRefreshTokenInvalid()
		}
		return "", err
	}

	uid, tokVer, err := parseUIDVer(val)
	if err != nil {
		return "", domain.ErrRefreshTokenInvalid()
	}

	curVer, err := s.getUserRTVer(ctx, uid)
	if err != nil {
		return "", err
	}

	if tokVer != curVer {
		return "", domain.ErrRefreshTokenInvalid()
	}

	return uid, nil
}

// ---- helpers ----

func (s *RedisSessionStore) getUserRTVer(ctx context.Context, userID string) (int64, error) {
	key := s.rtverPrefix + userID

	v, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		n, perr := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if perr == nil {
			return n, nil
		}
		// fallthrough: treat parse error as 0 and repair
	} else if !errors.Is(err, goredis.Nil) {
		return 0, err
	}

	// default ver = 0; ensure it exists (SETNX keeps it stable)
	_ = s.rdb.SetNX(ctx, key, "0", 0).Err()
	return 0, nil
}

func parseUIDVer(s string) (uid string, ver int64, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("bad token val")
	}
	uid = strings.TrimSpace(parts[0])
	if uid == "" {
		return "", 0, fmt.Errorf("empty uid")
	}
	ver, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return "", 0, err
	}
	return uid, ver, nil
}

func (s *RedisSessionStore) newOpaqueToken() (string, error) {
	b := make([]byte, s.tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// URL-safe, no padding
	return base64.RawURLEncoding.EncodeToString(b), nil
}
