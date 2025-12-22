package domain

import "testing"

func TestIsValidRole(t *testing.T) {
	cases := []struct {
		role string
		ok   bool
	}{
		{"user", true},
		{"moderator", true},
		{"admin", true},
		{"", false},
		{"root", false},
	}

	for _, c := range cases {
		if IsValidRole(c.role) != c.ok {
			t.Fatalf("unexpected IsValidRole(%q)", c.role)
		}
	}
}

func TestRoleRank(t *testing.T) {
	if RoleRank("user") >= RoleRank("moderator") {
		t.Fatalf("user should be lower than moderator")
	}
	if RoleRank("moderator") >= RoleRank("admin") {
		t.Fatalf("moderator should be lower than admin")
	}
	if RoleRank("invalid") != 0 {
		t.Fatalf("invalid role should rank 0")
	}
}
