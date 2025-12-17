package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string                 `json:"status"` // "healthy" or "unhealthy"
	Timestamp string                 `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
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

func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]CheckResult)
	overallStatus := "healthy"

	// Check Postgres
	dbCheck := app.checkDatabase(ctx)
	checks["database"] = dbCheck
	if dbCheck.Status != "up" {
		overallStatus = "unhealthy"
	}

	// Check Redis
	redisCheck := app.checkRedis(ctx)
	checks["redis"] = redisCheck
	if redisCheck.Status != "up" {
		overallStatus = "unhealthy"
	}

	// Check RabbitMQ
	rabbitCheck := app.checkRabbitMQ()
	checks["rabbitmq"] = rabbitCheck
	if rabbitCheck.Status != "up" {
		overallStatus = "unhealthy"
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

func (app *application) checkDatabase(ctx context.Context) CheckResult {
	start := time.Now()

	if app.db == nil {
		return CheckResult{
			Status: "down",
			Error:  "database connection not initialized",
		}
	}

	// Try to ping the database
	if err := app.db.PingContext(ctx); err != nil {
		return CheckResult{
			Status:       "down",
			ResponseTime: time.Since(start).String(),
			Error:        err.Error(),
		}
	}

	// Additional check: try a simple query
	if db, ok := app.db.(*sql.DB); ok {
		var result int
		if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
			return CheckResult{
				Status:       "down",
				ResponseTime: time.Since(start).String(),
				Error:        err.Error(),
			}
		}
	}

	return CheckResult{
		Status:       "up",
		ResponseTime: time.Since(start).String(),
	}
}

func (app *application) checkRedis(ctx context.Context) CheckResult {
	start := time.Now()

	if app.redisClient == nil {
		return CheckResult{
			Status: "down",
			Error:  "redis client not initialized",
		}
	}

	// Ping Redis with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := app.redisClient.Ping(pingCtx).Err(); err != nil {
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

func (app *application) checkRabbitMQ() CheckResult {
	start := time.Now()

	if app.rabbitConn == nil || app.rabbitCh == nil {
		return CheckResult{
			Status: "down",
			Error:  "rabbitmq connection not initialized",
		}
	}

	// Check if connection is closed
	if conn, ok := app.rabbitConn.(*amqp.Connection); ok {
		if conn.IsClosed() {
			return CheckResult{
				Status:       "down",
				ResponseTime: time.Since(start).String(),
				Error:        "rabbitmq connection is closed",
			}
		}
	}

	// Check if channel is closed
	if ch, ok := app.rabbitCh.(*amqp.Channel); ok {
		if ch.IsClosed() {
			return CheckResult{
				Status:       "down",
				ResponseTime: time.Since(start).String(),
				Error:        "rabbitmq channel is closed",
			}
		}
	}

	return CheckResult{
		Status:       "up",
		ResponseTime: time.Since(start).String(),
	}
}
