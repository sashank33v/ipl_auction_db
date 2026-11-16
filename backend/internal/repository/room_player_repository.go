package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type RoomPlayer struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	PlayerID  string    `json:"player_id"`
	BasePrice string    `json:"base_price"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoomPlayerRepository struct {
	db DBTX
}

func NewRoomPlayerRepository(db DBTX) *RoomPlayerRepository {
	return &RoomPlayerRepository{db: db}
}

func (r *RoomPlayerRepository) Create(
	ctx context.Context,
	player *RoomPlayer,
) error {
	query := `
		INSERT INTO room_players (
			room_id, player_id, base_price, status
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		player.RoomID,
		player.PlayerID,
		player.BasePrice,
		player.Status,
	).Scan(
		&player.ID,
		&player.CreatedAt,
		&player.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create room player: %w", err)
	}

	return nil
}

func (r *RoomPlayerRepository) GetByID(
	ctx context.Context,
	id string,
) (*RoomPlayer, error) {
	query := `
		SELECT id, room_id, player_id, base_price,
		       status, created_at, updated_at
		FROM room_players
		WHERE id = $1
	`

	player := &RoomPlayer{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&player.ID,
		&player.RoomID,
		&player.PlayerID,
		&player.BasePrice,
		&player.Status,
		&player.CreatedAt,
		&player.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get room player: %w", err)
	}

	return player, nil
}

func (r *RoomPlayerRepository) GetByRoomAndPlayer(
	ctx context.Context,
	roomID string,
	playerID string,
) (*RoomPlayer, error) {
	query := `
		SELECT id, room_id, player_id, base_price,
		       status, created_at, updated_at
		FROM room_players
		WHERE room_id = $1 AND player_id = $2
	`

	player := &RoomPlayer{}

	err := r.db.QueryRow(ctx, query, roomID, playerID).Scan(
		&player.ID,
		&player.RoomID,
		&player.PlayerID,
		&player.BasePrice,
		&player.Status,
		&player.CreatedAt,
		&player.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get room player: %w", err)
	}

	return player, nil
}

func (r *RoomPlayerRepository) GetByRoom(
	ctx context.Context,
	roomID string,
) ([]RoomPlayer, error) {
	query := `
		SELECT id, room_id, player_id, base_price,
		       status, created_at, updated_at
		FROM room_players
		WHERE room_id = $1
		ORDER BY created_at
	`

	return r.getPlayers(ctx, query, roomID)
}

func (r *RoomPlayerRepository) GetByRoomAndStatus(
	ctx context.Context,
	roomID string,
	status string,
) ([]RoomPlayer, error) {
	query := `
		SELECT id, room_id, player_id, base_price,
		       status, created_at, updated_at
		FROM room_players
		WHERE room_id = $1 AND status = $2
		ORDER BY created_at
	`

	return r.getPlayers(ctx, query, roomID, status)
}

func (r *RoomPlayerRepository) getPlayers(
	ctx context.Context,
	query string,
	args ...any,
) ([]RoomPlayer, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get room players: %w", err)
	}
	defer rows.Close()

	players := make([]RoomPlayer, 0)

	for rows.Next() {
		var player RoomPlayer

		err := rows.Scan(
			&player.ID,
			&player.RoomID,
			&player.PlayerID,
			&player.BasePrice,
			&player.Status,
			&player.CreatedAt,
			&player.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room player: %w", err)
		}

		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate room players: %w", err)
	}

	return players, nil
}

func (r *RoomPlayerRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status string,
) error {
	query := `
		UPDATE room_players
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	tag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update room player status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
