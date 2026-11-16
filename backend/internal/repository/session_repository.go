package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Session struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type SessionRepository struct {
	db DBTX
}

func NewSessionRepository(db DBTX) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(
	ctx context.Context,
	session *Session,
) error {
	query := `
		INSERT INTO sessions (
			user_id, token_hash, expires_at
		)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (r *SessionRepository) GetByTokenHash(
	ctx context.Context,
	tokenHash string,
) (*Session, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at,
		       revoked_at, created_at, last_used_at
		FROM sessions
		WHERE token_hash = $1
	`

	session := &Session{}

	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.CreatedAt,
		&session.LastUsedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) GetActiveByUser(
	ctx context.Context,
	userID string,
) ([]Session, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at,
		       revoked_at, created_at, last_used_at
		FROM sessions
		WHERE user_id = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]Session, 0)

	for rows.Next() {
		var session Session

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.TokenHash,
			&session.ExpiresAt,
			&session.RevokedAt,
			&session.CreatedAt,
			&session.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}

	return sessions, nil
}

func (r *SessionRepository) UpdateLastUsed(
	ctx context.Context,
	id string,
) error {
	query := `
		UPDATE sessions
		SET last_used_at = NOW()
		WHERE id = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update session activity: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *SessionRepository) Revoke(
	ctx context.Context,
	id string,
) error {
	query := `
		UPDATE sessions
		SET revoked_at = NOW()
		WHERE id = $1 AND revoked_at IS NULL
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
