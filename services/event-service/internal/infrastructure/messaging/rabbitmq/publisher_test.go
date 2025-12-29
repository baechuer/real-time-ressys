package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPublisher_Integration verifies the full message lifecycle using a real RabbitMQ container.
func TestPublisher_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// 1. Spin up RabbitMQ container using Testcontainers.
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management",
		ExposedPorts: []string{"5672/tcp"},
		WaitingFor:   wait.ForLog("Server startup complete"),
	}
	rabbitC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	assert.NoError(t, err)
	defer rabbitC.Terminate(ctx)

	port, _ := rabbitC.MappedPort(ctx, "5672")
	url := "amqp://guest:guest@localhost:" + port.Port()

	// 2. Setup the infrastructure (Exchange, Queue, and Binding).
	// This is required because the publisher uses the 'mandatory' flag.
	exchangeName := "test.exchange"
	routingKey := "test.key"
	prepareExchangeAndQueue(t, url, exchangeName, routingKey)

	// 3. Initialize the Publisher.
	p, err := NewPublisher(url, exchangeName)
	assert.NoError(t, err)
	defer p.Close()

	t.Run("publish_successfully", func(t *testing.T) {
		payloadMap := map[string]string{"msg": "hello from test"}
		body, _ := json.Marshal(payloadMap)
		messageID := "test-msg-uuid-123"

		// Use a simple retry to handle potential async latency in RabbitMQ routing table updates.
		var publishErr error
		for i := 0; i < 3; i++ {
			publishErr = p.PublishEvent(ctx, routingKey, messageID, body)
			if publishErr == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		assert.NoError(t, publishErr)
	})
}

// prepareExchangeAndQueue ensures the routing path exists to prevent NO_ROUTE errors.
func prepareExchangeAndQueue(t *testing.T, url, exchange, key string) {
	conn, err := amqp.Dial(url)
	assert.NoError(t, err)
	defer conn.Close()

	ch, err := conn.Channel()
	assert.NoError(t, err)
	defer ch.Close()

	// Declare a topic exchange for flexible routing.
	err = ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil)
	assert.NoError(t, err)

	// Declare an exclusive queue that is automatically deleted when the connection closes.
	q, err := ch.QueueDeclare("test.queue", false, false, true, false, nil)
	assert.NoError(t, err)

	// Create the binding. Without this, 'mandatory' publishing would fail.
	err = ch.QueueBind(q.Name, key, exchange, false, nil)
	assert.NoError(t, err)

	// Allow a brief moment for RabbitMQ internal metadata to synchronize.
	time.Sleep(200 * time.Millisecond)
}
