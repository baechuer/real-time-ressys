package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestAuthClient_GetEmail_SendsSecret(t *testing.T) {
	// Mock auth-service
	secretReceived := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secretReceived = r.Header.Get("X-Internal-Secret")
		if secretReceived != "my-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Valid response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user": {"id": "u1", "email": "test@example.com"}}`))
	}))
	defer server.Close()

	client := NewAuthClient(server.URL, "my-secret", zerolog.Nop())

	email, err := client.GetEmail(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", email)
	}

	if secretReceived != "my-secret" {
		t.Errorf("expected header X-Internal-Secret: my-secret, got %q", secretReceived)
	}
}

func TestAuthClient_GetEmail_NoSecret_FailsIfServerRequiresIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Secret") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Client configured without secret
	client := NewAuthClient(server.URL, "", zerolog.Nop())

	_, err := client.GetEmail(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
