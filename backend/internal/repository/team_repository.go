package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Team struct {
	ID             string    `json:"id"`
	RoomID         string    `json:"room_id"`
	ParticipantID  string    `json:"participant_id"`
	FranchiseCode  string    `json:"franchise_code"`
	FranchiseName  string    `json:"franchise_name"`
	RemainingPurse string    `json:"remaining_purse"`
	CreatedAt      time.Time `json:"created_at"`
}

type TeamRepository struct {
	db DBTX
}

func NewTeamRepository(db DBTX) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(
	ctx context.Context,
	team *Team,
) error {
	query := `
		INSERT INTO teams (
			room_id,
			participant_id,
			franchise_code,
			franchise_name,
			remaining_purse
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		team.RoomID,
		team.ParticipantID,
		team.FranchiseCode,
		team.FranchiseName,
		team.RemainingPurse,
	).Scan(&team.ID, &team.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	return nil
}

func (r *TeamRepository) GetByID(
	ctx context.Context,
	id string,
) (*Team, error) {
	query := `
		SELECT id, room_id, participant_id,
		       franchise_code, franchise_name,
		       remaining_purse, created_at
		FROM teams
		WHERE id = $1
	`

	team := &Team{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&team.ID,
		&team.RoomID,
		&team.ParticipantID,
		&team.FranchiseCode,
		&team.FranchiseName,
		&team.RemainingPurse,
		&team.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return team, nil
}

func (r *TeamRepository) GetByParticipant(
	ctx context.Context,
	roomID string,
	participantID string,
) (*Team, error) {
	query := `
		SELECT id, room_id, participant_id,
		       franchise_code, franchise_name,
		       remaining_purse, created_at
		FROM teams
		WHERE room_id = $1 AND participant_id = $2
	`

	team := &Team{}

	err := r.db.QueryRow(
		ctx, query, roomID, participantID,
	).Scan(
		&team.ID,
		&team.RoomID,
		&team.ParticipantID,
		&team.FranchiseCode,
		&team.FranchiseName,
		&team.RemainingPurse,
		&team.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get participant team: %w", err)
	}

	return team, nil
}

func (r *TeamRepository) GetByRoom(
	ctx context.Context,
	roomID string,
) ([]Team, error) {
	query := `
		SELECT id, room_id, participant_id,
		       franchise_code, franchise_name,
		       remaining_purse, created_at
		FROM teams
		WHERE room_id = $1
		ORDER BY created_at
	`

	rows, err := r.db.Query(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get room teams: %w", err)
	}
	defer rows.Close()

	teams := make([]Team, 0)

	for rows.Next() {
		var team Team

		err := rows.Scan(
			&team.ID,
			&team.RoomID,
			&team.ParticipantID,
			&team.FranchiseCode,
			&team.FranchiseName,
			&team.RemainingPurse,
			&team.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}

		teams = append(teams, team)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate teams: %w", err)
	}

	return teams, nil
}

func (r *TeamRepository) UpdateRemainingPurse(
	ctx context.Context,
	id string,
	remainingPurse string,
) error {
	query := `
		UPDATE teams
		SET remaining_purse = $1
		WHERE id = $2
	`

	tag, err := r.db.Exec(ctx, query, remainingPurse, id)
	if err != nil {
		return fmt.Errorf("failed to update team purse: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
