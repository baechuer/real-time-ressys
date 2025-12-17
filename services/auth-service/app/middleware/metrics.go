package middleware

import (
	"net/http"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Metrics creates middleware that records Prometheus metrics for HTTP requests
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code and size
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Get request size
			requestSize := r.ContentLength
			if requestSize < 0 {
				requestSize = 0
			}

			// Process request
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start)

			// Get endpoint (route pattern)
			routePattern := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && len(rctx.RoutePatterns) > 0 {
				routePattern = rctx.RoutePatterns[len(rctx.RoutePatterns)-1]
			}

			// Record metrics
			metrics.RecordHTTPRequest(
				r.Method,
				routePattern,
				ww.Status(),
				duration,
				requestSize,
				int64(ww.BytesWritten()),
			)
		})
	}
}
