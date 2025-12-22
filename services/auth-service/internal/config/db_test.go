package config

import "testing"

func TestNewDB_EmptyDSN(t *testing.T) {
	_, err := NewDB("", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewDB_InvalidDSN(t *testing.T) {
	_, err := NewDB("postgres://invalid:5432/db", false)
	if err == nil {
		t.Fatal("expected ping failure")
	}
}

func TestNewDB_PingTimeoutOrRefused(t *testing.T) {
	// localhost:1 is almost guaranteed to refuse
	_, err := NewDB("postgres://user:pass@localhost:1/db", false)
	if err == nil {
		t.Fatal("expected ping failure")
	}
}
