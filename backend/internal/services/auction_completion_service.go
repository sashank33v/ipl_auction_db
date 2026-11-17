package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionCompletionService struct {
	txManager *database.TransactionManager
}

func NewAuctionCompletionService(
	txManager *database.TransactionManager,
) *AuctionCompletionService {
	return &AuctionCompletionService{
		txManager: txManager,
	}
}

func (s *AuctionCompletionService) CompleteAuction(
	ctx context.Context,
	params repository.CompleteAuctionParams,
) (*repository.AuctionCompletionResult, error) {
	var result *repository.AuctionCompletionResult

	err := s.txManager.WithinTransaction(
		ctx,
		func(tx pgx.Tx) error {
			completionRepo :=
				repository.NewAuctionCompletionRepository(tx)

			completionResult, err := completionRepo.CompleteAuction(
				ctx,
				params,
			)
			if err != nil {
				return err
			}

			result = completionResult
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf(
			"complete auction workflow: %w",
			err,
		)
	}

	return result, nil
}
