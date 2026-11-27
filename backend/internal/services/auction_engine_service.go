package services

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/auction"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionEngineService struct {
	txManager *database.TransactionManager
	repo      *repository.AuctionEngineRepository
}

func NewAuctionEngineService(tm *database.TransactionManager, repo *repository.AuctionEngineRepository) *AuctionEngineService {
	return &AuctionEngineService{txManager: tm, repo: repo}
}

func (s *AuctionEngineService) RecoverOnStartup(ctx context.Context) (int64, error) {
	return s.repo.RecoverOnStartup(ctx)
}

func (s *AuctionEngineService) Advance(ctx context.Context, id string) (string, error) {
	var transition string
	err := s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		transition, err = repository.NewAuctionEngineRepository(tx).Advance(ctx, id)
		return err
	})
	if err != nil {
		return "", err
	}
	return transition, nil
}

func (s *AuctionEngineService) Run(ctx context.Context) {
	auction.RunTimer(ctx, 250*time.Millisecond, func(ctx context.Context) {
		queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		ids, err := s.repo.DueAuctions(queryCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("auction engine list: %v", err)
			}
			return
		}
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			roomCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			transition, err := s.Advance(roomCtx, id)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("auction engine %s: %v", id, err)
				}
				continue
			}
			if transition != "" {
				log.Printf("auction %s: %s", id, transition)
			}
		}
	})
}

func (s *AuctionService) ResumeRecoveredAuction(ctx context.Context, id, userID string) error {
	return s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		return repository.NewAuctionEngineRepository(tx).ResumeRecovered(ctx, id, userID)
	})
}
