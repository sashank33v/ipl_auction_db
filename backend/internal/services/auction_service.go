package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionService struct {
	auctionRepo *repository.AuctionRepository
	roomRepo    *repository.RoomRepository
}

func NewAuctionService(
	auctionRepo *repository.AuctionRepository,
	roomRepo *repository.RoomRepository,
) *AuctionService {
	return &AuctionService{
		auctionRepo: auctionRepo,
		roomRepo:    roomRepo,
	}
}

func (s *AuctionService) CreateAuction(
	ctx context.Context,
	roomID string,
) (*repository.Auction, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	if room == nil {
		return nil, errors.New("room not found")
	}

	if room.Status != "WAITING" {
		return nil, errors.New("auction can only be created for waiting rooms")
	}

	existing, err := s.auctionRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("check existing auction: %w", err)
	}
	if existing != nil {
		return nil, errors.New("auction already exists for this room")
	}

	auction := &repository.Auction{
		RoomID:             roomID,
		Status:             "WAITING",
		CurrentRoundNumber: 0,
		RoundType:          "MAIN",
	}

	if err := s.auctionRepo.Create(ctx, auction); err != nil {
		return nil, fmt.Errorf("create auction: %w", err)
	}

	return auction, nil
}

func (s *AuctionService) GetAuctionByID(
	ctx context.Context,
	id string,
) (*repository.Auction, error) {
	if id == "" {
		return nil, errors.New("auction ID is required")
	}

	auction, err := s.auctionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}

	return auction, nil
}

func (s *AuctionService) GetAuctionByRoom(
	ctx context.Context,
	roomID string,
) (*repository.Auction, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	auction, err := s.auctionRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room auction: %w", err)
	}

	return auction, nil
}
