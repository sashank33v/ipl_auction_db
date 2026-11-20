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
	FullName  string    `json:"full_name"`
	BasePrice string    `json:"base_price"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type RoomPlayerQueueItem struct {
	RoomPlayerID    string `json:"room_player_id"`
	RoomID          string `json:"room_id"`
	PlayerID        string `json:"player_id"`
	FullName        string `json:"full_name"`
	Country         string `json:"country"`
	Role            string `json:"role"`
	IsOverseas      bool   `json:"is_overseas"`
	BasePrice       string `json:"base_price"`
	AuctionCategory string `json:"auction_category"`
	Status          string `json:"status"`
	AuctionSequence int    `json:"auction_sequence"`
	ProfileImageURL string `json:"profile_image_url"`
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
		SELECT
    rp.id,
    rp.room_id,
    rp.player_id,
    p.full_name,
    rp.base_price,
    rp.status,
    rp.created_at,
    rp.updated_at
FROM room_players rp
JOIN players p ON p.id = rp.player_id
WHERE rp.room_id = $1
ORDER BY p.full_name ASC
	`

	return r.getPlayers(ctx, query, roomID)
}

func (r *RoomPlayerRepository) GetByRoomAndStatus(
	ctx context.Context,
	roomID string,
	status string,
) ([]RoomPlayer, error) {
	query := `
		SELECT
    rp.id,
    rp.room_id,
    rp.player_id,
    p.full_name,
    rp.base_price,
    rp.status,
    rp.created_at,
    rp.updated_at
FROM room_players rp
JOIN players p ON p.id = rp.player_id
WHERE rp.room_id = $1
		WHERE rp.room_id = $1 AND rp.status = $2
		ORDER BY rp.auction_sequence ASC
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
			&player.FullName,
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

func (r *RoomPlayerRepository) GetAvailablePlayersByCategory(
	ctx context.Context,
	roomID string,
	category string,
) ([]RoomPlayerQueueItem, error) {

	query := `
        SELECT
            rp.id,
            rp.room_id,
            rp.player_id,
            p.full_name,
            p.country,
            p.role,
            p.is_overseas,
            rp.base_price,
            p.auction_category,
            rp.status,
            rp.auction_sequence,
            COALESCE(p.profile_image_url, '')
        FROM room_players rp
        JOIN players p ON p.id = rp.player_id
        WHERE rp.room_id = $1
          AND p.auction_category = $2
          AND p.is_auction_eligible = TRUE
          AND p.is_active = TRUE
          AND rp.status = 'AVAILABLE'
        ORDER BY rp.auction_sequence ASC
    `

	rows, err := r.db.Query(ctx, query, roomID, category)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get available players by category: %w",
			err,
		)
	}
	defer rows.Close()

	players := make([]RoomPlayerQueueItem, 0)

	for rows.Next() {
		var player RoomPlayerQueueItem

		err := rows.Scan(
			&player.RoomPlayerID,
			&player.RoomID,
			&player.PlayerID,
			&player.FullName,
			&player.Country,
			&player.Role,
			&player.IsOverseas,
			&player.BasePrice,
			&player.AuctionCategory,
			&player.Status,
			&player.AuctionSequence,
			&player.ProfileImageURL,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to scan category player: %w",
				err,
			)
		}

		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"failed to iterate category players: %w",
			err,
		)
	}

	return players, nil
}
