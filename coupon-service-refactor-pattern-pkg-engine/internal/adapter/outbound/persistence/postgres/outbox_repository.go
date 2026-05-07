package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	repo "coupon-service/internal/interface/repository"
	"coupon-service/internal/model"
)

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

var _ repo.OutboxRepository = (*OutboxRepository)(nil)

//////////////////////////////////////////////////
// Insert Event (ใช้ใน Transaction ของ Service)
//////////////////////////////////////////////////

func (r *OutboxRepository) InsertTx(
	ctx context.Context,
	tx *sql.Tx,
	event model.OutboxInsert,
) error {

	data, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events (
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			status
		)
		VALUES ($1,$2,$3,$4,'PENDING')
	`,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		data,
	)

	return err
}

//////////////////////////////////////////////////
// GetPending (Worker ใช้)
//////////////////////////////////////////////////

func (r *OutboxRepository) GetPending(
	ctx context.Context,
	tx *sql.Tx,
	limit int,
) ([]model.OutboxEvent, error) {

	rows, err := tx.QueryContext(ctx, `
		SELECT
			id,
			aggregate_id,
			event_type,
			payload,
			status,
			created_at
		FROM outbox_events
		WHERE status = 'PENDING'
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var events []model.OutboxEvent

	for rows.Next() {

		var e model.OutboxEvent
		var payloadBytes []byte
		var createdAt time.Time

		if err := rows.Scan(
			&e.ID,
			&e.AggregateID,
			&e.Type,
			&payloadBytes,
			&e.Status,
			&createdAt,
		); err != nil {
			return nil, err
		}

		e.Payload = payloadBytes
		e.CreatedAt = createdAt

		events = append(events, e)
	}

	return events, nil
}

//////////////////////////////////////////////////
// Mark Event Sent
//////////////////////////////////////////////////

func (r *OutboxRepository) MarkSent(
	ctx context.Context,
	tx *sql.Tx,
	id int64,
) error {

	_, err := tx.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'SENT'
		WHERE id = $1
	`, id)

	return err
}
