package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type SessionService struct {
	sessionRepo *repository.SessionRepository
}

func NewSessionService(
	sessionRepo *repository.SessionRepository,
) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
	}
}

func (s *SessionService) CreateSession(
	ctx context.Context,
	session *repository.Session,
) error {
	if session.UserID == "" {
		return errors.New("user ID is required")
	}

	if session.TokenHash == "" {
		return errors.New("token hash is required")
	}

	if !session.ExpiresAt.After(time.Now()) {
		return errors.New("session expiry must be in the future")
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

func (s *SessionService) GetSessionByTokenHash(
	ctx context.Context,
	tokenHash string,
) (*repository.Session, error) {
	if tokenHash == "" {
		return nil, errors.New("token hash is required")
	}

	session, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if session == nil {
		return nil, errors.New("session not found")
	}

	if session.RevokedAt != nil {
		return nil, errors.New("session has been revoked")
	}

	if !session.ExpiresAt.After(time.Now()) {
		return nil, errors.New("session has expired")
	}

	return session, nil
}

func (s *SessionService) GetActiveSessionsByUser(
	ctx context.Context,
	userID string,
) ([]repository.Session, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	sessions, err := s.sessionRepo.GetActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get active sessions: %w", err)
	}

	return sessions, nil
}

func (s *SessionService) UpdateLastUsed(
	ctx context.Context,
	sessionID string,
) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}

	return s.sessionRepo.UpdateLastUsed(ctx, sessionID)
}

func (s *SessionService) RevokeSession(
	ctx context.Context,
	sessionID string,
) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}

	return s.sessionRepo.Revoke(ctx, sessionID)
}
