package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AMQPBackend publishes notifications to an AMQP/RabbitMQ exchange.
type AMQPBackend struct {
	url        string
	exchange   string
	routingKey string
	mu         sync.Mutex
	conn       *amqp.Connection
	ch         *amqp.Channel
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

func (a *AMQPBackend) ensureConnection() error {
	if a.conn != nil && !a.conn.IsClosed() {
		return nil
	}

	conn, err := amqp.Dial(a.url)
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("amqp channel: %w", err)
	}

	// Declare exchange (idempotent)
	if a.exchange != "" {
		if err := ch.ExchangeDeclare(a.exchange, "topic", true, false, false, false, nil); err != nil {
			ch.Close()
			conn.Close()
			return fmt.Errorf("amqp exchange declare: %w", err)
		}
	}

	a.conn = conn
	a.ch = ch
	return nil
}

// Publish sends a message to the AMQP exchange.
func (a *AMQPBackend) Publish(ctx context.Context, payload []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("amqp backend closed")
	}

	if err := a.ensureConnection(); err != nil {
		slog.Warn("amqp connection failed", "error", err)
		return err
	}

	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return a.ch.PublishWithContext(pubCtx,
		a.exchange,
		a.routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	)
}

func (a *AMQPBackend) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	if a.ch != nil {
		a.ch.Close()
	}
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}
