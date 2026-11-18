package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Auction struct {
	ID                  string     `json:"id"`
	RoomID              string     `json:"room_id"`
	Status              string     `json:"status"`
	CurrentPlayerID     *string    `json:"current_player_id"`
	CurrentBid          *string    `json:"current_bid"`
	CurrentBidderTeamID *string    `json:"current_bidder_team_id"`
	TimerStartedAt      *time.Time `json:"timer_started_at"`
	TimerDeadlineAt     *time.Time `json:"timer_deadline_at"`
	CurrentRoundNumber  int        `json:"current_round_number"`
	RoundType           string     `json:"round_type"`
	PausedAt            *time.Time `json:"paused_at"`
	LastControlledBy    *string    `json:"last_controlled_by"`
	StartedAt           *time.Time `json:"started_at"`
	CompletedAt         *time.Time `json:"completed_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type AuctionRepository struct {
	db DBTX
}

func NewAuctionRepository(db DBTX) *AuctionRepository {
	return &AuctionRepository{db: db}
}

func (r *AuctionRepository) Create(
	ctx context.Context,
	auction *Auction,
) error {
	query := `
		INSERT INTO auctions (
			room_id, status, current_round_number, round_type
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		auction.RoomID,
		auction.Status,
		auction.CurrentRoundNumber,
		auction.RoundType,
	).Scan(&auction.ID, &auction.CreatedAt, &auction.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create auction: %w", err)
	}

	return nil
}

func (r *AuctionRepository) GetByID(
	ctx context.Context,
	id string,
) (*Auction, error) {
	query := `
		SELECT
			id, room_id, status, current_player_id,
			current_bid, current_bidder_team_id,
			timer_started_at, timer_deadline_at,
			current_round_number, round_type, paused_at,
			last_controlled_by, started_at, completed_at,
			created_at, updated_at
		FROM auctions
		WHERE id = $1
	`

	return r.scanAuction(ctx, query, id)
}

func (r *AuctionRepository) GetByRoom(
	ctx context.Context,
	roomID string,
) (*Auction, error) {
	query := `
		SELECT
			id, room_id, status, current_player_id,
			current_bid, current_bidder_team_id,
			timer_started_at, timer_deadline_at,
			current_round_number, round_type, paused_at,
			last_controlled_by, started_at, completed_at,
			created_at, updated_at
		FROM auctions
		WHERE room_id = $1
	`

	return r.scanAuction(ctx, query, roomID)
}

func (r *AuctionRepository) scanAuction(
	ctx context.Context,
	query string,
	args ...any,
) (*Auction, error) {
	auction := &Auction{}

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&auction.ID,
		&auction.RoomID,
		&auction.Status,
		&auction.CurrentPlayerID,
		&auction.CurrentBid,
		&auction.CurrentBidderTeamID,
		&auction.TimerStartedAt,
		&auction.TimerDeadlineAt,
		&auction.CurrentRoundNumber,
		&auction.RoundType,
		&auction.PausedAt,
		&auction.LastControlledBy,
		&auction.StartedAt,
		&auction.CompletedAt,
		&auction.CreatedAt,
		&auction.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}

	return auction, nil
}

func (r *AuctionRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status string,
) error {
	query := `
		UPDATE auctions
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	tag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update auction status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
func (r *AuctionRepository) StartAuction(
	ctx context.Context,
	auctionID string,
) error {
	query := `
        UPDATE auctions
        SET
            status = 'ACTIVE',
            started_at = NOW(),
            updated_at = NOW()
        WHERE id = $1
          AND status = 'WAITING'
    `

	tag, err := r.db.Exec(ctx, query, auctionID)
	if err != nil {
		return fmt.Errorf("failed to start auction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("auction not found or not in waiting state")
	}

	return nil
}
