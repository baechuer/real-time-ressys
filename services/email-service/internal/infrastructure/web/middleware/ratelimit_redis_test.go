package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog"
)

// ---- fake script (in-memory counter) ----

type fakeAllowScript struct {
	counts map[string]int
}

func newFakeAllowScript() *fakeAllowScript {
	return &fakeAllowScript{counts: map[string]int{}}
}

// IMPORTANT: return types as int64 to be maximally compatible with redigo redis.Scan
func (s *fakeAllowScript) Do(conn redis.Conn, key string, windowMS int64, limit int) ([]any, error) {
	_ = conn
	_ = windowMS // TTL/window correctness belongs to integration tests

	s.counts[key]++
	cur := s.counts[key]

	ok := int64(1)
	if limit > 0 && cur > limit {
		ok = 0
	}

	// Use []interface{} under the hood (alias of []any), elements are int64
	return []any{ok, int64(cur)}, nil
}

// ---- minimal pool that always returns a dummy conn ----

type dummyConn struct{}

func (c dummyConn) Close() error                   { return nil }
func (c dummyConn) Err() error                     { return nil }
func (c dummyConn) Do(string, ...any) (any, error) { return nil, nil }
func (c dummyConn) Send(string, ...any) error      { return nil }
func (c dummyConn) Flush() error                   { return nil }
func (c dummyConn) Receive() (any, error)          { return nil, nil }

func newDummyPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:   1,
		MaxActive: 1,
		Wait:      true,
		Dial:      func() (redis.Conn, error) { return dummyConn{}, nil },
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			_ = ctx
			return dummyConn{}, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_ = c
			_ = t
			return nil
		},
	}
}

func newJSONReq(body string) *http.Request {
	r := httptest.NewRequest("POST", "http://example.com/api/verify", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "1.2.3.4:12345"
	return r
}

func readAllBody(t *testing.T, r *http.Request) string {
	t.Helper()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, _ := json.Marshal(v)
	return string(b)
}

func attachErrHook(rl *RedisRateLimiter) func() error {
	var got error
	rl.onError = func(e error) { got = e }
	return func() error { return got }
}

func TestRateLimiter_PerIP_LimitsAfterN(t *testing.T) {
	pool := newDummyPool()
	script := newFakeAllowScript()

	rl := NewRedisRateLimiter(pool, RedisRateLimitConfig{
		Enabled:     true,
		IPLimit:     2,
		IPWindow:    10 * time.Second,
		TokenLimit:  0,
		TokenWindow: 0,
		KeyPrefix:   "rl:test",
	}, zerolog.Nop())

	// inject fake script + hook
	rl.script = script
	getErr := attachErrHook(rl)

	called := 0
	h := rl.WrapJSONTokenEndpoint("api_verify", func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(200)
	})

	// 1,2 ok
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h(w, newJSONReq(mustJSON(t, map[string]string{"token": "abc"})))

		if w.Code == 502 {
			t.Fatalf("#%d got 502; rl error: %v", i+1, getErr())
		}
		if w.Code != 200 {
			t.Fatalf("#%d expected 200 got %d", i+1, w.Code)
		}
	}

	// 3 -> 429
	{
		w := httptest.NewRecorder()
		h(w, newJSONReq(mustJSON(t, map[string]string{"token": "abc"})))

		if w.Code == 502 {
			t.Fatalf("3rd got 502; rl error: %v", getErr())
		}
		if w.Code != 429 {
			t.Fatalf("expected 429 got %d", w.Code)
		}
		if w.Header().Get("Retry-After") == "" {
			t.Fatalf("expected Retry-After header")
		}
	}

	if called != 2 {
		t.Fatalf("expected handler called 2 times, got %d", called)
	}
}

