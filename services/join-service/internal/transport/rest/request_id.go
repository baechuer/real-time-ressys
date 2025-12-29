package rest

import (
	"net/http"

	appCtx "github.com/baechuer/real-time-ressys/services/join-service/internal/pkg/context"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

// RequestID injects a request id into context and response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(requestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, rid)

		ctx := appCtx.WithRequestID(r.Context(), rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
