package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AuctionCompletionRepository struct {
	db DBTX
}

func NewAuctionCompletionRepository(
	db DBTX,
) *AuctionCompletionRepository {
	return &AuctionCompletionRepository{db: db}
}

type CompleteAuctionParams struct {
	AuctionID    string
	EventPayload json.RawMessage
}

type AuctionCompletionResult struct {
	AuctionID string
	RoomID    string
	Status    string
}

func (r *AuctionCompletionRepository) CompleteAuction(
	ctx context.Context,
	params CompleteAuctionParams,
) (*AuctionCompletionResult, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New("auction completion requires a transaction")
	}

	if params.AuctionID == "" {
		return nil, errors.New("auction ID is required")
	}

	if len(params.EventPayload) == 0 ||
		!json.Valid(params.EventPayload) {
		return nil, errors.New("invalid auction completion payload")
	}

	var (
		roomID          string
		status          string
		roundType       string
		currentPlayerID *string
		currentBid      *string
		currentBidderID *string
	)

	// Lock auction and read its current state.
	err := tx.QueryRow(ctx, `
		SELECT
			room_id,
			status,
			round_type,
			current_player_id,
			current_bid,
			current_bidder_team_id
		FROM auctions
		WHERE id = $1
		FOR UPDATE
	`, params.AuctionID).Scan(
		&roomID,
		&status,
		&roundType,
		&currentPlayerID,
		&currentBid,
		&currentBidderID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("auction not found")
		}
		return nil, fmt.Errorf("lock auction: %w", err)
	}

	if status != "ACTIVE" {
		return nil, errors.New("auction is not active")
	}

	if currentPlayerID != nil ||
		currentBid != nil ||
		currentBidderID != nil {
		return nil, errors.New("auction still has an active player or bid")
	}

	if roundType != "MAIN" && roundType != "UNSOLD" {
		return nil, errors.New("invalid auction round type")
	}

	// Lock room and read squad constraints.
	var (
		roomStatus   string
		minSquadSize int
		maxSquadSize int
		maxOverseas  int
	)

	err = tx.QueryRow(ctx, `
		SELECT
			status,
			min_squad_size,
			max_squad_size,
			max_overseas_players
		FROM rooms
		WHERE id = $1
		FOR UPDATE
	`, roomID).Scan(
		&roomStatus,
		&minSquadSize,
		&maxSquadSize,
		&maxOverseas,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("room not found")
		}
		return nil, fmt.Errorf("lock room: %w", err)
	}

	if roomStatus != "IN_PROGRESS" {
		return nil, errors.New("room is not in progress")
	}

	// Validate team count.
	var teamCount int

	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM teams
		WHERE room_id = $1
	`, roomID).Scan(&teamCount)

	if err != nil {
		return nil, fmt.Errorf("count teams: %w", err)
	}

	if teamCount < 4 || teamCount > 10 {
		return nil, errors.New("room must have between 4 and 10 teams")
	}

	// Lock all teams before validating their squads.
	teamRows, err := tx.Query(ctx, `
		SELECT id, remaining_purse
		FROM teams
		WHERE room_id = $1
		ORDER BY id
		FOR UPDATE
	`, roomID)

	if err != nil {
		return nil, fmt.Errorf("lock teams: %w", err)
	}

	type teamInfo struct {
		id             string
		remainingPurse string
	}

	var teams []teamInfo

	for teamRows.Next() {
		var team teamInfo

		if err := teamRows.Scan(
			&team.id,
			&team.remainingPurse,
		); err != nil {
			teamRows.Close()
			return nil, fmt.Errorf("scan team: %w", err)
		}

		teams = append(teams, team)
	}

	if err := teamRows.Err(); err != nil {
		teamRows.Close()
		return nil, fmt.Errorf("iterate teams: %w", err)
	}

	teamRows.Close()

	// Validate every team's squad constraints.
	for _, team := range teams {
		var (
			squadSize    int
			overseasSize int
		)

		err = tx.QueryRow(ctx, `
			SELECT
				COUNT(s.id),
				COUNT(s.id) FILTER (
					WHERE p.is_overseas = TRUE
				)
			FROM squads s
			JOIN room_players rp
				ON rp.id = s.room_player_id
			   AND rp.room_id = s.room_id
			JOIN players p ON p.id = rp.player_id
			WHERE s.team_id = $1
			  AND s.room_id = $2
		`, team.id, roomID).Scan(
			&squadSize,
			&overseasSize,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"get squad counts for team %s: %w",
				team.id,
				err,
			)
		}

		if squadSize < minSquadSize {
			return nil, fmt.Errorf(
				"team %s has only %d players; minimum is %d",
				team.id,
				squadSize,
				minSquadSize,
			)
		}

		if squadSize > maxSquadSize {
			return nil, fmt.Errorf(
				"team %s exceeds maximum squad size of %d",
				team.id,
				maxSquadSize,
			)
		}

		if overseasSize > maxOverseas {
			return nil, fmt.Errorf(
				"team %s exceeds overseas player limit of %d",
				team.id,
				maxOverseas,
			)
		}

		if team.remainingPurse == "" {
			return nil, fmt.Errorf(
				"invalid remaining purse for team %s",
				team.id,
			)
		}
	}

	// Ensure no player remains IN_AUCTION.
	var activePlayers int

	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM room_players
		WHERE room_id = $1
		  AND status = 'IN_AUCTION'
	`, roomID).Scan(&activePlayers)

	if err != nil {
		return nil, fmt.Errorf("check active players: %w", err)
	}

	if activePlayers > 0 {
		return nil, errors.New("auction still has a player in progress")
	}

	// Complete auction.
	tag, err := tx.Exec(ctx, `
		UPDATE auctions
		SET status = 'COMPLETED',
		    completed_at = NOW(),
		    current_player_id = NULL,
		    current_bid = NULL,
		    current_bidder_team_id = NULL,
		    timer_started_at = NULL,
		    timer_deadline_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'ACTIVE'
	`, params.AuctionID)

	if err != nil {
		return nil, fmt.Errorf("complete auction: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return nil, errors.New("failed to complete auction")
	}

	// Complete room.
	tag, err = tx.Exec(ctx, `
		UPDATE rooms
		SET status = 'COMPLETED',
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'IN_PROGRESS'
	`, roomID)

	if err != nil {
		return nil, fmt.Errorf("complete room: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return nil, errors.New("failed to complete room")
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

	// Persist completion event.
	var eventID string

	err = tx.QueryRow(ctx, `
		INSERT INTO auction_events (
			room_id,
			auction_id,
			sequence_number,
			event_type,
			payload
		)
		VALUES ($1, $2, $3, 'AUCTION_COMPLETED', $4)
		RETURNING id
	`,
		roomID,
		params.AuctionID,
		sequence,
		params.EventPayload,
	).Scan(&eventID)

	if err != nil {
		return nil, fmt.Errorf("insert completion event: %w", err)
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
		VALUES ($1, $2, $3, 'AUCTION_COMPLETED', $4, 'PENDING')
	`,
		roomID,
		params.AuctionID,
		eventID,
		params.EventPayload,
	)

	if err != nil {
		return nil, fmt.Errorf("insert completion outbox event: %w", err)
	}

	return &AuctionCompletionResult{
		AuctionID: params.AuctionID,
		RoomID:    roomID,
		Status:    "COMPLETED",
	}, nil
}
