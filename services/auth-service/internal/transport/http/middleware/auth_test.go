package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// ---- fakes ----

type fakeVerifier struct {
	claims auth.TokenClaims
	err    error
	calls  int
	gotTok string
}

func (f *fakeVerifier) VerifyAccessToken(token string) (auth.TokenClaims, error) {
	f.calls++
	f.gotTok = token
	return f.claims, f.err
}

type fakeUsers struct {
	ver   int64
	err   error
	calls int
	gotID string
}

func (u *fakeUsers) GetTokenVersion(ctx context.Context, userID string) (int64, error) {
	u.calls++
	u.gotID = userID
	return u.ver, u.err
}

type writeErrRecorder struct {
	calls int
	last  error
}

func (w *writeErrRecorder) fn(_ http.ResponseWriter, _ *http.Request, err error) {
	w.calls++
	w.last = err
}

// next handler checks context injection
type nextRecorder struct {
	calls   int
	gotUID  string
	gotRole string
}

func (n *nextRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n.calls++
	uid, _ := UserIDFromContext(r.Context())
	role, _ := RoleFromContext(r.Context())
	n.gotUID = uid
	n.gotRole = role
	w.WriteHeader(http.StatusOK)
}

// helper to run middleware around a handler
func runAuthMW(t *testing.T, verifier TokenVerifier, users UserVersionReader, writeErr WriteErrFunc, req *http.Request) (*httptest.ResponseRecorder, *writeErrRecorder, *nextRecorder) {
	t.Helper()

	rr := httptest.NewRecorder()
	we := &writeErrRecorder{}
	nx := &nextRecorder{}

	// if caller doesn't pass a writeErr, use recorder
	if writeErr == nil {
		writeErr = we.fn
	}

	h := Auth(verifier, users, writeErr)(nx)
	h.ServeHTTP(rr, req)

	return rr, we, nx
}

// ---- tests ----

func TestAuth_MissingAuthorizationHeader_ReturnsTokenMissing(t *testing.T) {
	v := &fakeVerifier{}
	u := &fakeUsers{}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	if !domain.Is(we.last, "token_missing") {
		t.Fatalf("expected token_missing, got %v", we.last)
	}
	if v.calls != 0 {
		t.Fatalf("verifier should not be called when header missing")
	}
}

func TestAuth_BadAuthorizationScheme_ReturnsTokenInvalid(t *testing.T) {
	v := &fakeVerifier{}
	u := &fakeUsers{}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Basic abc")

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	if !domain.Is(we.last, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", we.last)
	}
	if v.calls != 0 {
		t.Fatalf("verifier should not be called on bad scheme")
	}
}

func TestAuth_BearerButEmptyToken_ReturnsTokenInvalid(t *testing.T) {
	v := &fakeVerifier{}
	u := &fakeUsers{}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer   ")

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	if !domain.Is(we.last, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", we.last)
	}
	if v.calls != 0 {
		t.Fatalf("verifier should not be called when raw token empty")
	}
}

func TestAuth_VerifierReturnsError_PropagatesToWriteErr(t *testing.T) {
	// If verifier returns domain error, middleware should pass through 그대로.
	v := &fakeVerifier{err: domain.ErrTokenExpired()}
	u := &fakeUsers{}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer abc")

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	if !domain.Is(we.last, "token_expired") {
		t.Fatalf("expected token_expired, got %v", we.last)
	}
	if v.calls != 1 || v.gotTok != "abc" {
		t.Fatalf("expected verifier called with token=abc, calls=%d gotTok=%q", v.calls, v.gotTok)
	}
}

func TestAuth_ClaimsMissingUserID_ReturnsTokenInvalid(t *testing.T) {
	v := &fakeVerifier{
		claims: auth.TokenClaims{
			UserID: "   ", // empty after trim
			Role:   "user",
			Ver:    0,
		},
	}
	u := &fakeUsers{}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer abc")

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	if !domain.Is(we.last, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", we.last)
	}
	// users.GetTokenVersion should NOT be called since claim invalid
	if u.calls != 0 {
		t.Fatalf("expected users not called, got %d", u.calls)
	}
}

func TestAuth_UsersNil_SkipsVersionCheck_AndInjectsContext(t *testing.T) {
	v := &fakeVerifier{
		claims: auth.TokenClaims{
			UserID: "u-1",
			Role:   "user",
			Ver:    5,
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer tok")

	_, we, nx := runAuthMW(t, v, nil, nil, req)

	if we.calls != 0 {
		t.Fatalf("expected writeErr not called, got %d (%v)", we.calls, we.last)
	}
	if nx.calls != 1 {
		t.Fatalf("expected next called once, got %d", nx.calls)
	}
	if nx.gotUID != "u-1" || nx.gotRole != "user" {
		t.Fatalf("expected ctx uid=u-1 role=user, got uid=%q role=%q", nx.gotUID, nx.gotRole)
	}
}

func TestAuth_UsersGetTokenVersionError_ReturnsThatError(t *testing.T) {
	v := &fakeVerifier{
		claims: auth.TokenClaims{
			UserID: "u-1",
			Role:   "user",
			Ver:    5,
		},
	}
	u := &fakeUsers{err: domain.ErrDBUnavailable(errors.New("db down"))}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer tok")

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	// ensure it's the same domain code (db_unavailable)
	if !domain.Is(we.last, "db_unavailable") {
		t.Fatalf("expected db_unavailable, got %v", we.last)
	}
	if u.calls != 1 || u.gotID != "u-1" {
		t.Fatalf("expected users called once with u-1, calls=%d gotID=%q", u.calls, u.gotID)
	}
}

func TestAuth_TokenRevoked_WhenClaimsVerLessThanDB_ReturnsTokenInvalid(t *testing.T) {
	v := &fakeVerifier{
		claims: auth.TokenClaims{
			UserID: "u-1",
			Role:   "user",
			Ver:    1,
		},
	}
	u := &fakeUsers{ver: 2} // current higher => revoked

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer tok")

	_, we, nx := runAuthMW(t, v, u, nil, req)

	if nx.calls != 0 {
		t.Fatalf("expected next not called")
	}
	if we.calls != 1 {
		t.Fatalf("expected writeErr called once, got %d", we.calls)
	}
	if !domain.Is(we.last, "token_invalid") {
		t.Fatalf("expected token_invalid, got %v", we.last)
	}
}

func TestAuth_TokenNotRevoked_WhenClaimsVerEqualOrGreater_InjectsContext(t *testing.T) {
	cases := []struct {
		name       string
		claimsVer  int64
		currentVer int64
	}{
		{"equal", 3, 3},
		{"greater", 4, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &fakeVerifier{
				claims: auth.TokenClaims{
					UserID: "u-1",
					Role:   "admin",
					Ver:    tc.claimsVer,
				},
			}
			u := &fakeUsers{ver: tc.currentVer}

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("Authorization", "Bearer tok")

			_, we, nx := runAuthMW(t, v, u, nil, req)

			if we.calls != 0 {
				t.Fatalf("expected writeErr not called, got %d (%v)", we.calls, we.last)
			}
			if nx.calls != 1 {
				t.Fatalf("expected next called once, got %d", nx.calls)
			}
			if nx.gotUID != "u-1" || nx.gotRole != "admin" {
				t.Fatalf("expected ctx uid=u-1 role=admin, got uid=%q role=%q", nx.gotUID, nx.gotRole)
			}
		})
	}
}
