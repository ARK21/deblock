package kafka

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
)

type Publisher interface {
	Publish(ctx context.Context, event any) error
}

type WatermillBus struct {
	*cqrs.EventBus
}

func (w WatermillBus) Publish(ctx context.Context, event any) error {
	return w.EventBus.Publish(ctx, event)
}

func NewEventBus(pub message.Publisher, topic string) (*cqrs.EventBus, error) {
	bus, err := cqrs.NewEventBusWithConfig(pub, cqrs.EventBusConfig{
		GeneratePublishTopic: func(params cqrs.GenerateEventPublishTopicParams) (string, error) {
			return topic, nil
		},
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create cqrs event bus: %w", err)
	}

	return bus, nil
}
