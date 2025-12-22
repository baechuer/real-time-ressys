package auth

import (
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func requireErrCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code=%q, got nil", code)
	}
	if !domain.Is(err, code) {
		t.Fatalf("expected code=%q, got err=%v", code, err)
	}
}
