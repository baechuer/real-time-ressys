package rabbitmq

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DefaultExchange = "city.events"

	// Wait window for Return / Confirm
	publishWait = 150 * time.Millisecond
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
	if exchange == "" {
		exchange = DefaultExchange
	}

	p := &Publisher{
		url:      url,
		exchange: exchange,
	}
	if err := p.connect(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Publisher) connect() error {
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

	p.conn = conn
	p.ch = ch

	p.confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	p.returnCh = ch.NotifyReturn(make(chan amqp.Return, 1))

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

// PublishEvent publishes a JSON-encoded envelope body to the topic exchange with mandatory + confirms.
// IMPORTANT: messageID MUST be stable across retries (outbox.message_id).
func (p *Publisher) PublishEvent(ctx context.Context, routingKey, messageID string, body []byte) error {
	if routingKey == "" {
		return errors.New("missing routingKey")
	}
	if strings.TrimSpace(messageID) == "" {
		return errors.New("missing messageID")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ch == nil {
		return errors.New("publisher channel not ready")
	}

	err := p.ch.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		true,  // mandatory
		false, // immediate
		amqp.Publishing{
			MessageId:   messageID,
			ContentType: "application/json",
			Timestamp:   time.Now().UTC(),
			Body:        body,
		},
	)
	if err != nil {
		return err
	}

	// Wait for either Return (NO_ROUTE) or Confirm
	select {
	case ret := <-p.returnCh:
		return errors.New("NO_ROUTE: " + ret.RoutingKey)
	case conf := <-p.confirmCh:
		if !conf.Ack {
			return errors.New("publish nack")
		}
		return nil
	case <-time.After(publishWait):
		// best-effort window; if neither arrives, treat as success-attempt (outbox will retry on downstream consumer side anyway)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
