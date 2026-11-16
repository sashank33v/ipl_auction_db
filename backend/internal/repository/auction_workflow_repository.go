package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AuctionWorkflowRepository struct {
	db DBTX
}

func NewAuctionWorkflowRepository(db DBTX) *AuctionWorkflowRepository {
	return &AuctionWorkflowRepository{db: db}
}

type AcceptBidParams struct {
	AuctionID    string
	RoomPlayerID string
	TeamID       string
	BidAmount    string
	RequestID    string
	EventPayload json.RawMessage
}

func (r *AuctionWorkflowRepository) AcceptBid(
	ctx context.Context,
	params AcceptBidParams,
) error {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return errors.New("bid workflow requires a transaction")
	}

	var (
		roomID          string
		auctionStatus   string
		currentPlayerID *string
		currentBid      *string
		teamRoomID      string
		purse           string
		roomPlayerRoom  string
		playerStatus    string
		minIncrement    string
		maxSquadSize    int
		maxOverseas     int
	)

	err := tx.QueryRow(ctx, `
		SELECT a.room_id, a.status, a.current_player_id,
		       a.current_bid, r.min_bid_increment,
		       r.max_squad_size, r.max_overseas_players
		FROM auctions a
		JOIN rooms r ON r.id = a.room_id
		WHERE a.id = $1
		FOR UPDATE OF a
	`, params.AuctionID).Scan(
		&roomID, &auctionStatus, &currentPlayerID,
		&currentBid, &minIncrement, &maxSquadSize, &maxOverseas,
	)
	if err != nil {
		return fmt.Errorf("get auction state: %w", err)
	}

	if auctionStatus != "LIVE" {
		return errors.New("auction is not live")
	}
	if currentPlayerID == nil || *currentPlayerID != params.RoomPlayerID {
		return errors.New("player is not currently in auction")
	}

	// Idempotency check.
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM bids
		WHERE auction_id = $1 AND request_id = $2
	`, params.AuctionID, params.RequestID).Scan(&existingID)

	if err == nil {
		return errors.New("bid request already processed")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check duplicate bid: %w", err)
	}

	err = tx.QueryRow(ctx, `
		SELECT room_id, remaining_purse
		FROM teams
		WHERE id = $1
		FOR UPDATE
	`, params.TeamID).Scan(&teamRoomID, &purse)
	if err != nil {
		return fmt.Errorf("get bidding team: %w", err)
	}

	if teamRoomID != roomID {
		return errors.New("team does not belong to this room")
	}

	err = tx.QueryRow(ctx, `
		SELECT rp.room_id, rp.status, rp.base_price,
		       p.is_overseas
		FROM room_players rp
		JOIN players p ON p.id = rp.player_id
		WHERE rp.id = $1
		FOR UPDATE OF rp
	`, params.RoomPlayerID).Scan(
		&roomPlayerRoom, &playerStatus,
		new(string), new(bool),
	)
	if err != nil {
		return fmt.Errorf("get room player: %w", err)
	}

	if roomPlayerRoom != roomID || playerStatus != "IN_AUCTION" {
		return errors.New("player is not available for bidding")
	}

	var currentAmount string
	if currentBid != nil {
		currentAmount = *currentBid
	} else {
		err = tx.QueryRow(ctx, `
			SELECT base_price FROM room_players WHERE id = $1
		`, params.RoomPlayerID).Scan(&currentAmount)
		if err != nil {
			return fmt.Errorf("get base price: %w", err)
		}
	}

	var squadSize int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM squads WHERE team_id = $1
	`, params.TeamID).Scan(&squadSize)
	if err != nil {
		return fmt.Errorf("count squad: %w", err)
	}

	if squadSize >= maxSquadSize {
		return errors.New("team squad is full")
	}

	var amountOK bool
	err = tx.QueryRow(ctx, `
		SELECT $1::numeric >= $2::numeric + $3::numeric
		   AND $1::numeric <= $4::numeric
	`, params.BidAmount, currentAmount, minIncrement, purse).Scan(&amountOK)
	if err != nil {
		return fmt.Errorf("validate bid amount: %w", err)
	}
	if !amountOK {
		return errors.New("bid amount is invalid or exceeds remaining purse")
	}

	// Allocate the next bid sequence while auction row is locked.
	var sequence int64
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(bid_sequence), 0) + 1
		FROM bids WHERE auction_id = $1
	`, params.AuctionID).Scan(&sequence)
	if err != nil {
		return fmt.Errorf("get bid sequence: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO bids (
			auction_id, room_player_id, team_id,
			bid_amount, request_id, bid_sequence
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		params.AuctionID, params.RoomPlayerID,
		params.TeamID, params.BidAmount,
		params.RequestID, sequence,
	)
	if err != nil {
		return fmt.Errorf("insert bid: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE auctions
		SET current_bid = $1,
		    current_bidder_team_id = $2,
		    timer_started_at = NOW(),
		    timer_deadline_at = NOW() + INTERVAL '7 seconds',
		    updated_at = NOW()
		WHERE id = $3
	`, params.BidAmount, params.TeamID, params.AuctionID)
	if err != nil {
		return fmt.Errorf("update auction: %w", err)
	}

	var eventID string
	err = tx.QueryRow(ctx, `
		INSERT INTO auction_events (
			auction_id, sequence_number, event_type, payload
		)
		VALUES ($1, $2, 'BID_ACCEPTED', $3)
		RETURNING id
	`, params.AuctionID, sequence, params.EventPayload).Scan(&eventID)
	if err != nil {
		return fmt.Errorf("insert auction event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			auction_id, event_id, event_type, payload, status
		)
		VALUES ($1, $2, 'BID_ACCEPTED', $3, 'PENDING')
	`, params.AuctionID, eventID, params.EventPayload)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}
