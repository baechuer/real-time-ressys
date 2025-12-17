package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"` // "healthy" or "unhealthy"
	Timestamp string                 `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
	Uptime    string                 `json:"uptime,omitempty"`
}

// CheckResult represents the result of a dependency health check
type CheckResult struct {
	Status       string `json:"status"` // "up" or "down"
	ResponseTime string `json:"response_time,omitempty"`
	Error        string `json:"error,omitempty"`
}

var startTime = time.Now()

// Handler handles health check requests
type Handler struct {
	rabbitMQConn *amqp.Connection
	rabbitMQCh   *amqp.Channel
	redisClient  *redis.Client
	emailSender  EmailSenderChecker
}

// EmailSenderChecker interface for checking email provider health
type EmailSenderChecker interface {
	CheckHealth(ctx context.Context) error
}

// NewHandler creates a new health check handler
func NewHandler(
	rabbitMQConn *amqp.Connection,
	rabbitMQCh *amqp.Channel,
	redisClient *redis.Client,
	emailSender EmailSenderChecker,
) *Handler {
	return &Handler{
		rabbitMQConn: rabbitMQConn,
		rabbitMQCh:   rabbitMQCh,
		redisClient:  redisClient,
		emailSender:  emailSender,
	}
}

// HealthCheck handles the main health check endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]CheckResult)
	overallStatus := "healthy"

	// Check RabbitMQ
	rabbitCheck := h.checkRabbitMQ(ctx)
	checks["rabbitmq"] = rabbitCheck
	if rabbitCheck.Status != "up" {
		overallStatus = "unhealthy"
	}

	// Check Redis
	redisCheck := h.checkRedis(ctx)
	checks["redis"] = redisCheck
	if redisCheck.Status != "up" {
		overallStatus = "unhealthy"
	}

	// Check Email Provider (if available)
	if h.emailSender != nil {
		emailCheck := h.checkEmailProvider(ctx)
		checks["email"] = emailCheck
		if emailCheck.Status != "up" {
			// Email provider failure doesn't make service unhealthy (can retry later)
			// But we still report it
		}
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
		Uptime:    time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")

	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// HealthCheckRabbitMQ handles RabbitMQ-specific health check
func (h *Handler) HealthCheckRabbitMQ(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	check := h.checkRabbitMQ(ctx)
	w.Header().Set("Content-Type", "application/json")

	statusCode := http.StatusOK
	if check.Status != "up" {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(check)
}

// HealthCheckRedis handles Redis-specific health check
func (h *Handler) HealthCheckRedis(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	check := h.checkRedis(ctx)
	w.Header().Set("Content-Type", "application/json")

	statusCode := http.StatusOK
	if check.Status != "up" {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(check)
}

// HealthCheckEmail handles email provider-specific health check
func (h *Handler) HealthCheckEmail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	check := h.checkEmailProvider(ctx)
	w.Header().Set("Content-Type", "application/json")

	statusCode := http.StatusOK
	if check.Status != "up" {
		statusCode = http.StatusServiceUnavailable
	}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(check)
}

func (h *Handler) checkRabbitMQ(ctx context.Context) CheckResult {
	start := time.Now()

	if h.rabbitMQConn == nil || h.rabbitMQConn.IsClosed() {
		return CheckResult{
			Status: "down",
			Error:  "rabbitmq connection not initialized or closed",
		}
	}

	if h.rabbitMQCh == nil || h.rabbitMQCh.IsClosed() {
		return CheckResult{
			Status: "down",
			Error:  "rabbitmq channel not initialized or closed",
		}
	}

	// Try to declare a test queue to verify connection
	testQueue := "health.check.temp"
	_, err := h.rabbitMQCh.QueueDeclare(
		testQueue,
		false, // not durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return CheckResult{
			Status:       "down",
			ResponseTime: time.Since(start).String(),
			Error:        err.Error(),
		}
	}

	// Clean up test queue
	_, _ = h.rabbitMQCh.QueueDelete(testQueue, false, false, false)

	return CheckResult{
		Status:       "up",
		ResponseTime: time.Since(start).String(),
	}
}

func (h *Handler) checkRedis(ctx context.Context) CheckResult {
	start := time.Now()

	if h.redisClient == nil {
		return CheckResult{
			Status: "down",
			Error:  "redis client not initialized",
		}
	}

	// Ping Redis with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := h.redisClient.Ping(pingCtx).Err(); err != nil {
		return CheckResult{
			Status:       "down",
			ResponseTime: time.Since(start).String(),
			Error:        err.Error(),
		}
	}

	return CheckResult{
		Status:       "up",
		ResponseTime: time.Since(start).String(),
	}
}

func (h *Handler) checkEmailProvider(ctx context.Context) CheckResult {
	start := time.Now()

	if h.emailSender == nil {
		return CheckResult{
			Status: "down",
			Error:  "email sender not initialized",
		}
	}

	if err := h.emailSender.CheckHealth(ctx); err != nil {
		return CheckResult{
			Status:       "down",
			ResponseTime: time.Since(start).String(),
			Error:        err.Error(),
		}
	}

	return CheckResult{
		Status:       "up",
		ResponseTime: time.Since(start).String(),
	}
}

