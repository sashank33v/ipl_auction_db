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
) (*Bid, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New("bid workflow requires a transaction")
	}

	if params.AuctionID == "" ||
		params.RoomPlayerID == "" ||
		params.TeamID == "" ||
		params.BidAmount == "" ||
		params.RequestID == "" {
		return nil, errors.New("missing bid details")
	}

	if len(params.EventPayload) == 0 ||
		!json.Valid(params.EventPayload) {
		return nil, errors.New("invalid event payload")
	}

	// Initial idempotency check.
	existingBid, err := r.getExistingBid(
		ctx,
		tx,
		params.AuctionID,
		params.RequestID,
	)
	if err != nil {
		return nil, err
	}

	if existingBid != nil {
		return existingBid, nil
	}

	var (
		roomID          string
		auctionStatus   string
		currentPlayerID *string
		currentBid      *string
		minIncrement    string
		minSquadSize    int
		maxSquadSize    int
		maxOverseas     int
	)

	// Lock auction to serialize simultaneous bids.
	err = tx.QueryRow(ctx, `
		SELECT
			a.room_id,
			a.status,
			a.current_player_id,
			a.current_bid,
			r.min_bid_increment,
			r.min_squad_size,
			r.max_squad_size,
			r.max_overseas_players
		FROM auctions a
		JOIN rooms r ON r.id = a.room_id
		WHERE a.id = $1
		FOR UPDATE OF a
	`, params.AuctionID).Scan(
		&roomID,
		&auctionStatus,
		&currentPlayerID,
		&currentBid,
		&minIncrement,
		&minSquadSize,
		&maxSquadSize,
		&maxOverseas,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("auction not found")
		}
		return nil, fmt.Errorf("get auction state: %w", err)
	}

	// Recheck idempotency after acquiring the auction lock.
	// This handles concurrent retries with the same request ID.
	existingBid, err = r.getExistingBid(
		ctx,
		tx,
		params.AuctionID,
		params.RequestID,
	)
	if err != nil {
		return nil, err
	}

	if existingBid != nil {
		return existingBid, nil
	}

	if auctionStatus != "ACTIVE" {
		return nil, errors.New("auction is not active")
	}

	if currentPlayerID == nil ||
		*currentPlayerID != params.RoomPlayerID {
		return nil, errors.New("player is not currently in auction")
	}

	// Lock and validate team.
	var (
		teamRoomID string
		purse      string
	)

	err = tx.QueryRow(ctx, `
		SELECT room_id, remaining_purse
		FROM teams
		WHERE id = $1
		FOR UPDATE
	`, params.TeamID).Scan(&teamRoomID, &purse)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("team not found")
		}
		return nil, fmt.Errorf("get team: %w", err)
	}

	if teamRoomID != roomID {
		return nil, errors.New("team does not belong to this room")
	}

	// Lock and validate current room player.
	var (
		roomPlayerRoom  string
		playerStatus    string
		playerBasePrice string
		playerOverseas  bool
	)

	err = tx.QueryRow(ctx, `
		SELECT
			rp.room_id,
			rp.status,
			rp.base_price,
			p.is_overseas
		FROM room_players rp
		JOIN players p ON p.id = rp.player_id
		WHERE rp.id = $1
		  AND rp.room_id = $2
		FOR UPDATE OF rp
	`, params.RoomPlayerID, roomID).Scan(
		&roomPlayerRoom,
		&playerStatus,
		&playerBasePrice,
		&playerOverseas,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("room player not found")
		}
		return nil, fmt.Errorf("get room player: %w", err)
	}

	if roomPlayerRoom != roomID ||
		playerStatus != "IN_AUCTION" {
		return nil, errors.New("player is not available for bidding")
	}

	// Get current squad counts.
	var squadCount, overseasCount int

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
	`, params.TeamID, roomID).Scan(
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

	// Validate bid amount.
	var amountOK bool

	if currentBid == nil {
		err = tx.QueryRow(ctx, `
			SELECT
				$1::numeric = $2::numeric
				AND $1::numeric <= $3::numeric
		`,
			params.BidAmount,
			playerBasePrice,
			purse,
		).Scan(&amountOK)
	} else {
		err = tx.QueryRow(ctx, `
			SELECT
				$1::numeric >= $2::numeric + $3::numeric
				AND $1::numeric <= $4::numeric
		`,
			params.BidAmount,
			*currentBid,
			minIncrement,
			purse,
		).Scan(&amountOK)
	}

	if err != nil {
		return nil, fmt.Errorf("validate bid amount: %w", err)
	}

	if !amountOK {
		return nil, errors.New("bid amount is invalid or exceeds purse")
	}

	// Protect the minimum squad size with sufficient remaining purse.
	remainingSlots := minSquadSize - (squadCount + 1)

	if remainingSlots > 0 {
		overseasAfterPurchase := overseasCount

		if playerOverseas {
			overseasAfterPurchase++
		}

		overseasSlots := maxOverseas - overseasAfterPurchase

		if overseasSlots < 0 {
			return nil, errors.New("overseas player limit exceeded")
		}

		var reserveOK bool

		err = tx.QueryRow(ctx, `
			WITH eligible AS (
				SELECT rp.base_price, p.is_overseas
				FROM room_players rp
				JOIN players p ON p.id = rp.player_id
				WHERE rp.room_id = $1
				  AND rp.id <> $2
				  AND rp.status IN ('AVAILABLE', 'UNSOLD')
			),
			domestic AS (
				SELECT base_price
				FROM eligible
				WHERE is_overseas = FALSE
				ORDER BY base_price
			),
			overseas AS (
				SELECT base_price
				FROM eligible
				WHERE is_overseas = TRUE
				ORDER BY base_price
			),
			options AS (
				SELECT g.n AS overseas_needed
				FROM generate_series(
					GREATEST(
						0,
						$3::integer - (SELECT COUNT(*) FROM domestic)
					),
					LEAST(
						$3::integer,
						$4::integer,
						(SELECT COUNT(*) FROM overseas)
					)
				) AS g(n)
			),
			reserves AS (
				SELECT
					(
						SELECT COALESCE(SUM(base_price), 0)
						FROM (
							SELECT base_price
							FROM domestic
							LIMIT ($3 - options.overseas_needed)
						) d
					)
					+
					(
						SELECT COALESCE(SUM(base_price), 0)
						FROM (
							SELECT base_price
							FROM overseas
							LIMIT options.overseas_needed
						) o
					) AS reserve_amount
				FROM options
			)
			SELECT
				EXISTS (SELECT 1 FROM reserves)
				AND $5::numeric - $6::numeric >=
					COALESCE(
						(SELECT MIN(reserve_amount) FROM reserves),
						0
					)
		`,
			roomID,
			params.RoomPlayerID,
			remainingSlots,
			overseasSlots,
			purse,
			params.BidAmount,
		).Scan(&reserveOK)

		if err != nil {
			return nil, fmt.Errorf("calculate squad reserve: %w", err)
		}

		if !reserveOK {
			return nil, errors.New(
				"insufficient purse to complete minimum squad",
			)
		}
	}

	// Generate next bid sequence.
	var bidSequence int64

	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(bid_sequence), 0) + 1
		FROM bids
		WHERE auction_id = $1
	`, params.AuctionID).Scan(&bidSequence)

	if err != nil {
		return nil, fmt.Errorf("get bid sequence: %w", err)
	}

	// Insert accepted bid.
	bid := &Bid{}

	err = tx.QueryRow(ctx, `
		INSERT INTO bids (
			room_id,
			auction_id,
			room_player_id,
			team_id,
			bid_amount,
			request_id,
			bid_sequence
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			auction_id,
			room_player_id,
			team_id,
			bid_amount,
			request_id,
			bid_sequence
	`,
		roomID,
		params.AuctionID,
		params.RoomPlayerID,
		params.TeamID,
		params.BidAmount,
		params.RequestID,
		bidSequence,
	).Scan(
		&bid.ID,
		&bid.AuctionID,
		&bid.RoomPlayerID,
		&bid.TeamID,
		&bid.BidAmount,
		&bid.RequestID,
		&bid.BidSequence,
	)

	if err != nil {
		return nil, fmt.Errorf("insert bid: %w", err)
	}

	// Update current bid and reset seven-second timer.
	_, err = tx.Exec(ctx, `
		UPDATE auctions
		SET current_bid = $1,
		    current_bidder_team_id = $2,
		    timer_started_at = NOW(),
		    timer_deadline_at = NOW() + INTERVAL '7 seconds',
		    updated_at = NOW()
		WHERE id = $3
	`,
		params.BidAmount,
		params.TeamID,
		params.AuctionID,
	)

	if err != nil {
		return nil, fmt.Errorf("update auction: %w", err)
	}

	// Generate independent auction event sequence.
	var eventSequence int64

	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_number), 0) + 1
		FROM auction_events
		WHERE auction_id = $1
	`, params.AuctionID).Scan(&eventSequence)

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
		VALUES ($1, $2, $3, 'BID_ACCEPTED', $4)
		RETURNING id
	`,
		roomID,
		params.AuctionID,
		eventSequence,
		params.EventPayload,
	).Scan(&eventID)

	if err != nil {
		return nil, fmt.Errorf("insert auction event: %w", err)
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
		VALUES ($1, $2, $3, 'BID_ACCEPTED', $4, 'PENDING')
	`,
		roomID,
		params.AuctionID,
		eventID,
		params.EventPayload,
	)

	if err != nil {
		return nil, fmt.Errorf("insert outbox event: %w", err)
	}

	return bid, nil
}

func (r *AuctionWorkflowRepository) getExistingBid(
	ctx context.Context,
	tx pgx.Tx,
	auctionID string,
	requestID string,
) (*Bid, error) {
	bid := &Bid{}

	err := tx.QueryRow(ctx, `
		SELECT
			id,
			auction_id,
			room_player_id,
			team_id,
			bid_amount,
			request_id,
			bid_sequence
		FROM bids
		WHERE auction_id = $1
		  AND request_id = $2
	`,
		auctionID,
		requestID,
	).Scan(
		&bid.ID,
		&bid.AuctionID,
		&bid.RoomPlayerID,
		&bid.TeamID,
		&bid.BidAmount,
		&bid.RequestID,
		&bid.BidSequence,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("check duplicate bid: %w", err)
	}

	return bid, nil
}
