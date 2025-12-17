package metrics

import (
	"net/http"
	"testing"
	"time"
)

// These tests are lightweight sanity checks to ensure that
// metrics functions can be called without panicking.

func TestRecordMessageConsumed(t *testing.T) {
	RecordMessageConsumed("test-queue", "email_verification")
}

func TestRecordEmailSentAndFailed(t *testing.T) {
	duration := 150 * time.Millisecond
	RecordEmailSent("email_verification", "smtp", duration)
	RecordEmailFailed("email_verification", "smtp", "provider_error")
}

func TestRecordRetryAndDLQ(t *testing.T) {
	RecordRetryAttempt("email_verification")
	RecordDLQMessage("email_verification", "test_reason")
}

func TestRecordProcessingAndIdempotency(t *testing.T) {
	RecordMessageProcessing("email_verification", 200*time.Millisecond)
	RecordIdempotencyHit()
	RecordIdempotencyMiss()
}

func TestWorkerPoolMetrics(t *testing.T) {
	SetWorkerPoolJobsActive(5)
	SetWorkerPoolJobsQueued(10)
}

func TestMetricsHandler(t *testing.T) {
	h := MetricsHandler()
	if h == nil {
		t.Fatal("MetricsHandler returned nil")
	}

	// Basic type assertion
	if _, ok := h.(http.Handler); !ok {
		t.Fatal("MetricsHandler does not implement http.Handler")
	}
}


