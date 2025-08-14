package kafka

import (
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"
	"github.com/ThreeDotsLabs/watermill/message"
)

type Producer interface {
	Send(event TxEvent) error
	Close() error
}

func NewKafkaPublisher(brokers []string) (message.Publisher, error) {
	logger := watermill.NewStdLogger(false, false)

	var pub message.Publisher

	pub, err := kafka.NewPublisher(
		kafka.PublisherConfig{
			Brokers:   brokers,
			Marshaler: kafka.DefaultMarshaler{},
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating kafka publisher: %w", err)
	}

	return pub, nil
}
