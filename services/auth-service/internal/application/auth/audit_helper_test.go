package auth

import (
	"errors"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestDomainCode(t *testing.T) {
	t.Parallel()

	if got := domainCode(nil); got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}

	derr := domain.ErrForbidden()
	if got := domainCode(derr); got == "" || got == "non_domain_error" {
		t.Fatalf("expected domain error code, got %q", got)
	}

	if got := domainCode(errors.New("x")); got != "non_domain_error" {
		t.Fatalf("expected non_domain_error, got %q", got)
	}
}
