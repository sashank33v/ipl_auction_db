package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type UnsoldNomination struct {
	ID           string    `json:"id"`
	AuctionID    string    `json:"auction_id"`
	RoomPlayerID string    `json:"room_player_id"`
	TeamID       string    `json:"team_id"`
	RoundNumber  int       `json:"round_number"`
	NominatedAt  time.Time `json:"nominated_at"`
}

type UnsoldNominationRepository struct {
	db DBTX
}

func NewUnsoldNominationRepository(
	db DBTX,
) *UnsoldNominationRepository {
	return &UnsoldNominationRepository{db: db}
}

func (r *UnsoldNominationRepository) Create(
	ctx context.Context,
	nomination *UnsoldNomination,
) error {
	query := `
		INSERT INTO unsold_nominations (
			auction_id,
			room_player_id,
			team_id,
			round_number
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, nominated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		nomination.AuctionID,
		nomination.RoomPlayerID,
		nomination.TeamID,
		nomination.RoundNumber,
	).Scan(&nomination.ID, &nomination.NominatedAt)

	if err != nil {
		return fmt.Errorf("failed to create nomination: %w", err)
	}

	return nil
}

func (r *UnsoldNominationRepository) GetByID(
	ctx context.Context,
	id string,
) (*UnsoldNomination, error) {
	query := `
		SELECT id, auction_id, room_player_id,
		       team_id, round_number, nominated_at
		FROM unsold_nominations
		WHERE id = $1
	`

	nomination := &UnsoldNomination{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&nomination.ID,
		&nomination.AuctionID,
		&nomination.RoomPlayerID,
		&nomination.TeamID,
		&nomination.RoundNumber,
		&nomination.NominatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get nomination: %w", err)
	}

	return nomination, nil
}

func (r *UnsoldNominationRepository) GetByRound(
	ctx context.Context,
	auctionID string,
	roundNumber int,
) ([]UnsoldNomination, error) {
	query := `
		SELECT id, auction_id, room_player_id,
		       team_id, round_number, nominated_at
		FROM unsold_nominations
		WHERE auction_id = $1 AND round_number = $2
		ORDER BY nominated_at
	`

	rows, err := r.db.Query(ctx, query, auctionID, roundNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get round nominations: %w", err)
	}
	defer rows.Close()

	nominations := make([]UnsoldNomination, 0)

	for rows.Next() {
		var nomination UnsoldNomination

		err := rows.Scan(
			&nomination.ID,
			&nomination.AuctionID,
			&nomination.RoomPlayerID,
			&nomination.TeamID,
			&nomination.RoundNumber,
			&nomination.NominatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nomination: %w", err)
		}

		nominations = append(nominations, nomination)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate nominations: %w", err)
	}

	return nominations, nil
}

func (r *UnsoldNominationRepository) GetByTeamAndRound(
	ctx context.Context,
	auctionID string,
	teamID string,
	roundNumber int,
) ([]UnsoldNomination, error) {
	query := `
		SELECT id, auction_id, room_player_id,
		       team_id, round_number, nominated_at
		FROM unsold_nominations
		WHERE auction_id = $1
		  AND team_id = $2
		  AND round_number = $3
		ORDER BY nominated_at
	`

	rows, err := r.db.Query(ctx, query, auctionID, teamID, roundNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get team nominations: %w", err)
	}
	defer rows.Close()

	nominations := make([]UnsoldNomination, 0)

	for rows.Next() {
		var nomination UnsoldNomination

		err := rows.Scan(
			&nomination.ID,
			&nomination.AuctionID,
			&nomination.RoomPlayerID,
			&nomination.TeamID,
			&nomination.RoundNumber,
			&nomination.NominatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nomination: %w", err)
		}

		nominations = append(nominations, nomination)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate nominations: %w", err)
	}

	return nominations, nil
}
