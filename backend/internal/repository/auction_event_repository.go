package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type AuctionEvent struct {
	ID             string          `json:"id"`
	AuctionID      string          `json:"auction_id"`
	SequenceNumber int64           `json:"sequence_number"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"created_at"`
}

type AuctionEventRepository struct {
	db DBTX
}

func NewAuctionEventRepository(
	db DBTX,
) *AuctionEventRepository {
	return &AuctionEventRepository{db: db}
}

func (r *AuctionEventRepository) Create(
	ctx context.Context,
	event *AuctionEvent,
) error {
	query := `
		INSERT INTO auction_events (
			auction_id,
			sequence_number,
			event_type,
			payload
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		event.AuctionID,
		event.SequenceNumber,
		event.EventType,
		event.Payload,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create auction event: %w", err)
	}

	return nil
}

func (r *AuctionEventRepository) GetBySequence(
	ctx context.Context,
	auctionID string,
	sequenceNumber int64,
) (*AuctionEvent, error) {
	query := `
		SELECT id, auction_id, sequence_number,
		       event_type, payload, created_at
		FROM auction_events
		WHERE auction_id = $1 AND sequence_number = $2
	`

	event := &AuctionEvent{}

	err := r.db.QueryRow(
		ctx, query, auctionID, sequenceNumber,
	).Scan(
		&event.ID,
		&event.AuctionID,
		&event.SequenceNumber,
		&event.EventType,
		&event.Payload,
		&event.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auction event: %w", err)
	}

	return event, nil
}

func (r *AuctionEventRepository) GetAfterSequence(
	ctx context.Context,
	auctionID string,
	sequenceNumber int64,
	limit int,
) ([]AuctionEvent, error) {
	query := `
		SELECT id, auction_id, sequence_number,
		       event_type, payload, created_at
		FROM auction_events
		WHERE auction_id = $1
		  AND sequence_number > $2
		ORDER BY sequence_number
		LIMIT $3
	`

	rows, err := r.db.Query(
		ctx, query, auctionID, sequenceNumber, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get auction events: %w", err)
	}
	defer rows.Close()

	events := make([]AuctionEvent, 0)

	for rows.Next() {
		var event AuctionEvent

		err := rows.Scan(
			&event.ID,
			&event.AuctionID,
			&event.SequenceNumber,
			&event.EventType,
			&event.Payload,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan auction event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate auction events: %w", err)
	}

	return events, nil
}
