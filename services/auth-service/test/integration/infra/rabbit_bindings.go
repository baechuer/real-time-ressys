package infra

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// BindQueue binds a durable queue to the topic exchange for routed-publish tests.
func BindQueue(conn *amqp.Connection, exchange, queue, bindingKey string) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	return ch.QueueBind(queue, bindingKey, exchange, false, nil)
}

// ConsumeOne pulls 1 message (best-effort) for verifying side effects.
func ConsumeOne(conn *amqp.Connection, queue string, wait time.Duration) ([]byte, bool, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, false, err
	}
	defer ch.Close()

	msgs, err := ch.Consume(queue, "", true, false, false, false, nil)
	if err != nil {
		return nil, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	select {
	case m := <-msgs:
		return m.Body, true, nil
	case <-ctx.Done():
		return nil, false, nil
	}
}
