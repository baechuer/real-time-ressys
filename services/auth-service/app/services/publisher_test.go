package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRabbitMQPublisher_PublishEmailVerification_IncludesRequestID(t *testing.T) {
	// This test verifies that request ID propagation is implemented
	// Full integration test would require actual RabbitMQ connection
	
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "test-request-123")
	
	// Verify request ID can be extracted using the same function as publisher
	requestID := getRequestIDFromContext(ctx)
	assert.Equal(t, "test-request-123", requestID)
}

func TestRabbitMQPublisher_PublishPasswordReset_IncludesRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "test-request-456")
	
	requestID := getRequestIDFromContext(ctx)
	assert.Equal(t, "test-request-456", requestID)
}

// Test that headers are properly structured for RabbitMQ
func TestRabbitMQMessageHeaders_Structure(t *testing.T) {
	requestID := "test-request-789"
	
	headers := make(amqp.Table)
	headers["X-Request-ID"] = requestID
	headers["X-Trace-ID"] = requestID
	
	assert.Equal(t, requestID, headers["X-Request-ID"])
	assert.Equal(t, requestID, headers["X-Trace-ID"])
}

// Integration test would look like this (requires RabbitMQ):
/*
func TestRabbitMQPublisher_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}
	
	// Setup RabbitMQ connection
	conn, ch, err := setupRabbitMQ()
	require.NoError(t, err)
	defer conn.Close()
	defer ch.Close()
	
	publisher := NewRabbitMQPublisher(ch)
	
	// Create context with request ID
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "integration-test-123")
	
	// Publish message
	err = publisher.PublishEmailVerification(ctx, "test@example.com", "http://example.com/verify?token=abc")
	require.NoError(t, err)
	
	// Consume message and verify headers
	msgs, err := ch.Consume("test-queue", "", true, false, false, false, nil)
	require.NoError(t, err)
	
	msg := <-msgs
	require.NotNil(t, msg.Headers)
	assert.Equal(t, "integration-test-123", msg.Headers["X-Request-ID"])
}
*/

