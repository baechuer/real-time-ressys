package event

import "context"

type NoopPublisher struct{}

func (NoopPublisher) PublishEvent(ctx context.Context, routingKey, messageID string, body []byte) error {
	return nil
}
