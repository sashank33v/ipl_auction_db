package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type OutboxEventService struct {
	outboxRepo *repository.OutboxEventRepository
}

func NewOutboxEventService(
	outboxRepo *repository.OutboxEventRepository,
) *OutboxEventService {
	return &OutboxEventService{
		outboxRepo: outboxRepo,
	}
}

func (s *OutboxEventService) GetOutboxEventByID(
	ctx context.Context,
	id string,
) (*repository.OutboxEvent, error) {
	if id == "" {
		return nil, errors.New("outbox event ID is required")
	}

	event, err := s.outboxRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get outbox event: %w", err)
	}

	return event, nil
}

func (s *OutboxEventService) GetPendingEvents(
	ctx context.Context,
	limit int,
) ([]repository.OutboxEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	if limit > 500 {
		limit = 500
	}

	events, err := s.outboxRepo.GetPending(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending events: %w", err)
	}

	return events, nil
}
