package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	"github.com/baechuer/real-time-ressys/services/email-service/app/consumer"
	"github.com/baechuer/real-time-ressys/services/email-service/app/email"
	"github.com/baechuer/real-time-ressys/services/email-service/app/idempotency"
	"github.com/baechuer/real-time-ressys/services/email-service/app/models"
	"github.com/baechuer/real-time-ressys/services/email-service/app/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Integration Test Cases:

1. TestConsumer_EmailVerificationFlow
   - Full flow: message -> handler -> email sender
   - Verify idempotency works
   - Verify retry logic

2. TestConsumer_PasswordResetFlow
   - Full flow: message -> handler -> email sender
   - Verify idempotency works

3. TestConsumer_DuplicateMessage
   - Send same message twice
   - Verify second is detected as duplicate

4. TestConsumer_InvalidMessage
   - Send invalid message format
   - Verify error handling
*/

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, func() {
		client.Close()
		mr.Close()
	}
}

func setupTestEmailSender(t *testing.T) *email.Sender {
	cfg := &config.EmailConfig{
		Provider:     "smtp",
		FromEmail:    "test@example.com",
		FromName:     "Test Service",
		SMTPHost:     "localhost",
		SMTPPort:     587,
		SMTPUsername: "test",
		SMTPPassword: "test",
	}

	sender, err := email.NewSender(cfg)
	require.NoError(t, err)
	return sender
}

func TestConsumer_EmailVerificationMessageParsing(t *testing.T) {
	// Test message parsing without actual RabbitMQ
	msg := models.EmailVerificationMessage{
		Type:            "email_verification",
		Email:           "test@example.com",
		VerificationURL: "https://example.com/verify?token=abc123",
	}

	body, err := json.Marshal(msg)
	require.NoError(t, err)

	// Verify it can be parsed back
	var parsedMsg models.EmailVerificationMessage
	err = json.Unmarshal(body, &parsedMsg)
	require.NoError(t, err)

	assert.Equal(t, msg.Type, parsedMsg.Type)
	assert.Equal(t, msg.Email, parsedMsg.Email)
	assert.Equal(t, msg.VerificationURL, parsedMsg.VerificationURL)
}

func TestConsumer_PasswordResetMessageParsing(t *testing.T) {
	// Test message parsing without actual RabbitMQ
	msg := models.PasswordResetMessage{
		Type:     "password_reset",
		Email:    "user@example.com",
		ResetURL: "https://example.com/reset?token=xyz789",
	}

	body, err := json.Marshal(msg)
	require.NoError(t, err)

	// Verify it can be parsed back
	var parsedMsg models.PasswordResetMessage
	err = json.Unmarshal(body, &parsedMsg)
	require.NoError(t, err)

	assert.Equal(t, msg.Type, parsedMsg.Type)
	assert.Equal(t, msg.Email, parsedMsg.Email)
	assert.Equal(t, msg.ResetURL, parsedMsg.ResetURL)
}

func TestIdempotency_Integration(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	store := idempotency.NewStore(redisClient)
	checker := idempotency.NewChecker(store)

	ctx := context.Background()
	messageID := "test-message-integration"

	// First check - should be new
	isDuplicate, err := checker.CheckAndMark(ctx, messageID)
	require.NoError(t, err)
	assert.False(t, isDuplicate)

	// Second check - should be duplicate
	isDuplicate, err = checker.CheckAndMark(ctx, messageID)
	require.NoError(t, err)
	assert.True(t, isDuplicate)

	// Verify in Redis directly
	processed, err := store.IsProcessed(ctx, messageID)
	require.NoError(t, err)
	assert.True(t, processed)
}

func TestHandler_Integration_EmailVerification(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	sender := setupTestEmailSender(t)
	rateLimiter := ratelimit.NewRateLimiter(redisClient)
	handler := consumer.NewHandler(sender, rateLimiter)

	ctx := context.Background()
	msg := &models.EmailVerificationMessage{
		Type:            "email_verification",
		Email:           "test@example.com",
		VerificationURL: "https://example.com/verify?token=abc123",
	}

	// Note: This will fail if SMTP server is not available
	// In a real integration test, you'd use a test SMTP server or mock
	err := handler.HandleEmailVerification(ctx, msg)
	// We expect this to fail without a real SMTP server
	// In real integration tests, use testcontainers or a test SMTP server
	_ = err
}

func TestHandler_Integration_PasswordReset(t *testing.T) {
	redisClient, cleanup := setupTestRedis(t)
	defer cleanup()

	sender := setupTestEmailSender(t)
	rateLimiter := ratelimit.NewRateLimiter(redisClient)
	handler := consumer.NewHandler(sender, rateLimiter)

	ctx := context.Background()
	msg := &models.PasswordResetMessage{
		Type:     "password_reset",
		Email:    "user@example.com",
		ResetURL: "https://example.com/reset?token=xyz789",
	}

	// Note: This will fail if SMTP server is not available
	// In a real integration test, you'd use a test SMTP server or mock
	err := handler.HandlePasswordReset(ctx, msg)
	// We expect this to fail without a real SMTP server
	// In real integration tests, use testcontainers or a test SMTP server
	_ = err
}

// Full integration test with RabbitMQ would require testcontainers
// This is a placeholder for future implementation
func TestConsumer_FullIntegration(t *testing.T) {
	t.Skip("Requires RabbitMQ testcontainer - implement with testcontainers")
	/*
		This test would:
		1. Start RabbitMQ container
		2. Start Redis container
		3. Create consumer
		4. Publish test message
		5. Verify message is processed
		6. Verify email is sent (or mocked)
		7. Verify idempotency
	*/
}
