package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type SquadPlayer struct {
	ID            string    `json:"id"`
	RoomID        string    `json:"room_id"`
	TeamID        string    `json:"team_id"`
	RoomPlayerID  string    `json:"room_player_id"`
	PurchasePrice string    `json:"purchase_price"`
	AcquiredAt    time.Time `json:"acquired_at"`
}

type SquadRepository struct {
	db DBTX
}

func NewSquadRepository(db DBTX) *SquadRepository {
	return &SquadRepository{db: db}
}

func (r *SquadRepository) Create(
	ctx context.Context,
	player *SquadPlayer,
) error {
	query := `
		INSERT INTO squads (
			room_id, team_id, room_player_id, purchase_price
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, acquired_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		player.RoomID,
		player.TeamID,
		player.RoomPlayerID,
		player.PurchasePrice,
	).Scan(&player.ID, &player.AcquiredAt)

	if err != nil {
		return fmt.Errorf("failed to add player to squad: %w", err)
	}

	return nil
}

func (r *SquadRepository) GetByID(
	ctx context.Context,
	id string,
) (*SquadPlayer, error) {
	query := `
		SELECT id, room_id, team_id,
		       room_player_id, purchase_price, acquired_at
		FROM squads
		WHERE id = $1
	`

	player := &SquadPlayer{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&player.ID,
		&player.RoomID,
		&player.TeamID,
		&player.RoomPlayerID,
		&player.PurchasePrice,
		&player.AcquiredAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get squad player: %w", err)
	}

	return player, nil
}

func (r *SquadRepository) GetByTeam(
	ctx context.Context,
	teamID string,
) ([]SquadPlayer, error) {
	query := `
		SELECT id, room_id, team_id,
		       room_player_id, purchase_price, acquired_at
		FROM squads
		WHERE team_id = $1
		ORDER BY acquired_at
	`

	rows, err := r.db.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team squad: %w", err)
	}
	defer rows.Close()

	players := make([]SquadPlayer, 0)

	for rows.Next() {
		var player SquadPlayer

		err := rows.Scan(
			&player.ID,
			&player.RoomID,
			&player.TeamID,
			&player.RoomPlayerID,
			&player.PurchasePrice,
			&player.AcquiredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan squad player: %w", err)
		}

		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate squad players: %w", err)
	}

	return players, nil
}

func (r *SquadRepository) GetByRoom(
	ctx context.Context,
	roomID string,
) ([]SquadPlayer, error) {
	query := `
		SELECT id, room_id, team_id,
		       room_player_id, purchase_price, acquired_at
		FROM squads
		WHERE room_id = $1
		ORDER BY acquired_at
	`

	rows, err := r.db.Query(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get room squads: %w", err)
	}
	defer rows.Close()

	players := make([]SquadPlayer, 0)

	for rows.Next() {
		var player SquadPlayer

		err := rows.Scan(
			&player.ID,
			&player.RoomID,
			&player.TeamID,
			&player.RoomPlayerID,
			&player.PurchasePrice,
			&player.AcquiredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan squad player: %w", err)
		}

		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate room squads: %w", err)
	}

	return players, nil
}
