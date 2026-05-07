package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// OutboxMessage represents a pending outbound message
type OutboxMessage struct {
	ID          int64
	Subject     string
	Payload     []byte
	Status      string // pending, sent, failed
	Attempts    int
	CreatedAt   time.Time
	ProcessedAt *time.Time
	Error       string
}

// Store persists outbox messages in the same DB transaction as business data
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// SaveWithTx saves an outbox message within an existing transaction
// This guarantees at-least-once delivery: if the NATS publish fails,
// the relay will retry from the DB.
func (s *Store) SaveWithTx(tx *sql.Tx, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	_, err = tx.ExecContext(context.Background(), `
		INSERT INTO reward_outbox (subject, payload, status, attempts, created_at)
		VALUES ($1, $2, 'pending', 0, NOW())
	`, subject, data)

	return err
}

// FetchPending fetches unprocessed outbox messages for relay
func (s *Store) FetchPending(ctx context.Context, limit int) ([]OutboxMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, subject, payload, status, attempts, created_at
		FROM reward_outbox
		WHERE status = 'pending' AND attempts < 5
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox: %w", err)
	}
	defer rows.Close()

	var msgs []OutboxMessage
	for rows.Next() {
		var m OutboxMessage
		if err := rows.Scan(&m.ID, &m.Subject, &m.Payload, &m.Status, &m.Attempts, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkSent marks a message as successfully sent
func (s *Store) MarkSent(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE reward_outbox
		SET status = 'sent', processed_at = $1
		WHERE id = $2
	`, now, id)
	return err
}

// MarkFailed increments attempts and sets error message
func (s *Store) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE reward_outbox
		SET attempts = attempts + 1,
		    error = $1,
		    status = CASE WHEN attempts + 1 >= 5 THEN 'dead' ELSE 'pending' END
		WHERE id = $2
	`, errMsg, id)
	return err
}

// PublishFn is the function to send a message to NATS
type PublishFn func(ctx context.Context, subject string, payload []byte) error

// Relay polls the outbox and publishes pending messages
type Relay struct {
	store     *Store
	publishFn PublishFn
	interval  time.Duration
	batchSize int
	logger    *slog.Logger
}

func NewRelay(store *Store, publishFn PublishFn, interval time.Duration) *Relay {
	return &Relay{
		store:     store,
		publishFn: publishFn,
		interval:  interval,
		batchSize: 100,
		logger:    slog.Default(),
	}
}

// Start runs the relay loop until context is cancelled
func (r *Relay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.Info("outbox relay started", "interval", r.interval)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("outbox relay stopped")
			return
		case <-ticker.C:
			if err := r.process(ctx); err != nil {
				r.logger.Error("outbox relay error", "error", err)
			}
		}
	}
}

func (r *Relay) process(ctx context.Context) error {
	msgs, err := r.store.FetchPending(ctx, r.batchSize)
	if err != nil {
		return fmt.Errorf("fetch pending: %w", err)
	}

	for _, msg := range msgs {
		if err := r.publishFn(ctx, msg.Subject, msg.Payload); err != nil {
			r.logger.Warn("outbox publish failed",
				"id", msg.ID,
				"subject", msg.Subject,
				"attempts", msg.Attempts,
				"error", err,
			)
			_ = r.store.MarkFailed(ctx, msg.ID, err.Error())
			continue
		}

		if err := r.store.MarkSent(ctx, msg.ID); err != nil {
			r.logger.Error("mark sent failed", "id", msg.ID, "error", err)
		}
	}

	return nil
}

// MigrationSQL returns the SQL to create the outbox table
const MigrationSQL = `
CREATE TABLE IF NOT EXISTS reward_outbox (
	id           BIGSERIAL    PRIMARY KEY,
	subject      TEXT         NOT NULL,
	payload      JSONB        NOT NULL,
	status       TEXT         NOT NULL DEFAULT 'pending',  -- pending | sent | dead
	attempts     INT          NOT NULL DEFAULT 0,
	error        TEXT,
	created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
	processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reward_outbox_pending
	ON reward_outbox(status, attempts, created_at)
	WHERE status = 'pending';
`
