package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type MainNominationWorkflowService struct {
	txManager *database.TransactionManager
}

func NewMainNominationWorkflowService(
	txManager *database.TransactionManager,
) *MainNominationWorkflowService {
	return &MainNominationWorkflowService{
		txManager: txManager,
	}
}

func (s *MainNominationWorkflowService) NominatePlayer(
	ctx context.Context,
	params repository.MainNominationParams,
) (*repository.MainNominationResult, error) {

	if s == nil || s.txManager == nil {
		return nil, errors.New(
			"transaction manager is not configured",
		)
	}

	if params.AuctionID == "" ||
		params.PlayerID == "" ||
		params.UserID == "" {
		return nil, errors.New(
			"auction ID, room player ID and user ID are required",
		)
	}

	var nomination *repository.MainNominationResult

	err := s.txManager.WithinTransaction(
		ctx,
		func(tx pgx.Tx) error {

			nominationRepo :=
				repository.NewMainNominationWorkflowRepository(tx)

			result, err := nominationRepo.NominatePlayer(
				ctx,
				params,
			)

			if err != nil {
				return err
			}

			if result == nil {
				return errors.New(
					"main nomination workflow returned an empty result",
				)
			}

			nomination = result

			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf(
			"main nomination workflow: %w",
			err,
		)
	}

	return nomination, nil
}
