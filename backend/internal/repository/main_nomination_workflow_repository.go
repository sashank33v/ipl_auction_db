package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type MainNominationWorkflowRepository struct {
	db DBTX
}

func NewMainNominationWorkflowRepository(
	db DBTX,
) *MainNominationWorkflowRepository {
	return &MainNominationWorkflowRepository{
		db: db,
	}
}

type MainNominationParams struct {
	AuctionID string
	PlayerID  string // room_players.id
	UserID    string
}

type MainNominationResult struct {
	AuctionID     string `json:"auction_id"`
	RoomID        string `json:"room_id"`
	RoomPlayerID  string `json:"room_player_id"`
	PlayerID      string `json:"player_id"`
	PlayerName    string `json:"player_name"`
	BasePrice     string `json:"base_price"`
	Status        string `json:"status"`
	RoundNumber   int    `json:"round_number"`
	RoundType     string `json:"round_type"`
	TimerDuration int    `json:"timer_duration_seconds"`
}

func (r *MainNominationWorkflowRepository) NominatePlayer(
	ctx context.Context,
	params MainNominationParams,
) (*MainNominationResult, error) {

	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New(
			"main nomination workflow requires a transaction",
		)
	}

	if params.AuctionID == "" ||
		params.PlayerID == "" ||
		params.UserID == "" {
		return nil, errors.New("missing nomination details")
	}

	// 1. Lock auction and validate its state.
	var (
		roomID      string
		status      string
		roundType   string
		roundNumber int
	)

	err := tx.QueryRow(ctx, `
		SELECT
			room_id,
			status,
			round_type,
			current_round_number
		FROM auctions
		WHERE id = $1
		FOR UPDATE
	`, params.AuctionID).Scan(
		&roomID,
		&status,
		&roundType,
		&roundNumber,
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

	if roundType != "MAIN" {
		return nil, errors.New(
			"auction is not accepting main nominations",
		)
	}

	// 2. Verify that the requester is the room host.
	var isHost bool

	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM rooms
			WHERE id = $1
			  AND host_user_id = $2
		)
	`, roomID, params.UserID).Scan(&isHost)

	if err != nil {
		return nil, fmt.Errorf("verify room host: %w", err)
	}

	if !isHost {
		return nil, errors.New(
			"only the room host can nominate players",
		)
	}

	// 3. Prevent nomination while another player is active.
	var currentPlayerID *string

	err = tx.QueryRow(ctx, `
		SELECT current_player_id
		FROM auctions
		WHERE id = $1
	`, params.AuctionID).Scan(&currentPlayerID)

	if err != nil {
		return nil, fmt.Errorf("get current player: %w", err)
	}

	if currentPlayerID != nil {
		return nil, errors.New(
			"another player is already in auction",
		)
	}

	// Generated main queues are authoritative. Existing unsequenced legacy auctions retain manual nomination.
	var nextID string
	err = tx.QueryRow(ctx, `SELECT id FROM room_players WHERE room_id=$1 AND status='AVAILABLE' AND auction_sequence IS NOT NULL ORDER BY auction_sequence,id LIMIT 1`, roomID).Scan(&nextID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("read next queue player: %w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var generated bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM room_players WHERE room_id=$1 AND auction_sequence IS NOT NULL)`, roomID).Scan(&generated); err != nil {
			return nil, err
		}
		if generated {
			return nil, errors.New("no available queued players remain")
		}
	}
	if err == nil && nextID != params.PlayerID {
		return nil, errors.New("only the next available queued player can be nominated")
	}

	// 4. Lock and validate the room player.
	var (
		masterPlayerID string
		playerName     string
		basePrice      string
		playerStatus   string
	)

	err = tx.QueryRow(ctx, `
		SELECT
			rp.player_id,
			p.full_name,
			rp.base_price::text,
			rp.status
		FROM room_players rp
		JOIN players p ON p.id = rp.player_id
		WHERE rp.id = $1
		  AND rp.room_id = $2
		FOR UPDATE OF rp
	`, params.PlayerID, roomID).Scan(
		&masterPlayerID,
		&playerName,
		&basePrice,
		&playerStatus,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("room player not found")
		}
		return nil, fmt.Errorf("get room player: %w", err)
	}

	if playerStatus != "AVAILABLE" {
		return nil, errors.New(
			"only available players can be nominated",
		)
	}

	// 5. Change player status to IN_AUCTION.
	tag, err := tx.Exec(ctx, `
		UPDATE room_players
		SET status = 'IN_AUCTION',
		    updated_at = NOW()
		WHERE id = $1
		  AND room_id = $2
		  AND status = 'AVAILABLE'
	`, params.PlayerID, roomID)

	if err != nil {
		return nil, fmt.Errorf(
			"update room player status: %w",
			err,
		)
	}

	if tag.RowsAffected() != 1 {
		return nil, errors.New(
			"player is no longer available",
		)
	}

	// 6. Update auction and start the 15-second timer.
	var deadline time.Time
	err = tx.QueryRow(ctx, `
		UPDATE auctions
		SET current_player_id = $1,
		    current_bid = NULL,
		    current_bidder_team_id = NULL,
		    timer_started_at = statement_timestamp(),
		    timer_deadline_at = statement_timestamp() + INTERVAL '15 seconds',
		    updated_at = statement_timestamp()
		WHERE id = $2
        RETURNING timer_deadline_at
	`,
		params.PlayerID,
		params.AuctionID,
	).Scan(&deadline)

	if err != nil {
		return nil, fmt.Errorf("update auction: %w", err)
	}

	// 7. Build nomination event payload.
	payload, err := json.Marshal(map[string]interface{}{
		"auction_id":        params.AuctionID,
		"room_id":           roomID,
		"room_player_id":    params.PlayerID,
		"player_id":         masterPlayerID,
		"player_name":       playerName,
		"base_price":        basePrice,
		"status":            "IN_AUCTION",
		"round_number":      roundNumber,
		"round_type":        roundType,
		"timer_seconds":     15,
		"timer_deadline_at": deadline,
	})

	if err != nil {
		return nil, fmt.Errorf(
			"marshal nomination event: %w",
			err,
		)
	}

	// 8. Allocate event sequence.
	var sequence int64

	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_number), 0) + 1
		FROM auction_events
		WHERE auction_id = $1
	`, params.AuctionID).Scan(&sequence)

	if err != nil {
		return nil, fmt.Errorf(
			"get event sequence: %w",
			err,
		)
	}

	// 9. Persist auction event.
	var eventID string

	err = tx.QueryRow(ctx, `
		INSERT INTO auction_events (
			room_id,
			auction_id,
			sequence_number,
			event_type,
			payload
		)
		VALUES ($1, $2, $3, 'PLAYER_NOMINATED', $4)
		RETURNING id
	`,
		roomID,
		params.AuctionID,
		sequence,
		payload,
	).Scan(&eventID)

	if err != nil {
		return nil, fmt.Errorf(
			"insert auction event: %w",
			err,
		)
	}

	// 10. Persist transactional outbox event.
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			room_id,
			auction_id,
			event_id,
			event_type,
			payload,
			status
		)
		VALUES ($1, $2, $3, 'PLAYER_NOMINATED', $4, 'PENDING')
	`,
		roomID,
		params.AuctionID,
		eventID,
		payload,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"insert outbox event: %w",
			err,
		)
	}

	return &MainNominationResult{
		AuctionID:     params.AuctionID,
		RoomID:        roomID,
		RoomPlayerID:  params.PlayerID,
		PlayerID:      masterPlayerID,
		PlayerName:    playerName,
		BasePrice:     basePrice,
		Status:        "IN_AUCTION",
		RoundNumber:   roundNumber,
		RoundType:     roundType,
		TimerDuration: 15,
	}, nil
}
