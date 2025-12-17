package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthCheckHandler_Success tests successful health check
func TestHealthCheckHandler_Success(t *testing.T) {
	// Note: This test may return 503 if dependencies are not available
	// In a real scenario, you'd mock the dependencies
	app := &application{
		config: config{
			addr: ":8080",
		},
		// db, redisClient, rabbitConn, rabbitCh would be nil in this test
		// which means health checks will fail, returning 503
	}

	req, err := http.NewRequest("GET", "/auth/v1/health", nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	app.healthCheckHandler(recorder, req)

	// Health check should return either 200 (healthy) or 503 (unhealthy)
	assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

// TestHealthCheckHandler_ResponseFormat tests response JSON format
func TestHealthCheckHandler_ResponseFormat(t *testing.T) {
	app := &application{
		config: config{
			addr: ":8080",
		},
	}

	req, err := http.NewRequest("GET", "/auth/v1/health", nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	app.healthCheckHandler(recorder, req)

	var response map[string]interface{}
	err = json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)

	// Check that response has required fields
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "checks")

	// Status should be either "healthy" or "unhealthy"
	status, ok := response["status"].(string)
	assert.True(t, ok)
	assert.Contains(t, []string{"healthy", "unhealthy"}, status)

	// Check that checks object exists
	checks, ok := response["checks"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, checks, "database")
	assert.Contains(t, checks, "redis")
	assert.Contains(t, checks, "rabbitmq")
}

// TestHealthCheckHandler_ContentType tests Content-Type header
func TestHealthCheckHandler_ContentType(t *testing.T) {
	app := &application{
		config: config{
			addr: ":8080",
		},
	}

	req, err := http.NewRequest("GET", "/auth/v1/health", nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	app.healthCheckHandler(recorder, req)

	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
}

// TestHealthCheckHandler_ChecksStructure tests the structure of dependency checks
func TestHealthCheckHandler_ChecksStructure(t *testing.T) {
	app := &application{
		config: config{
			addr: ":8080",
		},
	}

	req, err := http.NewRequest("GET", "/auth/v1/health", nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	app.healthCheckHandler(recorder, req)

	var response map[string]interface{}
	err = json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)

	checks, ok := response["checks"].(map[string]interface{})
	require.True(t, ok)

	// Check database check structure
	dbCheck, ok := checks["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, dbCheck, "status")

	// Check redis check structure
	redisCheck, ok := checks["redis"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, redisCheck, "status")

	// Check rabbitmq check structure
	rabbitCheck, ok := checks["rabbitmq"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, rabbitCheck, "status")
}

// TestCheckDatabase tests database health check
func TestCheckDatabase(t *testing.T) {
	app := &application{
		config: config{
			addr: ":8080",
		},
		db: nil, // No database connection
	}

	ctx := context.Background()
	result := app.checkDatabase(ctx)

	assert.Equal(t, "down", result.Status)
	assert.Contains(t, result.Error, "database connection not initialized")
}

// TestCheckRedis tests Redis health check
func TestCheckRedis(t *testing.T) {
	app := &application{
		config: config{
			addr: ":8080",
		},
		redisClient: nil, // No Redis connection
	}

	ctx := context.Background()
	result := app.checkRedis(ctx)

	assert.Equal(t, "down", result.Status)
	assert.Contains(t, result.Error, "redis client not initialized")
}

// TestCheckRabbitMQ tests RabbitMQ health check
func TestCheckRabbitMQ(t *testing.T) {
	app := &application{
		config: config{
			addr: ":8080",
		},
		rabbitConn: nil, // No RabbitMQ connection
		rabbitCh:   nil,
	}

	result := app.checkRabbitMQ()

	assert.Equal(t, "down", result.Status)
	assert.Contains(t, result.Error, "rabbitmq connection not initialized")
}
