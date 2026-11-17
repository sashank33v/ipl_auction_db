package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionUnsoldService struct {
	txManager *database.TransactionManager
}

func NewAuctionUnsoldService(
	txManager *database.TransactionManager,
) *AuctionUnsoldService {
	return &AuctionUnsoldService{
		txManager: txManager,
	}
}

func (s *AuctionUnsoldService) MarkUnsold(
	ctx context.Context,
	params repository.MarkUnsoldParams,
) (*repository.UnsoldResult, error) {
	var result *repository.UnsoldResult

	err := s.txManager.WithinTransaction(
		ctx,
		func(tx pgx.Tx) error {
			unsoldRepo := repository.NewAuctionUnsoldRepository(tx)

			unsoldResult, err := unsoldRepo.MarkUnsold(ctx, params)
			if err != nil {
				return err
			}

			result = unsoldResult
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("mark player unsold workflow: %w", err)
	}

	return result, nil
}
