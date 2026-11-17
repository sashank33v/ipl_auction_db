package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type BidWorkflowService struct {
	txManager *database.TransactionManager
}

func NewBidWorkflowService(
	txManager *database.TransactionManager,
) *BidWorkflowService {
	return &BidWorkflowService{
		txManager: txManager,
	}
}

func (s *BidWorkflowService) AcceptBid(
	ctx context.Context,
	params repository.AcceptBidParams,
) (*repository.Bid, error) {
	if s == nil || s.txManager == nil {
		return nil, errors.New("transaction manager is not configured")
	}

	if params.AuctionID == "" ||
		params.RoomPlayerID == "" ||
		params.TeamID == "" ||
		params.RequestID == "" {
		return nil, errors.New("auction ID, room player ID, team ID and request ID are required")
	}

	var acceptedBid *repository.Bid

	err := s.txManager.WithinTransaction(
		ctx,
		func(tx pgx.Tx) error {
			bidRepo := repository.NewAuctionWorkflowRepository(tx)

			bid, err := bidRepo.AcceptBid(ctx, params)
			if err != nil {
				return err
			}
			if bid == nil {
				return errors.New("bid workflow returned an empty bid")
			}

			acceptedBid = bid
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("accept bid workflow: %w", err)
	}

	return acceptedBid, nil
}
