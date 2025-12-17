package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "endpoint"},
	)

	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "endpoint"},
	)

	// Business logic metrics
	authRegistrationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_registrations_total",
			Help: "Total number of user registrations",
		},
	)

	authLoginsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_logins_total",
			Help: "Total number of user logins",
		},
	)

	authLoginsFailed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_logins_failed_total",
			Help: "Total number of failed login attempts",
		},
	)

	authTokenRefreshesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_token_refreshes_total",
			Help: "Total number of token refreshes",
		},
	)

	authEmailVerificationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_email_verifications_total",
			Help: "Total number of email verifications",
		},
	)

	authPasswordResetsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "auth_password_resets_total",
			Help: "Total number of password resets",
		},
	)

	// Dependency health metrics
	dependencyHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dependency_health",
			Help: "Health status of dependencies (1 = healthy, 0 = unhealthy)",
		},
		[]string{"dependency"},
	)
)

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	status := strconv.Itoa(statusCode)
	httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	httpRequestDuration.WithLabelValues(method, endpoint, status).Observe(duration.Seconds())
	httpRequestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
	httpResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
}

// RecordRegistration increments registration counter
func RecordRegistration() {
	authRegistrationsTotal.Inc()
}

// RecordLogin increments login counter
func RecordLogin() {
	authLoginsTotal.Inc()
}

// RecordLoginFailed increments failed login counter
func RecordLoginFailed() {
	authLoginsFailed.Inc()
}

// RecordTokenRefresh increments token refresh counter
func RecordTokenRefresh() {
	authTokenRefreshesTotal.Inc()
}

// RecordEmailVerification increments email verification counter
func RecordEmailVerification() {
	authEmailVerificationsTotal.Inc()
}

// RecordPasswordReset increments password reset counter
func RecordPasswordReset() {
	authPasswordResetsTotal.Inc()
}

// SetDependencyHealth sets the health status of a dependency
func SetDependencyHealth(dependency string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	dependencyHealth.WithLabelValues(dependency).Set(value)
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

