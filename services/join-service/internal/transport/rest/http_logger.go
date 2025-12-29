package rest

import (
	"net"
	"net/http"
	"time"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/pkg/logger"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *statusRecorder) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusRecorder) Write(p []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(p)
	rw.bytes += n
	return n, err
}

func HTTPLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			ip = host
		}

		logger.WithCtx(r.Context()).
			Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", ip).
			Int("status", rec.status).
			Int("bytes", rec.bytes).
			Dur("duration", time.Since(start)).
			Msg("http_request")
	})
}
