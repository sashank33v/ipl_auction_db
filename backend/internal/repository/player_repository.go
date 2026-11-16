package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Player struct {
	ID              string `json:"id"`
	FullName        string `json:"full_name"`
	Country         string `json:"country"`
	Role            string `json:"role"`
	IsOverseas      bool   `json:"is_overseas"`
	BasePrice       string `json:"base_price"`
	ProfileImageURL string `json:"profile_image_url"`
	IsActive        bool   `json:"is_active"`
}

type PlayerRepository struct {
	db DBTX
}

func NewPlayerRepository(db DBTX) *PlayerRepository {
	return &PlayerRepository{db: db}
}

func (r *PlayerRepository) Create(ctx context.Context, player *Player) error {
	query := `
		INSERT INTO players (
			full_name, country, role, is_overseas,
			base_price, profile_image_url
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, is_active
	`

	err := r.db.QueryRow(
		ctx,
		query,
		player.FullName,
		player.Country,
		player.Role,
		player.IsOverseas,
		player.BasePrice,
		player.ProfileImageURL,
	).Scan(&player.ID, &player.IsActive)

	if err != nil {
		return fmt.Errorf("failed to create player: %w", err)
	}

	return nil
}

func (r *PlayerRepository) GetByID(ctx context.Context, id string) (*Player, error) {
	query := `
		SELECT id, full_name, country, role, is_overseas,
		       base_price, profile_image_url, is_active
		FROM players
		WHERE id = $1
	`

	player := &Player{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&player.ID,
		&player.FullName,
		&player.Country,
		&player.Role,
		&player.IsOverseas,
		&player.BasePrice,
		&player.ProfileImageURL,
		&player.IsActive,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get player: %w", err)
	}

	return player, nil
}

func (r *PlayerRepository) GetAll(ctx context.Context) ([]Player, error) {
	query := `
		SELECT id, full_name, country, role, is_overseas,
		       base_price, profile_image_url, is_active
		FROM players
		WHERE is_active = TRUE
		ORDER BY full_name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get players: %w", err)
	}
	defer rows.Close()

	players := make([]Player, 0)

	for rows.Next() {
		var player Player

		err := rows.Scan(
			&player.ID,
			&player.FullName,
			&player.Country,
			&player.Role,
			&player.IsOverseas,
			&player.BasePrice,
			&player.ProfileImageURL,
			&player.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan player: %w", err)
		}

		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate players: %w", err)
	}

	return players, nil
}

func (r *PlayerRepository) Update(ctx context.Context, player *Player) error {
	query := `
		UPDATE players
		SET full_name = $1,
		    country = $2,
		    role = $3,
		    is_overseas = $4,
		    base_price = $5,
		    profile_image_url = $6,
		    updated_at = NOW()
		WHERE id = $7 AND is_active = TRUE
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		player.FullName,
		player.Country,
		player.Role,
		player.IsOverseas,
		player.BasePrice,
		player.ProfileImageURL,
		player.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update player: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *PlayerRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE players
		SET is_active = FALSE,
		    updated_at = NOW()
		WHERE id = $1 AND is_active = TRUE
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to deactivate player: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
