package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type BidService struct {
	bidRepo     *repository.BidRepository
	auctionRepo *repository.AuctionRepository
}

func NewBidService(
	bidRepo *repository.BidRepository,
	auctionRepo *repository.AuctionRepository,
) *BidService {
	return &BidService{
		bidRepo:     bidRepo,
		auctionRepo: auctionRepo,
	}
}

func (s *BidService) GetBidByID(
	ctx context.Context,
	id string,
) (*repository.Bid, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("bid ID is required")
	}

	bid, err := s.bidRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get bid: %w", err)
	}
	if bid == nil {
		return nil, errors.New("bid not found")
	}

	return bid, nil
}

func (s *BidService) GetBidByRequestID(
	ctx context.Context,
	auctionID string,
	requestID string,
) (*repository.Bid, error) {
	auctionID = strings.TrimSpace(auctionID)
	requestID = strings.TrimSpace(requestID)

	if auctionID == "" || requestID == "" {
		return nil, errors.New("auction ID and request ID are required")
	}

	auction, err := s.auctionRepo.GetByID(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return nil, errors.New("auction not found")
	}

	bid, err := s.bidRepo.GetByRequestID(
		ctx,
		auctionID,
		requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("get bid by request ID: %w", err)
	}
	if bid == nil {
		return nil, errors.New("bid not found")
	}

	return bid, nil
}

func (s *BidService) GetAuctionBids(
	ctx context.Context,
	auctionID string,
) ([]repository.Bid, error) {
	auctionID = strings.TrimSpace(auctionID)
	if auctionID == "" {
		return nil, errors.New("auction ID is required")
	}

	auction, err := s.auctionRepo.GetByID(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return nil, errors.New("auction not found")
	}

	bids, err := s.bidRepo.GetByAuction(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("get auction bids: %w", err)
	}

	return bids, nil
}
