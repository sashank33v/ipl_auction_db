package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/rooms"
)

type BidWorkflowService struct {
	txManager *database.TransactionManager
	actors    *rooms.Manager
}

func NewBidWorkflowService(
	txManager *database.TransactionManager,
) *BidWorkflowService {
	return &BidWorkflowService{
		txManager: txManager,
	}
}

// SetCommandManager is called once during server startup.
func (s *BidWorkflowService) SetCommandManager(m *rooms.Manager) { s.actors = m }

func (s *BidWorkflowService) AcceptBid(ctx context.Context, params repository.AcceptBidParams) (*repository.Bid, error) {
	if s == nil || s.actors == nil {
		return s.acceptBid(ctx, params)
	}
	var bid *repository.Bid
	err := s.actors.Do(ctx, params.AuctionID, func(ctx context.Context) error { var err error; bid, err = s.acceptBid(ctx, params); return err })
	if err != nil {
		return nil, err
	}
	return bid, nil
}

func (s *BidWorkflowService) acceptBid(
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
