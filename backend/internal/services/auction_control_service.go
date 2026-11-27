package services

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

// SetPaused shares the same per-auction command queue as bids and timers.
func (s *AuctionService) SetPaused(ctx context.Context, auctionID, userID string, pause bool) (*repository.Auction, error) {
	if s.txManager == nil {
		return nil, errors.New("transaction manager is not configured")
	}
	var state *repository.Auction
	run := func(ctx context.Context) error {
		return s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
			var err error
			state, err = repository.NewAuctionEngineRepository(tx).SetPaused(ctx, auctionID, userID, pause)
			return err
		})
	}
	var err error
	if s.actors != nil {
		err = s.actors.Do(ctx, strings.ToLower(auctionID), run)
	} else {
		err = run(ctx)
	}
	if err != nil {
		return nil, err
	}
	return state, nil
}
