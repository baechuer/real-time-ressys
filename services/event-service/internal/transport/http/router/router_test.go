package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/handlers"
	authmw "github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRouter_Routing(t *testing.T) {
	// 1. Setup minimal dependencies
	// We use nil/empty values for dependencies because we only care about the HTTP status code
	// from the middleware, not the handler logic itself.
	auth := authmw.NewAuth("secret", "issuer")
	h := handlers.NewEventsHandler(nil, nil)
	z := handlers.NewHealthHandler()
	r := New(h, auth, z)

	t.Run("public_route_returns_not_401", func(t *testing.T) {
		// We expect 404 or 200, but NOT 401, because it's public.
		// (It returns 404 here because our handler is nil and would crash if called,
		// but the router allows the request through the auth layer).
		req := httptest.NewRequest("GET", "/event/v1/events", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		assert.NotEqual(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("protected_route_returns_401_without_token", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/event/v1/events", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("organizer_route_prefix_is_correct", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/event/v1/organizer/events", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
