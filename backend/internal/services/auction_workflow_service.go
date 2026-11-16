package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionWorkflowService struct {
	txManager *database.TransactionManager
}

func NewAuctionWorkflowService(
	txManager *database.TransactionManager,
) *AuctionWorkflowService {
	return &AuctionWorkflowService{
		txManager: txManager,
	}
}

func (s *AuctionWorkflowService) AcceptBid(
	ctx context.Context,
	params repository.AcceptBidParams,
) error {
	if params.AuctionID == "" ||
		params.TeamID == "" ||
		params.RoomPlayerID == "" ||
		params.RequestID == "" ||
		params.BidAmount == "" {
		return errors.New("invalid bid request")
	}

	if len(params.EventPayload) == 0 ||
		!json.Valid(params.EventPayload) {
		return errors.New("invalid event payload")
	}

	return s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		var auctionStatus string

		err := tx.QueryRow(ctx, `
			SELECT status
			FROM auctions
			WHERE id = $1
			FOR UPDATE
		`, params.AuctionID).Scan(&auctionStatus)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("auction not found")
			}
			return fmt.Errorf("lock auction: %w", err)
		}

		if auctionStatus != "LIVE" {
			return errors.New("auction is not accepting bids")
		}

		workflowRepo := repository.NewAuctionWorkflowRepository(tx)

		if err := workflowRepo.AcceptBid(ctx, params); err != nil {
			return fmt.Errorf("accept bid: %w", err)
		}

		return nil
	})
}
