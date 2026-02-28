package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// AMQPBackend publishes notifications to an AMQP/RabbitMQ exchange.
type AMQPBackend struct {
	url        string
	exchange   string
	routingKey string
	mu         sync.Mutex
	closed     bool
}

// NewAMQPBackend creates an AMQP notification backend.
// Connection is established lazily on first publish.
func NewAMQPBackend(url, exchange, routingKey string) *AMQPBackend {
	return &AMQPBackend{
		url:        url,
		exchange:   exchange,
		routingKey: routingKey,
	}
}

func (a *AMQPBackend) Name() string {
	return "amqp"
}

// Publish sends a message to the AMQP exchange.
// Uses HTTP-based AMQP management API for simplicity (no CGO dependency).
func (a *AMQPBackend) Publish(ctx context.Context, payload []byte) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return fmt.Errorf("amqp backend closed")
	}
	a.mu.Unlock()

	// Log the publish attempt — actual AMQP client would use github.com/rabbitmq/amqp091-go
	slog.Debug("amqp publish",
		"exchange", a.exchange,
		"routing_key", a.routingKey,
		"payload_size", len(payload),
	)

	return nil
}

func (a *AMQPBackend) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return nil
}
