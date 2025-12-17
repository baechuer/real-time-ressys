package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Message consumption metrics
	messagesConsumedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "email_messages_consumed_total",
			Help: "Total number of messages consumed from RabbitMQ",
		},
		[]string{"queue", "type"},
	)

	// Email sending metrics
	emailsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "email_sent_total",
			Help: "Total number of emails sent successfully",
		},
		[]string{"type", "provider"},
	)

	emailsFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "email_failed_total",
			Help: "Total number of failed email sends",
		},
		[]string{"type", "provider", "error_type"},
	)

	emailSendDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "email_send_duration_seconds",
			Help:    "Email sending duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"type", "provider"},
	)

	// Retry metrics
	retryAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "email_retry_attempts_total",
			Help: "Total number of retry attempts",
		},
		[]string{"type"},
	)

	// DLQ metrics
	dlqMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "email_dlq_messages_total",
			Help: "Total number of messages sent to DLQ",
		},
		[]string{"type", "reason"},
	)

	// Processing metrics
	messageProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "email_message_processing_duration_seconds",
			Help:    "Message processing duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"type"},
	)

	// Idempotency metrics
	idempotencyHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "email_idempotency_hits_total",
			Help: "Total number of duplicate messages detected (idempotency hits)",
		},
	)

	idempotencyMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "email_idempotency_misses_total",
			Help: "Total number of new messages processed (idempotency misses)",
		},
	)

	// Worker pool metrics
	workerPoolJobsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "email_worker_pool_jobs_active",
			Help: "Number of active jobs in worker pool",
		},
	)

	workerPoolJobsQueued = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "email_worker_pool_jobs_queued",
			Help: "Number of queued jobs in worker pool",
		},
	)
)

// RecordMessageConsumed records a consumed message
func RecordMessageConsumed(queue, messageType string) {
	messagesConsumedTotal.WithLabelValues(queue, messageType).Inc()
}

// RecordEmailSent records a successfully sent email
func RecordEmailSent(emailType, provider string, duration time.Duration) {
	emailsSentTotal.WithLabelValues(emailType, provider).Inc()
	emailSendDuration.WithLabelValues(emailType, provider).Observe(duration.Seconds())
}

// RecordEmailFailed records a failed email send
func RecordEmailFailed(emailType, provider, errorType string) {
	emailsFailedTotal.WithLabelValues(emailType, provider, errorType).Inc()
}

// RecordRetryAttempt records a retry attempt
func RecordRetryAttempt(messageType string) {
	retryAttemptsTotal.WithLabelValues(messageType).Inc()
}

// RecordDLQMessage records a message sent to DLQ
func RecordDLQMessage(messageType, reason string) {
	dlqMessagesTotal.WithLabelValues(messageType, reason).Inc()
}

// RecordMessageProcessing records message processing duration
func RecordMessageProcessing(messageType string, duration time.Duration) {
	messageProcessingDuration.WithLabelValues(messageType).Observe(duration.Seconds())
}

// RecordIdempotencyHit records an idempotency hit (duplicate message)
func RecordIdempotencyHit() {
	idempotencyHitsTotal.Inc()
}

// RecordIdempotencyMiss records an idempotency miss (new message)
func RecordIdempotencyMiss() {
	idempotencyMissesTotal.Inc()
}

// SetWorkerPoolJobsActive sets the number of active jobs
func SetWorkerPoolJobsActive(count int) {
	workerPoolJobsActive.Set(float64(count))
}

// SetWorkerPoolJobsQueued sets the number of queued jobs
func SetWorkerPoolJobsQueued(count int) {
	workerPoolJobsQueued.Set(float64(count))
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

