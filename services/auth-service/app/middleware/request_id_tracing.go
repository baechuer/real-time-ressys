package middleware

import (
	"context"
	"net/http"
	"strconv"

	applogger "github.com/baechuer/real-time-ressys/services/auth-service/app/logger"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// RequestIDTracing creates middleware that propagates request ID through context
// and adds it to logger for all downstream services
func RequestIDTracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get request ID from chi middleware (already set by middleware.RequestID)
			requestID := middleware.GetReqID(r.Context())

			if requestID == "" {
				// Fallback: generate one if not set
				requestID = strconv.FormatUint(middleware.NextRequestID(), 10)
			}

			// Add request ID to response header
			w.Header().Set("X-Request-ID", requestID)

			// Create logger with request ID
			logger := applogger.Logger.With().Str("request_id", requestID).Logger()

			// Add logger to context for downstream use
			ctx := logger.WithContext(r.Context())

			// Also add request ID to context for programmatic access
			ctx = context.WithValue(ctx, "request_id", requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestIDFromContext retrieves request ID from context
func GetRequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

// GetLoggerFromContext retrieves logger from context (with request ID if available)
func GetLoggerFromContext(ctx context.Context) zerolog.Logger {
	logger := zerolog.Ctx(ctx)
	if logger.GetLevel() == zerolog.Disabled {
		// Fallback to global logger if not in context
		return applogger.Logger
	}
	return *logger
}
