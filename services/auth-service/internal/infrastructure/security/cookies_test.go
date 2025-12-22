package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetRefreshToken_SetsCookieAttributes(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	SetRefreshToken(rr, "tok123", 10*time.Minute, true)

	res := rr.Result()
	defer res.Body.Close()

	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected cookie set")
	}

	var c *http.Cookie
	for _, ck := range cookies {
		if ck.Name == RefreshCookieName {
			c = ck
			break
		}
	}
	if c == nil {
		t.Fatalf("expected %s cookie", RefreshCookieName)
	}

	if c.Value != "tok123" {
		t.Fatalf("expected value tok123, got %q", c.Value)
	}
	if c.Path != "/auth/v1" {
		t.Fatalf("expected path /auth/v1, got %q", c.Path)
	}
	if !c.HttpOnly {
		t.Fatalf("expected HttpOnly=true")
	}
	if !c.Secure {
		t.Fatalf("expected Secure=true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=Lax, got %v", c.SameSite)
	}
	if c.MaxAge <= 0 {
		t.Fatalf("expected MaxAge > 0, got %d", c.MaxAge)
	}
}

func TestClearRefreshToken_ClearsCookie(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	ClearRefreshToken(rr, false)

	res := rr.Result()
	defer res.Body.Close()

	var c *http.Cookie
	for _, ck := range res.Cookies() {
		if ck.Name == RefreshCookieName {
			c = ck
			break
		}
	}
	if c == nil {
		t.Fatalf("expected %s cookie", RefreshCookieName)
	}

	if c.Value != "" {
		t.Fatalf("expected empty value, got %q", c.Value)
	}
	if c.Path != "/auth/v1" {
		t.Fatalf("expected path /auth/v1, got %q", c.Path)
	}
	if c.MaxAge != -1 {
		t.Fatalf("expected MaxAge=-1, got %d", c.MaxAge)
	}
	if c.HttpOnly != true {
		t.Fatalf("expected HttpOnly=true")
	}
	if c.Secure != false {
		t.Fatalf("expected Secure=false")
	}
}

func TestReadRefreshToken_ReadsFromRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "http://example.com/auth/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "abc"})

	v, err := ReadRefreshToken(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v != "abc" {
		t.Fatalf("expected abc, got %q", v)
	}
}

func TestReadRefreshToken_Missing_ReturnsError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "http://example.com/auth/v1/me", nil)

	_, err := ReadRefreshToken(req)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
