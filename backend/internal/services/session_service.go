package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	if session == nil {
		return errors.New("session is required")
	}

	session.UserID = strings.TrimSpace(session.UserID)
	session.TokenHash = strings.TrimSpace(session.TokenHash)

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
	tokenHash = strings.TrimSpace(tokenHash)

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
	userID = strings.TrimSpace(userID)

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
	sessionID = strings.TrimSpace(sessionID)

	if sessionID == "" {
		return errors.New("session ID is required")
	}

	if err := s.sessionRepo.UpdateLastUsed(ctx, sessionID); err != nil {
		return fmt.Errorf("update session last used: %w", err)
	}

	return nil
}

func (s *SessionService) RevokeSession(
	ctx context.Context,
	sessionID string,
) error {
	sessionID = strings.TrimSpace(sessionID)

	if sessionID == "" {
		return errors.New("session ID is required")
	}

	if err := s.sessionRepo.Revoke(ctx, sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}
