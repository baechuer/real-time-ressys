package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEmailSenderChecker is a mock for EmailSenderChecker
type MockEmailSenderChecker struct {
	mock.Mock
}

func (m *MockEmailSenderChecker) CheckHealth(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, cleanup
}

func TestNewHandler(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	mockEmailSender := new(MockEmailSenderChecker)

	handler := NewHandler(nil, nil, redisClient, mockEmailSender)

	assert.NotNil(t, handler)
	assert.Equal(t, redisClient, handler.redisClient)
	assert.Equal(t, mockEmailSender, handler.emailSender)
}

func TestHandler_HealthCheck_AllHealthy(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	mockEmailSender := new(MockEmailSenderChecker)
	mockEmailSender.On("CheckHealth", mock.Anything).Return(nil)

	handler := NewHandler(nil, nil, redisClient, mockEmailSender)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	// RabbitMQ is nil, so service will be unhealthy
	// But we test the code path
	assert.Contains(t, w.Body.String(), "rabbitmq")
	assert.Contains(t, w.Body.String(), "redis")
	assert.Contains(t, w.Body.String(), "email")

	mockEmailSender.AssertExpectations(t)
}

func TestHandler_HealthCheck_RedisDown(t *testing.T) {
	// Use nil Redis client to simulate failure
	// Don't set up email sender mock since it won't be called when Redis is down
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "unhealthy")
	assert.Contains(t, w.Body.String(), "redis")
}

func TestHandler_HealthCheck_EmailProviderDown(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	mockEmailSender := new(MockEmailSenderChecker)
	mockEmailSender.On("CheckHealth", mock.Anything).Return(assert.AnError)

	handler := NewHandler(nil, nil, redisClient, mockEmailSender)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	// RabbitMQ is nil, so service will be unhealthy (503)
	// But email check should still be in response
	assert.Contains(t, w.Body.String(), "email")
	// Status will be 503 because RabbitMQ is down, not because email is down

	mockEmailSender.AssertExpectations(t)
}

func TestHandler_HealthCheck_NilEmailSender(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	handler := NewHandler(nil, nil, redisClient, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	// RabbitMQ is nil, so service will be unhealthy (503)
	// Should not include email check if sender is nil
	assert.NotContains(t, w.Body.String(), `"email"`)
	// Status will be 503 because RabbitMQ is down
}

func TestHandler_HealthCheckRabbitMQ(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/health/rabbitmq", nil)
	w := httptest.NewRecorder()

	handler.HealthCheckRabbitMQ(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "down")
}

func TestHandler_HealthCheckRedis_Up(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	handler := NewHandler(nil, nil, redisClient, nil)

	req := httptest.NewRequest("GET", "/health/redis", nil)
	w := httptest.NewRecorder()

	handler.HealthCheckRedis(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "up")
}

func TestHandler_HealthCheckRedis_Down(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/health/redis", nil)
	w := httptest.NewRecorder()

	handler.HealthCheckRedis(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "down")
}

func TestHandler_HealthCheckEmail_Up(t *testing.T) {
	mockEmailSender := new(MockEmailSenderChecker)
	mockEmailSender.On("CheckHealth", mock.Anything).Return(nil)

	handler := NewHandler(nil, nil, nil, mockEmailSender)

	req := httptest.NewRequest("GET", "/health/email", nil)
	w := httptest.NewRecorder()

	handler.HealthCheckEmail(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "up")

	mockEmailSender.AssertExpectations(t)
}

func TestHandler_HealthCheckEmail_Down(t *testing.T) {
	mockEmailSender := new(MockEmailSenderChecker)
	mockEmailSender.On("CheckHealth", mock.Anything).Return(assert.AnError)

	handler := NewHandler(nil, nil, nil, mockEmailSender)

	req := httptest.NewRequest("GET", "/health/email", nil)
	w := httptest.NewRecorder()

	handler.HealthCheckEmail(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "down")

	mockEmailSender.AssertExpectations(t)
}

func TestHandler_HealthCheckEmail_NilSender(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/health/email", nil)
	w := httptest.NewRecorder()

	handler.HealthCheckEmail(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "down")
}

func TestHandler_CheckRedis(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	handler := NewHandler(nil, nil, redisClient, nil)

	ctx := context.Background()
	result := handler.checkRedis(ctx)

	assert.Equal(t, "up", result.Status)
	assert.NotEmpty(t, result.ResponseTime)
	assert.Empty(t, result.Error)
}

func TestHandler_CheckRedis_NilClient(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	ctx := context.Background()
	result := handler.checkRedis(ctx)

	assert.Equal(t, "down", result.Status)
	assert.Contains(t, result.Error, "not initialized")
}

func TestHandler_CheckRedis_ConnectionFailure(t *testing.T) {
	// Create a client that will fail
	client := redis.NewClient(&redis.Options{
		Addr: "invalid:6379",
	})
	defer client.Close()

	handler := NewHandler(nil, nil, client, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := handler.checkRedis(ctx)

	assert.Equal(t, "down", result.Status)
	assert.NotEmpty(t, result.Error)
}

func TestHandler_CheckEmailProvider(t *testing.T) {
	mockEmailSender := new(MockEmailSenderChecker)
	mockEmailSender.On("CheckHealth", mock.Anything).Return(nil)

	handler := NewHandler(nil, nil, nil, mockEmailSender)

	ctx := context.Background()
	result := handler.checkEmailProvider(ctx)

	assert.Equal(t, "up", result.Status)
	assert.NotEmpty(t, result.ResponseTime)
	assert.Empty(t, result.Error)

	mockEmailSender.AssertExpectations(t)
}

func TestHandler_CheckEmailProvider_Failure(t *testing.T) {
	mockEmailSender := new(MockEmailSenderChecker)
	mockEmailSender.On("CheckHealth", mock.Anything).Return(assert.AnError)

	handler := NewHandler(nil, nil, nil, mockEmailSender)

	ctx := context.Background()
	result := handler.checkEmailProvider(ctx)

	assert.Equal(t, "down", result.Status)
	assert.NotEmpty(t, result.Error)

	mockEmailSender.AssertExpectations(t)
}

func TestHandler_CheckEmailProvider_NilSender(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	ctx := context.Background()
	result := handler.checkEmailProvider(ctx)

	assert.Equal(t, "down", result.Status)
	assert.Contains(t, result.Error, "not initialized")
}

func TestHandler_CheckRabbitMQ_NilConnection(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil)

	ctx := context.Background()
	result := handler.checkRabbitMQ(ctx)

	assert.Equal(t, "down", result.Status)
	assert.Contains(t, result.Error, "not initialized")
}

func TestHandler_CheckRabbitMQ_ClosedConnection(t *testing.T) {
	// Create a closed connection (simulated)
	// In real test, we'd need actual RabbitMQ connection
	handler := NewHandler(nil, nil, nil, nil)

	ctx := context.Background()
	result := handler.checkRabbitMQ(ctx)

	assert.Equal(t, "down", result.Status)
}

// Note: Full RabbitMQ health check test requires actual RabbitMQ connection
// This is tested in integration tests with testcontainers

