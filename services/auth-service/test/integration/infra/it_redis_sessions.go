//go:build integration

package infra

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

type itRedisSessionStore struct {
	rdb         *goredis.Client
	rtPrefix    string
	rtverPrefix string
	tokenBytes  int
}

func NewITRedisSessionStore(rdb *goredis.Client) *itRedisSessionStore {
	return &itRedisSessionStore{
		rdb:         rdb,
		rtPrefix:    "rt:",
		rtverPrefix: "rtver:",
		tokenBytes:  32,
	}
}

func (s *itRedisSessionStore) CreateRefreshToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", domain.ErrMissingField("user_id")
	}
	ver, err := s.getUserRTVer(ctx, userID)
	if err != nil {
		return "", err
	}

	tok, err := s.newOpaqueToken()
	if err != nil {
		return "", err
	}

	val := fmt.Sprintf("%s:%d", userID, ver)
	if err := s.rdb.Set(ctx, s.rtPrefix+tok, val, ttl).Err(); err != nil {
		return "", err
	}
	return tok, nil
}

func (s *itRedisSessionStore) RotateRefreshToken(ctx context.Context, oldToken string, ttl time.Duration) (string, error) {
	oldToken = strings.TrimSpace(oldToken)
	if oldToken == "" {
		return "", domain.ErrRefreshTokenInvalid()
	}

	newTok, err := s.newOpaqueToken()
	if err != nil {
		return "", err
	}

	const lua = `
local v = redis.call("GET", KEYS[1])
if not v then return nil end
redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[2], v, "PX", ARGV[1])
return v
`
	ttlms := ttl.Milliseconds()
	if ttlms <= 0 {
		ttlms = int64((7 * 24 * time.Hour).Milliseconds())
	}

	res, err := s.rdb.Eval(ctx, lua, []string{s.rtPrefix + oldToken, s.rtPrefix + newTok}, ttlms).Result()
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

	curVer, err := s.getUserRTVer(ctx, uid)
	if err != nil {
		return "", err
	}
	if tokVer != curVer {
		_ = s.rdb.Del(ctx, s.rtPrefix+newTok).Err()
		return "", domain.ErrRefreshTokenInvalid()
	}
	return newTok, nil
}

func (s *itRedisSessionStore) RevokeRefreshToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	_ = s.rdb.Del(ctx, s.rtPrefix+token).Err()
	return nil
}

func (s *itRedisSessionStore) RevokeAll(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return domain.ErrMissingField("user_id")
	}
	return s.rdb.Incr(ctx, s.rtverPrefix+userID).Err()
}

func (s *itRedisSessionStore) GetUserIDByRefreshToken(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", domain.ErrRefreshTokenInvalid()
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

func (s *itRedisSessionStore) getUserRTVer(ctx context.Context, userID string) (int64, error) {
	key := s.rtverPrefix + userID
	v, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		n, perr := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if perr == nil {
			return n, nil
		}
	} else if !errors.Is(err, goredis.Nil) {
		return 0, err
	}
	_ = s.rdb.SetNX(ctx, key, "0", 0).Err()
	return 0, nil
}

func parseUIDVer(s string) (string, int64, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("bad token val")
	}
	uid := strings.TrimSpace(parts[0])
	if uid == "" {
		return "", 0, fmt.Errorf("empty uid")
	}
	ver, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return "", 0, err
	}
	return uid, ver, nil
}

func (s *itRedisSessionStore) newOpaqueToken() (string, error) {
	b := make([]byte, s.tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
