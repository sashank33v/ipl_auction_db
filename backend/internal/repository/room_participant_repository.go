package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type RoomParticipant struct {
	ID               string    `json:"id"`
	RoomID           string    `json:"room_id"`
	UserID           string    `json:"user_id"`
	JoinedAt         time.Time `json:"joined_at"`
	ConnectionStatus string    `json:"connection_status"`
}

type RoomParticipantRepository struct {
	db DBTX
}

func NewRoomParticipantRepository(db DBTX) *RoomParticipantRepository {
	return &RoomParticipantRepository{db: db}
}

func (r *RoomParticipantRepository) Create(
	ctx context.Context,
	participant *RoomParticipant,
) error {
	query := `
		INSERT INTO room_participants (
			room_id, user_id, connection_status
		)
		VALUES ($1, $2, $3)
		RETURNING id, joined_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		participant.RoomID,
		participant.UserID,
		participant.ConnectionStatus,
	).Scan(&participant.ID, &participant.JoinedAt)

	if err != nil {
		return fmt.Errorf("failed to create participant: %w", err)
	}

	return nil
}

func (r *RoomParticipantRepository) GetByID(
	ctx context.Context,
	id string,
) (*RoomParticipant, error) {
	query := `
		SELECT id, room_id, user_id, joined_at, connection_status
		FROM room_participants
		WHERE id = $1
	`

	participant := &RoomParticipant{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&participant.ID,
		&participant.RoomID,
		&participant.UserID,
		&participant.JoinedAt,
		&participant.ConnectionStatus,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}

	return participant, nil
}

func (r *RoomParticipantRepository) GetByRoomAndUser(
	ctx context.Context,
	roomID string,
	userID string,
) (*RoomParticipant, error) {
	query := `
		SELECT id, room_id, user_id, joined_at, connection_status
		FROM room_participants
		WHERE room_id = $1 AND user_id = $2
	`

	participant := &RoomParticipant{}

	err := r.db.QueryRow(ctx, query, roomID, userID).Scan(
		&participant.ID,
		&participant.RoomID,
		&participant.UserID,
		&participant.JoinedAt,
		&participant.ConnectionStatus,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}

	return participant, nil
}

func (r *RoomParticipantRepository) GetByRoom(
	ctx context.Context,
	roomID string,
) ([]RoomParticipant, error) {
	query := `
		SELECT id, room_id, user_id, joined_at, connection_status
		FROM room_participants
		WHERE room_id = $1
		ORDER BY joined_at
	`

	rows, err := r.db.Query(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get room participants: %w", err)
	}
	defer rows.Close()

	participants := make([]RoomParticipant, 0)

	for rows.Next() {
		var participant RoomParticipant

		err := rows.Scan(
			&participant.ID,
			&participant.RoomID,
			&participant.UserID,
			&participant.JoinedAt,
			&participant.ConnectionStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}

		participants = append(participants, participant)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate participants: %w", err)
	}

	return participants, nil
}

func (r *RoomParticipantRepository) UpdateConnectionStatus(
	ctx context.Context,
	id string,
	status string,
) error {
	query := `
		UPDATE room_participants
		SET connection_status = $1
		WHERE id = $2
	`

	tag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update connection status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
