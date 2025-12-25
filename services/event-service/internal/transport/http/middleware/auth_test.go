package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_Require(t *testing.T) {
	secret := "test-secret"
	issuer := "test-issuer"
	auth := NewAuth(secret, issuer)

	// Helper to generate a valid token
	generateToken := func(uid, role string, iss string, secret string, expired bool) string {
		exp := time.Now().Add(time.Hour)
		if expired {
			exp = time.Now().Add(-time.Hour)
		}
		claims := Claims{
			UserID: uid,
			Role:   role,
			Ver:    1,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    iss,
				ExpiresAt: jwt.NewNumericDate(exp),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		ss, _ := token.SignedString([]byte(secret))
		return ss
	}

	t.Run("valid_token_should_pass_and_set_context", func(t *testing.T) {
		token := generateToken("user-123", "admin", issuer, secret, false)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		// Final handler to check if context was set
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "user-123", UserID(r))
			assert.Equal(t, "admin", Role(r))
			w.WriteHeader(http.StatusOK)
		})

		auth.Require(nextHandler).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("expired_token_should_fail", func(t *testing.T) {
		token := generateToken("user-1", "user", issuer, secret, true)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("wrong_secret_should_fail", func(t *testing.T) {
		token := generateToken("user-1", "user", issuer, "wrong-secret", false)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestAccessLog(t *testing.T) {
	req := httptest.NewRequest("GET", "/test-path", nil)
	rr := httptest.NewRecorder()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello"))
	})

	AccessLog(nextHandler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "hello", rr.Body.String())
}
