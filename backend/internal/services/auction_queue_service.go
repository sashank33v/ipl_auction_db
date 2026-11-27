package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/auction"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

var ErrQueueRoomNotFound = errors.New("room not found")
var ErrQueueHostOnly = errors.New("only room host can prepare queue")

func (s *RoomPlayerService) PrepareAuctionQueue(ctx context.Context, roomID, userID string, opts auction.QueueOptions) ([]repository.RoomPlayer, error) {
	if s.txManager == nil {
		return nil, errors.New("transaction manager is not configured")
	}
	var result []repository.RoomPlayer
	err := s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		room, err := repository.NewRoomRepository(tx).GetByIDForUpdate(ctx, roomID)
		if err != nil {
			return err
		}
		if room == nil {
			return ErrQueueRoomNotFound
		}
		if room.HostUserID != userID {
			return ErrQueueHostOnly
		}
		if room.Status != "WAITING" {
			return fmt.Errorf("%w: room is no longer waiting", repository.ErrQueueConflict)
		}
		repo := repository.NewRoomPlayerRepository(tx)
		if _, err = repo.EnsureAuctionQueue(ctx, roomID, opts, true); err != nil {
			return err
		}
		result, err = repo.GetByRoom(ctx, roomID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
