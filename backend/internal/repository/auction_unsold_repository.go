package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AuctionUnsoldRepository struct {
	db DBTX
}

func NewAuctionUnsoldRepository(db DBTX) *AuctionUnsoldRepository {
	return &AuctionUnsoldRepository{db: db}
}

type MarkUnsoldParams struct {
	AuctionID    string
	EventPayload json.RawMessage
}

type UnsoldResult struct {
	AuctionID    string
	RoomPlayerID string
}

func (r *AuctionUnsoldRepository) MarkUnsold(
	ctx context.Context,
	params MarkUnsoldParams,
) (*UnsoldResult, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New("unsold workflow requires a transaction")
	}

	if params.AuctionID == "" {
		return nil, errors.New("auction ID is required")
	}

	if len(params.EventPayload) == 0 ||
		!json.Valid(params.EventPayload) {
		return nil, errors.New("invalid event payload")
	}

	var (
		roomID          string
		status          string
		currentPlayerID *string
		currentBid      *string
		currentBidderID *string
		timerExpired    bool
	)

	// Lock auction and read its current state.
	err := tx.QueryRow(ctx, `
		SELECT
			room_id,
			status,
			current_player_id,
			current_bid,
			current_bidder_team_id,
			(
				timer_deadline_at IS NOT NULL
				AND timer_deadline_at <= clock_timestamp()
			)
		FROM auctions
		WHERE id = $1
		FOR UPDATE
	`, params.AuctionID).Scan(
		&roomID,
		&status,
		&currentPlayerID,
		&currentBid,
		&currentBidderID,
		&timerExpired,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("auction not found")
		}
		return nil, fmt.Errorf("get auction: %w", err)
	}

	if status != "ACTIVE" {
		return nil, errors.New("auction is not active")
	}

	if currentPlayerID == nil {
		return nil, errors.New("no player is currently in auction")
	}

	if currentBid != nil || currentBidderID != nil {
		return nil, errors.New("player has an accepted bid and cannot be marked unsold")
	}

	if !timerExpired {
		return nil, errors.New("auction timer has not expired")
	}

	// Lock and validate current room player.
	var playerStatus string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM room_players
		WHERE id = $1
		  AND room_id = $2
		FOR UPDATE
	`, *currentPlayerID, roomID).Scan(&playerStatus)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("room player not found")
		}
		return nil, fmt.Errorf("get room player: %w", err)
	}

	if playerStatus != "IN_AUCTION" {
		return nil, errors.New("player is not in auction")
	}

	// Mark player UNSOLD.
	tag, err := tx.Exec(ctx, `
		UPDATE room_players
		SET status = 'UNSOLD',
		    updated_at = NOW()
		WHERE id = $1
		  AND room_id = $2
		  AND status = 'IN_AUCTION'
	`,
		*currentPlayerID,
		roomID,
	)

	if err != nil {
		return nil, fmt.Errorf("mark player unsold: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return nil, errors.New("failed to mark player unsold")
	}

	// Allocate next event sequence.
	var sequence int64

	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_number), 0) + 1
		FROM auction_events
		WHERE auction_id = $1
	`, params.AuctionID).Scan(&sequence)

	if err != nil {
		return nil, fmt.Errorf("get event sequence: %w", err)
	}

	// Persist auction event.
	var eventID string

	err = tx.QueryRow(ctx, `
		INSERT INTO auction_events (
			room_id,
			auction_id,
			sequence_number,
			event_type,
			payload
		)
		VALUES ($1, $2, $3, 'PLAYER_UNSOLD', $4)
		RETURNING id
	`,
		roomID,
		params.AuctionID,
		sequence,
		params.EventPayload,
	).Scan(&eventID)

	if err != nil {
		return nil, fmt.Errorf("insert unsold event: %w", err)
	}

	// Persist outbox event.
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			room_id,
			auction_id,
			event_id,
			event_type,
			payload,
			status
		)
		VALUES ($1, $2, $3, 'PLAYER_UNSOLD', $4, 'PENDING')
	`,
		roomID,
		params.AuctionID,
		eventID,
		params.EventPayload,
	)

	if err != nil {
		return nil, fmt.Errorf("insert unsold outbox event: %w", err)
	}

	// Clear completed player's auction state.
	_, err = tx.Exec(ctx, `
		UPDATE auctions
		SET current_player_id = NULL,
		    current_bid = NULL,
		    current_bidder_team_id = NULL,
		    timer_started_at = NULL,
		    timer_deadline_at = NULL,
		    updated_at = statement_timestamp()
		WHERE id = $1
	`,
		params.AuctionID,
	)

	if err != nil {
		return nil, fmt.Errorf("clear auction state: %w", err)
	}

	return &UnsoldResult{
		AuctionID:    params.AuctionID,
		RoomPlayerID: *currentPlayerID,
	}, nil
}
