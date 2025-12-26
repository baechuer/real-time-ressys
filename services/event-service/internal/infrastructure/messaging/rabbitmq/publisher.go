package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	zlog "github.com/rs/zerolog/log"
)

const (
	defaultExchange = "city.events"
	publishWait     = 150 * time.Millisecond
)

type Publisher struct {
	url      string
	exchange string

	mu sync.Mutex

	conn *amqp.Connection
	ch   *amqp.Channel

	confirmCh <-chan amqp.Confirmation
	returnCh  <-chan amqp.Return
}

func NewPublisher(url, exchange string) (*Publisher, error) {
	if url == "" {
		return nil, errors.New("missing rabbit url")
	}
	if exchange == "" {
		exchange = defaultExchange
	}
	p := &Publisher{
		url:      url,
		exchange: exchange,
	}
	if err := p.connectLocked(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Publisher) connectLocked() error {
	// caller must hold p.mu or be in constructor before concurrent use
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	// enable publisher confirms
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	// small buffered chans
	p.confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	p.returnCh = ch.NotifyReturn(make(chan amqp.Return, 1))

	p.conn = conn
	p.ch = ch
	return nil
}

func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ch != nil {
		_ = p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
	return nil
}

// PublishEvent implements application port: event.EventPublisher
func (p *Publisher) PublishEvent(ctx context.Context, routingKey string, payload any) error {
	if routingKey == "" {
		return errors.New("missing routingKey")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// If caller didn't set a deadline, keep our own small window.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// lazily reconnect if needed
	if p.ch == nil || p.conn == nil || p.conn.IsClosed() {
		_ = p.Close()
		if err := p.connectLocked(); err != nil {
			return fmt.Errorf("rabbit reconnect failed: %w", err)
		}
	}

	pub := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
	}

	// mandatory=true -> NO_ROUTE surfaces in returnCh
	if err := p.ch.PublishWithContext(ctx, p.exchange, routingKey, true, false, pub); err != nil {
		return err
	}

	timer := time.NewTimer(publishWait)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			// If caller cancels / deadline, bubble up
			return ctx.Err()

		case ret := <-p.returnCh:
			// NOTE: ReplyCode is uint16 in amqp091-go
			zlog.Error().
				Str("exchange", p.exchange).
				Str("rk", routingKey).
				Int("code", int(ret.ReplyCode)).
				Str("reason", ret.ReplyText).
				Msg("rabbit publish returned (mandatory)")
			return fmt.Errorf("rabbit returned: %d %s", ret.ReplyCode, ret.ReplyText)

		case conf := <-p.confirmCh:
			if !conf.Ack {
				return errors.New("rabbit publish not acked")
			}
			return nil

		case <-timer.C:
			// best-effort: don't hard-fail business flow on confirm timing
			zlog.Warn().
				Str("exchange", p.exchange).
				Str("rk", routingKey).
				Msg("rabbit confirm/return timeout window elapsed")
			return nil
		}
	}
}
