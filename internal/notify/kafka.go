package notify

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaBackend publishes notification events to a Kafka topic.
type KafkaBackend struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaBackend(brokers []string, topic string) *KafkaBackend {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 100 * time.Millisecond,
		Async:        true,
	}
	return &KafkaBackend{writer: w, topic: topic}
}

func (k *KafkaBackend) Name() string {
	return "kafka"
}

func (k *KafkaBackend) Publish(ctx context.Context, payload []byte) error {
	return k.writer.WriteMessages(ctx, kafka.Message{
		Value: payload,
	})
}

func (k *KafkaBackend) Close() error {
	return k.writer.Close()
}
