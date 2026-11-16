package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionEventService struct {
	eventRepo   *repository.AuctionEventRepository
	auctionRepo *repository.AuctionRepository
}

func NewAuctionEventService(
	eventRepo *repository.AuctionEventRepository,
	auctionRepo *repository.AuctionRepository,
) *AuctionEventService {
	return &AuctionEventService{
		eventRepo:   eventRepo,
		auctionRepo: auctionRepo,
	}
}

func (s *AuctionEventService) GetEventBySequence(
	ctx context.Context,
	auctionID string,
	sequenceNumber int64,
) (*repository.AuctionEvent, error) {
	if auctionID == "" || sequenceNumber < 1 {
		return nil, errors.New("valid auction ID and sequence number are required")
	}

	auction, err := s.auctionRepo.GetByID(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return nil, errors.New("auction not found")
	}

	event, err := s.eventRepo.GetBySequence(
		ctx, auctionID, sequenceNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("get auction event: %w", err)
	}

	return event, nil
}

func (s *AuctionEventService) GetEventsAfterSequence(
	ctx context.Context,
	auctionID string,
	sequenceNumber int64,
) ([]repository.AuctionEvent, error) {
	if auctionID == "" || sequenceNumber < 0 {
		return nil, errors.New("valid auction ID and sequence number are required")
	}

	auction, err := s.auctionRepo.GetByID(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return nil, errors.New("auction not found")
	}
	const limit = 100
	events, err := s.eventRepo.GetAfterSequence(
		ctx, auctionID, sequenceNumber, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get auction events: %w", err)
	}

	return events, nil
}

func (s *AuctionEventService) ValidateEventPayload(
	payload json.RawMessage,
) error {
	if len(payload) == 0 || !json.Valid(payload) {
		return errors.New("invalid event payload")
	}

	return nil
}
