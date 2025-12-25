//go:build integration
// +build integration

package cases

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/test/integration/infra"
	"github.com/baechuer/real-time-ressys/services/event-service/test/integration/infra/wait"
)

type Env struct {
	BaseURL   string
	DBURL     string
	JWTSecret string
	JWTIssuer string

	UserToken  string
	AdminToken string
}

func mustEnv(t *testing.T, k string) string {
	t.Helper()
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("missing env %s", k)
	}
	return v
}

func setup(t *testing.T) Env {
	t.Helper()

	e := Env{
		BaseURL:   mustEnv(t, "EVENT_BASE_URL"),
		DBURL:     mustEnv(t, "DATABASE_URL"),
		JWTSecret: mustEnv(t, "JWT_SECRET"),
		JWTIssuer: mustEnv(t, "JWT_ISSUER"),
	}

	// Wait for event-service readiness endpoint.
	// 你需要在 event-service 加一个 GET /healthz（强烈建议）
	if err := wait.HTTP200(e.BaseURL+"/healthz", 10*time.Second); err != nil {
		t.Fatalf("event-service not ready: %v", err)
	}

	// Reset DB
	db, err := infra.OpenDB(e.DBURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := infra.PingDB(db); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	if err := infra.ResetEvents(db); err != nil {
		t.Fatalf("reset events: %v", err)
	}

	// Tokens (固定 uid 就行)
	e.UserToken, err = infra.MakeToken(e.JWTSecret, e.JWTIssuer, "11111111-1111-1111-1111-111111111111", "user", 0, 15*time.Minute)
	if err != nil {
		t.Fatalf("make user token: %v", err)
	}
	e.AdminToken, err = infra.MakeToken(e.JWTSecret, e.JWTIssuer, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "admin", 0, 15*time.Minute)
	if err != nil {
		t.Fatalf("make admin token: %v", err)
	}

	return e
}

type Envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Meta    map[string]string `json:"meta"`
	} `json:"error,omitempty"`
}

func doJSON(t *testing.T, method, url, token string, body any) (int, Envelope) {
	t.Helper()

	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	var env Envelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp.StatusCode, env
}
