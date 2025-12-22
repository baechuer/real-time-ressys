package domain

import "testing"

func TestUserStruct_DefaultZeroValues(t *testing.T) {
	var u User

	if u.Role != "" {
		t.Fatalf("expected empty role")
	}
	if u.Locked {
		t.Fatalf("expected Locked=false")
	}
	if u.EmailVerified {
		t.Fatalf("expected EmailVerified=false")
	}
}
