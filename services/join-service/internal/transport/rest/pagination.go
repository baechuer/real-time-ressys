package rest

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/domain"
	"github.com/google/uuid"
)

var errBadCursor = errors.New("bad cursor")

// cursor = base64url("RFC3339Nano|uuid")
func encodeCursor(c *domain.KeysetCursor) string {
	if c == nil {
		return ""
	}
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*domain.KeysetCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadCursor
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 2 {
		return nil, errBadCursor
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, errBadCursor
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, errBadCursor
	}
	return &domain.KeysetCursor{CreatedAt: t, ID: id}, nil
}
