package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

const (
	// Tier exchanges (topic)
	DLX10sExchange = "city.events.dlx.10s"
	DLX1mExchange  = "city.events.dlx.1m"
	DLX10mExchange = "city.events.dlx.10m"

	// Final DLQ exchange (topic)
	DLXFinalExchange = "city.events.dlx.final"

	// Final DLQ routing key
	rkFinalDLQ = "email.final.dlq"

	// publish reliability window
	publishWait = 250 * time.Millisecond
)

type RetryPublisher struct {
	ch *amqp.Channel
	lg zerolog.Logger

	confirmCh <-chan amqp.Confirmation
	returnCh  <-chan amqp.Return

	ex10s string
	ex1m  string
	ex10m string
	exDLQ string
}

func NewRetryPublisher(ch *amqp.Channel, lg zerolog.Logger) (*RetryPublisher, error) {
	if ch == nil {
		return nil, fmt.Errorf("nil channel")
	}

	// Enable publisher confirms
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("confirm mode: %w", err)
	}

	p := &RetryPublisher{
		ch:    ch,
		lg:    lg.With().Str("component", "retry_publisher").Logger(),
		ex10s: DLX10sExchange,
		ex1m:  DLX1mExchange,
		ex10m: DLX10mExchange,
		exDLQ: DLXFinalExchange,
	}

	// Must be registered AFTER Confirm(true/false)
	p.confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, 32))
	p.returnCh = ch.NotifyReturn(make(chan amqp.Return, 32))

	return p, nil
}

func (p *RetryPublisher) PublishRetry(
	ctx context.Context,
	tier string,
	orig amqp.Delivery,
	nextAttempt int,
	cause error,
) error {
	ex := p.ex10m
	switch tier {
	case "10s":
		ex = p.ex10s
	case "1m":
		ex = p.ex1m
	case "10m":
		ex = p.ex10m
	}

	h := copyHeaders(orig.Headers)
	h["x-attempt"] = nextAttempt
	h["x-orig-routing-key"] = orig.RoutingKey
	if cause != nil {
		h["x-error"] = cause.Error()
	}

	pub := amqp.Publishing{
		ContentType:   orig.ContentType,
		Body:          orig.Body,
		DeliveryMode:  amqp.Persistent,
		Timestamp:     time.Now(),
		Headers:       h,
		CorrelationId: orig.CorrelationId,
		MessageId:     orig.MessageId,
	}

	// IMPORTANT:
	// - mandatory=true so NO_ROUTE is observable via Return channel
	// - preserve original business routing key
	if err := p.ch.PublishWithContext(ctx, ex, orig.RoutingKey, true, false, pub); err != nil {
		return fmt.Errorf("publish retry: %w", err)
	}

	return p.waitAckOrReturn(ctx, ex, orig.RoutingKey)
}

func (p *RetryPublisher) PublishFinal(
	ctx context.Context,
	orig amqp.Delivery,
	reason string,
	cause error,
) error {
	h := copyHeaders(orig.Headers)
	h["x-orig-routing-key"] = orig.RoutingKey
	h["x-dlq-reason"] = reason
	if cause != nil {
		h["x-error"] = cause.Error()
	}

	pub := amqp.Publishing{
		ContentType:   orig.ContentType,
		Body:          orig.Body,
		DeliveryMode:  amqp.Persistent,
		Timestamp:     time.Now(),
		Headers:       h,
		CorrelationId: orig.CorrelationId,
		MessageId:     orig.MessageId,
	}

	if err := p.ch.PublishWithContext(ctx, p.exDLQ, rkFinalDLQ, true, false, pub); err != nil {
		return fmt.Errorf("publish final dlq: %w", err)
	}

	return p.waitAckOrReturn(ctx, p.exDLQ, rkFinalDLQ)
}

func (p *RetryPublisher) waitAckOrReturn(ctx context.Context, exchange, rk string) error {
	timer := time.NewTimer(publishWait)
	defer timer.Stop()

	for {
		select {
		case r := <-p.returnCh:
			// NO_ROUTE -> treat as fatal publish error (otherwise retry disappears silently)
			return fmt.Errorf("publish returned: reply=%d text=%q exchange=%q rk=%q",
				r.ReplyCode, r.ReplyText, r.Exchange, r.RoutingKey)

		case c := <-p.confirmCh:
			if !c.Ack {
				return fmt.Errorf("publish nacked by broker (exchange=%q rk=%q)", exchange, rk)
			}
			return nil

		case <-timer.C:
			// if neither return nor confirm arrives in the window, treat as error
			// (keeps behavior consistent with your auth-service publisher contract)
			return errors.New("publish wait timeout (no confirm/return)")

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func copyHeaders(in amqp.Table) amqp.Table {
	out := amqp.Table{}
	if in == nil {
		return out
	}
	for k, v := range in {
		out[k] = v
	}
	return out
}
