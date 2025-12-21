package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

type OneTimeTokenStore struct {
	rdb    *goredis.Client
	prefix string // e.g. "ott:"
}

func NewOneTimeTokenStore(c *Client) *OneTimeTokenStore {
	var rdb *goredis.Client
	if c != nil {
		rdb = c.rdb
	}
	return &OneTimeTokenStore{
		rdb:    rdb,
		prefix: "ott:",
	}
}

func (s *OneTimeTokenStore) Save(ctx context.Context, kind auth.OneTimeTokenKind, token string, userID string, ttl time.Duration) error {
	token = strings.TrimSpace(token)
	userID = strings.TrimSpace(userID)
	if token == "" {
		return domain.ErrMissingField("token")
	}
	if userID == "" {
		return domain.ErrMissingField("user_id")
	}
	if s.rdb == nil {
		return errors.New("redis one-time-token store not configured")
	}
	if ttl <= 0 {
		return domain.ErrMissingField("ttl")
	}

	key := s.key(kind, token)
	// overwrite is fine (new request generates new token anyway)
	return s.rdb.Set(ctx, key, userID, ttl).Err()
}

func (s *OneTimeTokenStore) Consume(ctx context.Context, kind auth.OneTimeTokenKind, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", domain.ErrMissingField("token")
	}
	if s.rdb == nil {
		return "", errors.New("redis one-time-token store not configured")
	}

	key := s.key(kind, token)

	// Atomic GET + DEL
	const lua = `
local v = redis.call("GET", KEYS[1])
if not v then
  return nil
end
redis.call("DEL", KEYS[1])
return v
`
	res, err := s.rdb.Eval(ctx, lua, []string{key}).Result()
	if err != nil {
		return "", fmt.Errorf("ott consume: %w", err)
	}
	if res == nil {
		// token not found/expired/consumed
		return "", domain.ErrTokenInvalid()
	}

	uid, ok := res.(string)
	if !ok || strings.TrimSpace(uid) == "" {
		return "", domain.ErrTokenInvalid()
	}

	return uid, nil
}

func (s *OneTimeTokenStore) Peek(ctx context.Context, kind auth.OneTimeTokenKind, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", domain.ErrMissingField("token")
	}
	if s.rdb == nil {
		return "", errors.New("redis one-time-token store not configured")
	}

	key := s.key(kind, token)

	uid, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", domain.ErrTokenInvalid()
		}
		return "", fmt.Errorf("ott peek: %w", err)
	}

	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "", domain.ErrTokenInvalid()
	}
	return uid, nil
}

func (s *OneTimeTokenStore) key(kind auth.OneTimeTokenKind, token string) string {
	// kind is controlled constant ("verify_email"/"password_reset")
	return s.prefix + string(kind) + ":" + token
}
