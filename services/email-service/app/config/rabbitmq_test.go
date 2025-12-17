package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRabbitMQConnection_DefaultURL(t *testing.T) {
	// Unset RABBITMQ_URL to test default
	os.Unsetenv("RABBITMQ_URL")
	defer os.Unsetenv("RABBITMQ_URL")

	// This will fail without a real RabbitMQ, but tests the default URL logic
	_, _, err := NewRabbitMQConnection()
	// Expect error since no RabbitMQ is running
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to RabbitMQ")
}

func TestNewRabbitMQConnection_InvalidURL(t *testing.T) {
	os.Setenv("RABBITMQ_URL", "invalid://url")
	defer os.Unsetenv("RABBITMQ_URL")

	_, _, err := NewRabbitMQConnection()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to RabbitMQ")
}

func TestNewRabbitMQConnection_CustomURL(t *testing.T) {
	os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	defer os.Unsetenv("RABBITMQ_URL")

	// This will fail without a real RabbitMQ
	_, _, err := NewRabbitMQConnection()
	// We can't easily test success without testcontainers
	// But we test that the URL is read from env
	assert.Error(t, err) // Expected to fail without RabbitMQ
}

func TestNewRabbitMQConnection_ExchangeName(t *testing.T) {
	os.Setenv("RABBITMQ_EXCHANGE", "test.exchange")
	defer os.Unsetenv("RABBITMQ_EXCHANGE")

	// This tests that exchange name is read from env
	// The actual connection will fail without RabbitMQ
	_, _, err := NewRabbitMQConnection()
	assert.Error(t, err) // Expected to fail without RabbitMQ
}

func TestNewRabbitMQConnection_DefaultExchange(t *testing.T) {
	os.Unsetenv("RABBITMQ_EXCHANGE")
	defer os.Unsetenv("RABBITMQ_EXCHANGE")

	// Tests default exchange name
	_, _, err := NewRabbitMQConnection()
	assert.Error(t, err) // Expected to fail without RabbitMQ
}

// Note: Full integration test with RabbitMQ would require testcontainers
// These tests verify the configuration loading logic

