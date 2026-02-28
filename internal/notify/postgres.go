package notify

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresBackend publishes notifications to a PostgreSQL table.
type PostgresBackend struct {
	connStr string
	table   string
	db      *sql.DB
	mu      sync.Mutex
}

// NewPostgresBackend creates a PostgreSQL notification backend.
func NewPostgresBackend(connStr, table string) (*PostgresBackend, error) {
	if table == "" {
		table = "s3_events"
	}
	return &PostgresBackend{
		connStr: connStr,
		table:   table,
	}, nil
}

func (p *PostgresBackend) Name() string {
	return "postgres"
}

func (p *PostgresBackend) ensureConnection() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db != nil {
		return nil
	}
	db, err := sql.Open("postgres", p.connStr)
	if err != nil {
		return fmt.Errorf("postgres connect: %w", err)
	}
	p.db = db

	// Create table if not exists
	createSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGSERIAL PRIMARY KEY,
		event_time TIMESTAMPTZ DEFAULT NOW(),
		payload JSONB NOT NULL
	)`, p.table)
	if _, err := db.Exec(createSQL); err != nil {
		slog.Warn("postgres create table failed", "error", err)
	}
	return nil
}

// Publish inserts an event into the PostgreSQL table.
func (p *PostgresBackend) Publish(ctx context.Context, payload []byte) error {
	if err := p.ensureConnection(); err != nil {
		return err
	}
	p.mu.Lock()
	db := p.db
	p.mu.Unlock()

	insertSQL := fmt.Sprintf("INSERT INTO %s (payload) VALUES ($1)", p.table)
	_, err := db.ExecContext(ctx, insertSQL, string(payload))
	return err
}

func (p *PostgresBackend) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}
