package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type UnsoldNominationWorkflowRepository struct {
	db DBTX
}

func NewUnsoldNominationWorkflowRepository(
	db DBTX,
) *UnsoldNominationWorkflowRepository {
	return &UnsoldNominationWorkflowRepository{db: db}
}

type CreateNominationParams struct {
	AuctionID    string
	RoomPlayerID string
	TeamID       string
	EventPayload json.RawMessage
}

type NominationResult struct {
	ID           string
	AuctionID    string
	RoomPlayerID string
	TeamID       string
	RoundNumber  int
}

func (r *UnsoldNominationWorkflowRepository) CreateNomination(
	ctx context.Context,
	params CreateNominationParams,
) (*NominationResult, error) {
	tx, ok := r.db.(pgx.Tx)
	if !ok {
		return nil, errors.New("nomination workflow requires a transaction")
	}

	if params.AuctionID == "" ||
		params.RoomPlayerID == "" ||
		params.TeamID == "" {
		return nil, errors.New("missing nomination details")
	}

	if len(params.EventPayload) == 0 ||
		!json.Valid(params.EventPayload) {
		return nil, errors.New("invalid nomination event payload")
	}

	var (
		roomID      string
		status      string
		roundType   string
		roundNumber int
	)

	// Lock auction to serialize nominations within this auction.
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

	if status != "ACTIVE" || roundType != "UNSOLD" {
		return nil, errors.New("auction is not accepting unsold nominations")
	}

	// Validate team belongs to this room.
	var teamExists bool

	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM teams
			WHERE id = $1
			  AND room_id = $2
		)
	`, params.TeamID, roomID).Scan(&teamExists)

	if err != nil {
		return nil, fmt.Errorf("validate team: %w", err)
	}

	if !teamExists {
		return nil, errors.New("team does not belong to this room")
	}

	// Lock and validate the nominated player.
	var playerStatus string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM room_players
		WHERE id = $1
		  AND room_id = $2
		FOR UPDATE
	`, params.RoomPlayerID, roomID).Scan(&playerStatus)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("room player not found")
		}
		return nil, fmt.Errorf("get room player: %w", err)
	}

	if playerStatus != "UNSOLD" {
		return nil, errors.New("only unsold players can be nominated")
	}

	// Enforce maximum six nominations per team per round.
	var nominationCount int

	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM unsold_nominations
		WHERE auction_id = $1
		  AND team_id = $2
		  AND round_number = $3
	`,
		params.AuctionID,
		params.TeamID,
		roundNumber,
	).Scan(&nominationCount)

	if err != nil {
		return nil, fmt.Errorf("count nominations: %w", err)
	}

	if nominationCount >= 6 {
		return nil, errors.New("team has reached six nominations for this round")
	}

	// Insert nomination with room ID.
	result := &NominationResult{
		AuctionID:    params.AuctionID,
		RoomPlayerID: params.RoomPlayerID,
		TeamID:       params.TeamID,
		RoundNumber:  roundNumber,
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO unsold_nominations (
			room_id,
			auction_id,
			room_player_id,
			team_id,
			round_number,
			nominated_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (
			auction_id,
			round_number,
			team_id,
			room_player_id
		) DO NOTHING
		RETURNING id
	`,
		roomID,
		params.AuctionID,
		params.RoomPlayerID,
		params.TeamID,
		roundNumber,
	).Scan(&result.ID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("player already nominated by this team in this round")
		}
		return nil, fmt.Errorf("insert nomination: %w", err)
	}

	// Allocate event sequence.
	var sequence int64

	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_number), 0) + 1
		FROM auction_events
		WHERE auction_id = $1
	`, params.AuctionID).Scan(&sequence)

	if err != nil {
		return nil, fmt.Errorf("get event sequence: %w", err)
	}

	// Persist nomination event.
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
		params.EventPayload,
	).Scan(&eventID)

	if err != nil {
		return nil, fmt.Errorf("insert nomination event: %w", err)
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
		VALUES ($1, $2, $3, 'PLAYER_NOMINATED', $4, 'PENDING')
	`,
		roomID,
		params.AuctionID,
		eventID,
		params.EventPayload,
	)

	if err != nil {
		return nil, fmt.Errorf("insert nomination outbox event: %w", err)
	}

	return result, nil
}
