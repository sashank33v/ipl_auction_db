package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Bid struct {
	ID           string    `json:"id"`
	AuctionID    string    `json:"auction_id"`
	RoomPlayerID string    `json:"room_player_id"`
	TeamID       string    `json:"team_id"`
	BidAmount    string    `json:"bid_amount"`
	RequestID    string    `json:"request_id"`
	BidSequence  int64     `json:"bid_sequence"`
	CreatedAt    time.Time `json:"created_at"`
}

type BidRepository struct {
	db DBTX
}

func NewBidRepository(db DBTX) *BidRepository {
	return &BidRepository{db: db}
}

func (r *BidRepository) Create(
	ctx context.Context,
	bid *Bid,
) error {
	query := `
		INSERT INTO bids (
			auction_id,
			room_player_id,
			team_id,
			bid_amount,
			request_id,
			bid_sequence
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		bid.AuctionID,
		bid.RoomPlayerID,
		bid.TeamID,
		bid.BidAmount,
		bid.RequestID,
		bid.BidSequence,
	).Scan(&bid.ID, &bid.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create bid: %w", err)
	}

	return nil
}

func (r *BidRepository) GetByID(
	ctx context.Context,
	id string,
) (*Bid, error) {
	query := `
		SELECT id, auction_id, room_player_id,
		       team_id, bid_amount, request_id,
		       bid_sequence, created_at
		FROM bids
		WHERE id = $1
	`

	bid := &Bid{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&bid.ID,
		&bid.AuctionID,
		&bid.RoomPlayerID,
		&bid.TeamID,
		&bid.BidAmount,
		&bid.RequestID,
		&bid.BidSequence,
		&bid.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get bid: %w", err)
	}

	return bid, nil
}

func (r *BidRepository) GetByRequestID(
	ctx context.Context,
	auctionID string,
	requestID string,
) (*Bid, error) {
	query := `
		SELECT id, auction_id, room_player_id,
		       team_id, bid_amount, request_id,
		       bid_sequence, created_at
		FROM bids
		WHERE auction_id = $1 AND request_id = $2
	`

	bid := &Bid{}

	err := r.db.QueryRow(
		ctx, query, auctionID, requestID,
	).Scan(
		&bid.ID,
		&bid.AuctionID,
		&bid.RoomPlayerID,
		&bid.TeamID,
		&bid.BidAmount,
		&bid.RequestID,
		&bid.BidSequence,
		&bid.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get bid by request ID: %w", err)
	}

	return bid, nil
}

func (r *BidRepository) GetByAuction(
	ctx context.Context,
	auctionID string,
) ([]Bid, error) {
	query := `
		SELECT id, auction_id, room_player_id,
		       team_id, bid_amount, request_id,
		       bid_sequence, created_at
		FROM bids
		WHERE auction_id = $1
		ORDER BY bid_sequence
	`

	rows, err := r.db.Query(ctx, query, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auction bids: %w", err)
	}
	defer rows.Close()

	bids := make([]Bid, 0)

	for rows.Next() {
		var bid Bid

		err := rows.Scan(
			&bid.ID,
			&bid.AuctionID,
			&bid.RoomPlayerID,
			&bid.TeamID,
			&bid.BidAmount,
			&bid.RequestID,
			&bid.BidSequence,
			&bid.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}

		bids = append(bids, bid)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate bids: %w", err)
	}

	return bids, nil
}
