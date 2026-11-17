package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AuctionSaleRepository struct {
	db DBTX
}

func NewAuctionSaleRepository(db DBTX) *AuctionSaleRepository {
	return &AuctionSaleRepository{db: db}
}

type CompleteSaleParams struct {
	AuctionID    string
	EventPayload json.RawMessage
}

type SaleResult struct {
	SquadID       string
	AuctionID     string
	RoomPlayerID  string
	TeamID        string
	PurchasePrice string
}

func (r *AuctionSaleRepository) CompleteSale(
	ctx context.Context,
	params CompleteSaleParams,
) (*SaleResult, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New("sale workflow requires a transaction")
	}

	if params.AuctionID == "" {
		return nil, errors.New("auction ID is required")
	}

	if len(params.EventPayload) == 0 ||
		!json.Valid(params.EventPayload) {
		return nil, errors.New("invalid sale event payload")
	}

	var (
		roomID       string
		status       string
		roomPlayerID *string
		teamID       *string
		bidAmount    *string
		timerExpired bool
		maxSquadSize int
		maxOverseas  int
	)

	// Lock auction and read its authoritative state.
	err := tx.QueryRow(ctx, `
		SELECT
			a.room_id,
			a.status,
			a.current_player_id,
			a.current_bidder_team_id,
			a.current_bid,
			(
				a.timer_deadline_at IS NOT NULL
				AND a.timer_deadline_at <= NOW()
			),
			r.max_squad_size,
			r.max_overseas_players
		FROM auctions a
		JOIN rooms r ON r.id = a.room_id
		WHERE a.id = $1
		FOR UPDATE OF a
	`, params.AuctionID).Scan(
		&roomID,
		&status,
		&roomPlayerID,
		&teamID,
		&bidAmount,
		&timerExpired,
		&maxSquadSize,
		&maxOverseas,
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

	if roomPlayerID == nil ||
		teamID == nil ||
		bidAmount == nil {
		return nil, errors.New("no accepted bid available for sale")
	}

	if !timerExpired {
		return nil, errors.New("auction timer has not expired")
	}

	// Lock the winning team.
	var remainingPurse string

	err = tx.QueryRow(ctx, `
		SELECT remaining_purse
		FROM teams
		WHERE id = $1
		  AND room_id = $2
		FOR UPDATE
	`, *teamID, roomID).Scan(&remainingPurse)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("winning team not found")
		}
		return nil, fmt.Errorf("get winning team: %w", err)
	}

	// Lock and validate the player.
	var (
		playerStatus   string
		playerOverseas bool
	)

	err = tx.QueryRow(ctx, `
		SELECT
			rp.status,
			p.is_overseas
		FROM room_players rp
		JOIN players p ON p.id = rp.player_id
		WHERE rp.id = $1
		  AND rp.room_id = $2
		FOR UPDATE OF rp
	`, *roomPlayerID, roomID).Scan(
		&playerStatus,
		&playerOverseas,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("room player not found")
		}
		return nil, fmt.Errorf("get room player: %w", err)
	}

	if playerStatus != "IN_AUCTION" {
		return nil, errors.New("player is not in auction")
	}

	// Validate current squad constraints.
	var (
		squadCount    int
		overseasCount int
	)

	err = tx.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE p.is_overseas = TRUE)
		FROM squads s
		JOIN room_players rp
			ON rp.id = s.room_player_id
		   AND rp.room_id = s.room_id
		JOIN players p ON p.id = rp.player_id
		WHERE s.team_id = $1
		  AND s.room_id = $2
	`, *teamID, roomID).Scan(
		&squadCount,
		&overseasCount,
	)

	if err != nil {
		return nil, fmt.Errorf("get squad counts: %w", err)
	}

	if squadCount >= maxSquadSize {
		return nil, errors.New("team squad is full")
	}

	if playerOverseas && overseasCount >= maxOverseas {
		return nil, errors.New("team overseas player limit reached")
	}

	// Verify purse can cover the winning bid.
	var affordable bool

	err = tx.QueryRow(ctx, `
		SELECT $1::numeric >= $2::numeric
	`, remainingPurse, *bidAmount).Scan(&affordable)

	if err != nil {
		return nil, fmt.Errorf("validate winning bid: %w", err)
	}

	if !affordable {
		return nil, errors.New("winning team has insufficient purse")
	}

	// Deduct purse atomically.
	tag, err := tx.Exec(ctx, `
		UPDATE teams
		SET remaining_purse = remaining_purse - $1::numeric
		WHERE id = $2
		  AND room_id = $3
		  AND remaining_purse >= $1::numeric
	`,
		*bidAmount,
		*teamID,
		roomID,
	)

	if err != nil {
		return nil, fmt.Errorf("deduct team purse: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return nil, errors.New("team purse deduction failed")
	}

	// Assign player to winning squad.
	result := &SaleResult{
		AuctionID:     params.AuctionID,
		RoomPlayerID:  *roomPlayerID,
		TeamID:        *teamID,
		PurchasePrice: *bidAmount,
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO squads (
			room_id,
			team_id,
			room_player_id,
			purchase_price,
			acquired_at
		)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`,
		roomID,
		*teamID,
		*roomPlayerID,
		*bidAmount,
	).Scan(&result.SquadID)

	if err != nil {
		return nil, fmt.Errorf("add player to squad: %w", err)
	}

	// Mark player as SOLD.
	tag, err = tx.Exec(ctx, `
		UPDATE room_players
		SET status = 'SOLD',
		    updated_at = NOW()
		WHERE id = $1
		  AND room_id = $2
		  AND status = 'IN_AUCTION'
	`,
		*roomPlayerID,
		roomID,
	)

	if err != nil {
		return nil, fmt.Errorf("mark player sold: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return nil, errors.New("failed to mark player as sold")
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

	// Persist sale event.
	var eventID string

	err = tx.QueryRow(ctx, `
		INSERT INTO auction_events (
			room_id,
			auction_id,
			sequence_number,
			event_type,
			payload
		)
		VALUES ($1, $2, $3, 'PLAYER_SOLD', $4)
		RETURNING id
	`,
		roomID,
		params.AuctionID,
		sequence,
		params.EventPayload,
	).Scan(&eventID)

	if err != nil {
		return nil, fmt.Errorf("insert sale event: %w", err)
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
		VALUES ($1, $2, $3, 'PLAYER_SOLD', $4, 'PENDING')
	`,
		roomID,
		params.AuctionID,
		eventID,
		params.EventPayload,
	)

	if err != nil {
		return nil, fmt.Errorf("insert sale outbox event: %w", err)
	}

	// Clear completed player's auction state.
	_, err = tx.Exec(ctx, `
		UPDATE auctions
		SET current_player_id = NULL,
		    current_bid = NULL,
		    current_bidder_team_id = NULL,
		    timer_started_at = NULL,
		    timer_deadline_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`,
		params.AuctionID,
	)

	if err != nil {
		return nil, fmt.Errorf("clear auction state: %w", err)
	}

	return result, nil
}
