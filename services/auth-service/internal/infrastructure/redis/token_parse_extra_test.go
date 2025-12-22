package redis

import "testing"

func TestParseUIDVer_UidEmpty(t *testing.T) {
	t.Parallel()

	_, _, err := parseUIDVer(":1")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestParseUIDVer_VerNonNumeric(t *testing.T) {
	t.Parallel()

	_, _, err := parseUIDVer("u:abc")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestParseUIDVer_TooManyParts(t *testing.T) {
	t.Parallel()

	_, _, err := parseUIDVer("u:1:2")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestParseUIDVer_TooFewParts(t *testing.T) {
	t.Parallel()

	_, _, err := parseUIDVer("u")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
