package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Room struct {
	ID                 string `json:"id"`
	RoomCode           string `json:"room_code"`
	HostUserID         string `json:"host_user_id"`
	Name               string `json:"name"`
	Status             string `json:"status"`
	MaxParticipants    int    `json:"max_participants"`
	StartingPurse      string `json:"starting_purse"`
	MinSquadSize       int    `json:"min_squad_size"`
	MaxSquadSize       int    `json:"max_squad_size"`
	MaxOverseasPlayers int    `json:"max_overseas_players"`
	MinBidIncrement    string `json:"min_bid_increment"`
}

type RoomRepository struct {
	db DBTX
}

func NewRoomRepository(db DBTX) *RoomRepository {
	return &RoomRepository{db: db}
}

func (r *RoomRepository) Create(ctx context.Context, room *Room) error {
	query := `
		INSERT INTO rooms (
			room_code,
			host_user_id,
			name,
			status,
			max_participants,
			starting_purse,
			min_squad_size,
			max_squad_size,
			max_overseas_players,
			min_bid_increment
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err := r.db.QueryRow(
		ctx,
		query,
		room.RoomCode,
		room.HostUserID,
		room.Name,
		room.Status,
		room.MaxParticipants,
		room.StartingPurse,
		room.MinSquadSize,
		room.MaxSquadSize,
		room.MaxOverseasPlayers,
		room.MinBidIncrement,
	).Scan(&room.ID)

	if err != nil {
		return fmt.Errorf("failed to create room: %w", err)
	}

	return nil
}

func (r *RoomRepository) GetByID(ctx context.Context, id string) (*Room, error) {
	query := `
		SELECT
			id, room_code, host_user_id, name, status,
			max_participants, starting_purse, min_squad_size,
			max_squad_size, max_overseas_players, min_bid_increment
		FROM rooms
		WHERE id = $1
	`

	room := &Room{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&room.ID,
		&room.RoomCode,
		&room.HostUserID,
		&room.Name,
		&room.Status,
		&room.MaxParticipants,
		&room.StartingPurse,
		&room.MinSquadSize,
		&room.MaxSquadSize,
		&room.MaxOverseasPlayers,
		&room.MinBidIncrement,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	return room, nil
}

func (r *RoomRepository) GetByCode(ctx context.Context, code string) (*Room, error) {
	query := `
		SELECT
			id, room_code, host_user_id, name, status,
			max_participants, starting_purse, min_squad_size,
			max_squad_size, max_overseas_players, min_bid_increment
		FROM rooms
		WHERE room_code = $1
	`

	room := &Room{}

	err := r.db.QueryRow(ctx, query, code).Scan(
		&room.ID,
		&room.RoomCode,
		&room.HostUserID,
		&room.Name,
		&room.Status,
		&room.MaxParticipants,
		&room.StartingPurse,
		&room.MinSquadSize,
		&room.MaxSquadSize,
		&room.MaxOverseasPlayers,
		&room.MinBidIncrement,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get room by code: %w", err)
	}

	return room, nil
}

func (r *RoomRepository) GetAll(ctx context.Context) ([]Room, error) {
	query := `
		SELECT
			id, room_code, host_user_id, name, status,
			max_participants, starting_purse, min_squad_size,
			max_squad_size, max_overseas_players, min_bid_increment
		FROM rooms
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]Room, 0)

	for rows.Next() {
		var room Room

		err := rows.Scan(
			&room.ID,
			&room.RoomCode,
			&room.HostUserID,
			&room.Name,
			&room.Status,
			&room.MaxParticipants,
			&room.StartingPurse,
			&room.MinSquadSize,
			&room.MaxSquadSize,
			&room.MaxOverseasPlayers,
			&room.MinBidIncrement,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}

		rooms = append(rooms, room)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rooms: %w", err)
	}

	return rooms, nil
}

func (r *RoomRepository) Update(ctx context.Context, room *Room) error {
	query := `
		UPDATE rooms
		SET name = $1,
		    max_participants = $2,
		    updated_at = NOW()
		WHERE id = $3
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		room.Name,
		room.MaxParticipants,
		room.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update room: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *RoomRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM rooms WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete room: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
