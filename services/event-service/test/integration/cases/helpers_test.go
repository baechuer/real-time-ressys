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

	// Clean DB first to ensure migrations apply cleanly
	if err := infra.WipeDB(db); err != nil {
		t.Fatalf("wipe db: %v", err)
	}

	// Apply Migrations
	if err := infra.ApplyMigrations(db, "../../../migrations"); err != nil {
		t.Fatalf("apply migrations: %v", err)
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

// ... 现有的 helpers_test.go 代码 ...

// createAndPublish 辅助函数：快速创建一个已发布的活动用于测试
func createAndPublish(t *testing.T, e Env, title, desc string) string {
	t.Helper()

	body := map[string]any{
		"title":       title,
		"description": desc,
		"city":        "Sydney",
		"category":    "test",
		"start_time":  time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		"end_time":    time.Now().UTC().Add(25 * time.Hour).Format(time.RFC3339),
		"capacity":    100,
	}

	code, env := doJSON(t, "POST", e.BaseURL+"/event/v1/events", e.UserToken, body)
	if code != 201 {
		t.Fatalf("create failed: %d", code)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("unmarshal created id failed: %v", err)
	}

	code, _ = doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/publish", e.UserToken, nil)
	if code != 200 {
		t.Fatalf("publish failed: %d", code)
	}

	return created.ID
}
