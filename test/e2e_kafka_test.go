package e2e

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ARK21/deblock/internal/app/kafka"
	"github.com/stretchr/testify/require"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	wmk "github.com/ThreeDotsLabs/watermill-kafka/v3/pkg/kafka"

	"github.com/ThreeDotsLabs/watermill/components/cqrs"
)

func TestCQRS_EventBus_PublishesToKafka(t *testing.T) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		t.Skip("set KAFKA_BROKERS to run this test (e.g., 127.0.0.1:9092)")
	}
	topic := getenv("KAFKA_TOPIC", "tx-events")

	logger := watermill.NewStdLogger(false, false)

	pub, err := wmk.NewPublisher(
		wmk.PublisherConfig{
			Brokers:   []string{brokers},
			Marshaler: wmk.DefaultMarshaler{},
		},
		logger,
	)
	require.NoError(t, err)
	defer pub.Close()

	ebus, err := cqrs.NewEventBusWithConfig(pub, cqrs.EventBusConfig{
		Marshaler: cqrs.JSONMarshaler{
			GenerateName: cqrs.StructName,
		},
		GeneratePublishTopic: func(_ cqrs.GenerateEventPublishTopicParams) (string, error) {
			return topic, nil
		},
	})
	require.NoError(t, err)

	sub, err := wmk.NewSubscriber(
		wmk.SubscriberConfig{
			Brokers:       []string{brokers},
			Unmarshaler:   wmk.DefaultMarshaler{},
			ConsumerGroup: "e2e-cqrs",
		},
		logger,
	)
	require.NoError(t, err)
	defer sub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ch, err := sub.Subscribe(ctx, topic)
	require.NoError(t, err)

	e := kafka.MatchedTxEvent{
		Header:      kafka.NewMessageHeader("MatchedTxEvent"),
		UserID:      "u1",
		Address:     "0xabc",
		Direction:   "out",
		TxHash:      "0xdeadbeef",
		BlockNumber: 1,
		BlockTime:   1710000000,
		From:        "0xabc",
		To:          "0xdef",
		AmountWei:   "21000",
		AmountEth:   "0.000000000000021000",
		FeeWei:      "42000",
		FeeEth:      "0.000000000000042000",
		Status:      "success",
		ChainID:     1,
		Reorged:     false,
	}
	require.NoError(t, ebus.Publish(ctx, e))

	// ---- Consume and assert ----
	var msg *message.Message
	select {
	case msg = <-ch:
	case <-ctx.Done():
		t.Fatal("did not receive message from Kafka")
	}

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	require.Equal(t, "u1", payload["userId"])
	require.Equal(t, "out", payload["direction"])
	require.Equal(t, "0xdeadbeef", payload["txHash"])
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
