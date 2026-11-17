package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionSaleService struct {
	txManager *database.TransactionManager
}

func NewAuctionSaleService(
	txManager *database.TransactionManager,
) *AuctionSaleService {
	return &AuctionSaleService{
		txManager: txManager,
	}
}

func (s *AuctionSaleService) CompleteSale(
	ctx context.Context,
	params repository.CompleteSaleParams,
) (*repository.SaleResult, error) {
	var saleResult *repository.SaleResult

	err := s.txManager.WithinTransaction(
		ctx,
		func(tx pgx.Tx) error {
			saleRepo := repository.NewAuctionSaleRepository(tx)

			result, err := saleRepo.CompleteSale(ctx, params)
			if err != nil {
				return err
			}

			saleResult = result
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("complete sale workflow: %w", err)
	}

	return saleResult, nil
}