func TestRateLimiter_PerToken_IndependentAcrossTokens(t *testing.T) {
	pool := newDummyPool()
	script := newFakeAllowScript()

	rl := NewRedisRateLimiter(pool, RedisRateLimitConfig{
		Enabled:     true,
		IPLimit:     0,
		IPWindow:    0,
		TokenLimit:  2,
		TokenWindow: 30 * time.Second,
		KeyPrefix:   "rl:test",
	}, zerolog.Nop())

	rl.script = script
	getErr := attachErrHook(rl)

	h := rl.WrapJSONTokenEndpoint("api_verify", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// token A: two ok, third 429
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h(w, newJSONReq(mustJSON(t, map[string]string{"token": "A"})))

		if w.Code == 502 {
			t.Fatalf("A #%d got 502; rl error: %v", i+1, getErr())
		}
		if i < 2 && w.Code != 200 {
			t.Fatalf("A #%d expected 200 got %d", i+1, w.Code)
		}
		if i == 2 && w.Code != 429 {
			t.Fatalf("A #3 expected 429 got %d", w.Code)
		}
	}

	// token B still ok
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h(w, newJSONReq(mustJSON(t, map[string]string{"token": "B"})))

		if w.Code == 502 {
			t.Fatalf("B #%d got 502; rl error: %v", i+1, getErr())
		}
		if w.Code != 200 {
			t.Fatalf("B #%d expected 200 got %d", i+1, w.Code)
		}
	}
}

func TestRateLimiter_RestoresRequestBodyForDownstream(t *testing.T) {
	pool := newDummyPool()
	script := newFakeAllowScript()

	rl := NewRedisRateLimiter(pool, RedisRateLimitConfig{
		Enabled:     true,
		IPLimit:     10,
		IPWindow:    time.Minute,
		TokenLimit:  10,
		TokenWindow: time.Minute,
		KeyPrefix:   "rl:test",
	}, zerolog.Nop())

	rl.script = script
	getErr := attachErrHook(rl)

	h := rl.WrapJSONTokenEndpoint("api_verify", func(w http.ResponseWriter, r *http.Request) {
		b := readAllBody(t, r)
		if !strings.Contains(b, `"token":"abc"`) {
			t.Fatalf("downstream body not restored: %q", b)
		}
		w.WriteHeader(200)
	})

	w := httptest.NewRecorder()
	h(w, newJSONReq(`{"token":"abc"}`))

	if w.Code == 502 {
		t.Fatalf("got 502; rl error: %v", getErr())
	}
	if w.Code != 200 {
		t.Fatalf("expected 200 got %d", w.Code)
	}
}

func TestRateLimiter_BadJSON_Returns400(t *testing.T) {
	pool := newDummyPool()
	script := newFakeAllowScript()

	rl := NewRedisRateLimiter(pool, RedisRateLimitConfig{
		Enabled:     true,
		IPLimit:     10,
		IPWindow:    time.Minute,
		TokenLimit:  10,
		TokenWindow: time.Minute,
		KeyPrefix:   "rl:test",
	}, zerolog.Nop())

	rl.script = script
	_ = attachErrHook(rl) // not needed here, but safe

	called := 0
	h := rl.WrapJSONTokenEndpoint("api_verify", func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(200)
	})

	w := httptest.NewRecorder()
	h(w, newJSONReq(`{not-json`))

	if w.Code != 400 {
		t.Fatalf("expected 400 got %d", w.Code)
	}
	if called != 0 {
		t.Fatalf("expected handler not called, got %d", called)
	}
}

func TestRateLimiter_Disabled_PassesThrough(t *testing.T) {
	pool := newDummyPool()

	rl := NewRedisRateLimiter(pool, RedisRateLimitConfig{
		Enabled: false,
	}, zerolog.Nop())

	called := 0
	h := rl.WrapJSONTokenEndpoint("api_verify", func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(200)
	})

	w := httptest.NewRecorder()
	h(w, newJSONReq(`{"token":"abc"}`))

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if called != 1 {
		t.Fatalf("expected handler called once, got %d", called)
	}
}
