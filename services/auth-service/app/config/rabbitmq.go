package config

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NewRabbitMQConnection creates and verifies a RabbitMQ connection using RABBITMQ_URL.
// Example: amqp://user:password@localhost:5672/
func NewRabbitMQConnection() (*amqp.Connection, *amqp.Channel, error) {
	url := GetString("RABBITMQ_URL", "")
	if url == "" {
		return nil, nil, fmt.Errorf("RABBITMQ_URL is required")
	}

	conn, err := amqp.DialConfig(url, amqp.Config{
		Locale: "en_US",
		Dial:   amqp.DefaultDial(10 * time.Second),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	// Optional: declare a durable topic exchange for auth events
	if err := ch.ExchangeDeclare(
		"auth.events", // name
		"topic",       // type
		true,          // durable
		false,         // auto-deleted
		false,         // internal
		false,         // no-wait
		nil,           // arguments
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return conn, ch, nil
}
