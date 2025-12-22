package http_handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
)

// mustJSONBody marshals v to JSON and returns an io.Reader for request body.
func mustJSONBody(t *testing.T, v any) io.Reader {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// mustReadJSON decodes JSON from r into out.
// It tries to decode directly into out.
// If that fails, it tries {"data": <out>} wrapper (common in APIs).
func mustReadJSON(t *testing.T, r io.Reader, out any) {
	t.Helper()

	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// 1) Try decode directly
	if err := json.Unmarshal(raw, out); err == nil {
		return
	}

	// 2) Try decode with {"data": ...} wrapper
	wrapped := struct {
		Data json.RawMessage `json:"data"`
	}{}
	if err := json.Unmarshal(raw, &wrapped); err != nil || len(wrapped.Data) == 0 {
		t.Fatalf("decode json failed; body=%s", string(raw))
	}

	if err := json.Unmarshal(wrapped.Data, out); err != nil {
		t.Fatalf("decode wrapped.data failed; body=%s err=%v", string(raw), err)
	}
}

// readCookie finds cookie by name from response headers.
func readCookie(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// withUserCtx injects user_id + role into request context (your middleware keys).
func withUserCtx(req *http.Request, userID, role string) *http.Request {
	ctx := middleware.WithUser(req.Context(), userID, role)
	return req.WithContext(ctx)
}

// withURLParam injects chi URL param (e.g. /users/{id}) into request context.
func withURLParam(req *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)

	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}
func mustExtractUserIDFromRegisterBody(t *testing.T, body *bytes.Buffer) string {
	t.Helper()

	var raw any
	if err := json.Unmarshal(body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal register response: %v; body=%s", err, body.String())
	}

	// helper: safe map get
	asMap := func(v any) (map[string]any, bool) {
		m, ok := v.(map[string]any)
		return m, ok
	}

	// try paths:
	// 1) root.user.id
	if root, ok := asMap(raw); ok {
		// root.user.id
		if u, ok := asMap(root["user"]); ok {
			if id, _ := u["id"].(string); id != "" {
				return id
			}
		}
		// root.data.user.id
		if d, ok := asMap(root["data"]); ok {
			if u, ok := asMap(d["user"]); ok {
				if id, _ := u["id"].(string); id != "" {
					return id
				}
			}
		}
	}

	t.Fatalf("expected user.id in register response; body=%s", body.String())
	return ""
}
