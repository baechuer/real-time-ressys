package config

import (
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NewRabbitMQConnection creates a new RabbitMQ connection and channel
func NewRabbitMQConnection() (*amqp.Connection, *amqp.Channel, error) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange (should match auth-service)
	exchangeName := GetString("RABBITMQ_EXCHANGE", "auth.events")
	err = ch.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return conn, ch, nil
}

