package event

import "context"

type NoopPublisher struct{}

func (NoopPublisher) PublishEvent(ctx context.Context, routingKey string, payload any) error {
	return nil
}
