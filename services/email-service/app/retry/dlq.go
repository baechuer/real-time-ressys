package retry

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DLQHandler handles publishing messages to Dead Letter Queue
type DLQHandler struct {
	ch      *amqp.Channel
	dlqName string
}

// NewDLQHandler creates a new DLQ handler
func NewDLQHandler(ch *amqp.Channel, dlqName string) (*DLQHandler, error) {
	// Declare DLQ
	_, err := ch.QueueDeclare(
		dlqName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare DLQ: %w", err)
	}

	return &DLQHandler{
		ch:      ch,
		dlqName: dlqName,
	}, nil
}

// PublishToDLQ publishes a failed message to the Dead Letter Queue
func (d *DLQHandler) PublishToDLQ(ctx context.Context, delivery amqp.Delivery, reason string) error {
	// Add failure reason to headers
	headers := make(amqp.Table)
	if delivery.Headers != nil {
		for k, v := range delivery.Headers {
			headers[k] = v
		}
	}
	headers["x-failure-reason"] = reason
	headers["x-failed-at"] = fmt.Sprintf("%d", time.Now().Unix())

	err := d.ch.PublishWithContext(
		ctx,
		"",        // exchange (use default)
		d.dlqName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  delivery.ContentType,
			Body:         delivery.Body,
			Headers:      headers,
			DeliveryMode: amqp.Persistent,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	return nil
}
