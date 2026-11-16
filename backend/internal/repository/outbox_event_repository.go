package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type OutboxEvent struct {
	ID          string          `json:"id"`
	AuctionID   string          `json:"auction_id"`
	EventID     string          `json:"event_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at"`
}

type OutboxEventRepository struct {
	db DBTX
}

func NewOutboxEventRepository(db DBTX) *OutboxEventRepository {
	return &OutboxEventRepository{db: db}
}

func (r *OutboxEventRepository) Create(
	ctx context.Context,
	event *OutboxEvent,
) error {
	query := `
		INSERT INTO outbox_events (
			auction_id, event_id, event_type, payload, status
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		event.AuctionID,
		event.EventID,
		event.EventType,
		event.Payload,
		event.Status,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	return nil
}

func (r *OutboxEventRepository) GetByID(
	ctx context.Context,
	id string,
) (*OutboxEvent, error) {
	query := `
		SELECT id, auction_id, event_id, event_type,
		       payload, status, created_at, published_at
		FROM outbox_events
		WHERE id = $1
	`

	event := &OutboxEvent{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&event.ID,
		&event.AuctionID,
		&event.EventID,
		&event.EventType,
		&event.Payload,
		&event.Status,
		&event.CreatedAt,
		&event.PublishedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get outbox event: %w", err)
	}

	return event, nil
}

func (r *OutboxEventRepository) GetPending(
	ctx context.Context,
	limit int,
) ([]OutboxEvent, error) {
	query := `
		SELECT id, auction_id, event_id, event_type,
		       payload, status, created_at, published_at
		FROM outbox_events
		WHERE status = 'PENDING'
		ORDER BY created_at
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]OutboxEvent, 0)

	for rows.Next() {
		var event OutboxEvent

		err := rows.Scan(
			&event.ID,
			&event.AuctionID,
			&event.EventID,
			&event.EventType,
			&event.Payload,
			&event.Status,
			&event.CreatedAt,
			&event.PublishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan outbox event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate outbox events: %w", err)
	}

	return events, nil
}

func (r *OutboxEventRepository) MarkProcessing(
	ctx context.Context,
	id string,
) error {
	query := `
		UPDATE outbox_events
		SET status = 'PROCESSING'
		WHERE id = $1 AND status = 'PENDING'
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event processing: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *OutboxEventRepository) MarkPublished(
	ctx context.Context,
	id string,
) error {
	query := `
		UPDATE outbox_events
		SET status = 'PUBLISHED', published_at = NOW()
		WHERE id = $1 AND status = 'PROCESSING'
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark outbox event published: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
