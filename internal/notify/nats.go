package notify

import (
	"context"

	"github.com/nats-io/nats.go"
)

// NATSBackend publishes notification events to a NATS subject.
type NATSBackend struct {
	conn    *nats.Conn
	subject string
}

func NewNATSBackend(url, subject string) (*NATSBackend, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NATSBackend{conn: conn, subject: subject}, nil
}

func (n *NATSBackend) Name() string {
	return "nats"
}

func (n *NATSBackend) Publish(_ context.Context, payload []byte) error {
	return n.conn.Publish(n.subject, payload)
}

func (n *NATSBackend) Close() error {
	n.conn.Close()
	return nil
}
