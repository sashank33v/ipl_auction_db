package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type UnsoldNominationWorkflowService struct {
	txManager *database.TransactionManager
}

func NewUnsoldNominationWorkflowService(
	txManager *database.TransactionManager,
) *UnsoldNominationWorkflowService {
	return &UnsoldNominationWorkflowService{
		txManager: txManager,
	}
}

func (s *UnsoldNominationWorkflowService) CreateNomination(
	ctx context.Context,
	params repository.CreateNominationParams,
) (*repository.NominationResult, error) {
	var nomination *repository.NominationResult

	err := s.txManager.WithinTransaction(
		ctx,
		func(tx pgx.Tx) error {
			nominationRepo :=
				repository.NewUnsoldNominationWorkflowRepository(tx)

			result, err := nominationRepo.CreateNomination(
				ctx,
				params,
			)
			if err != nil {
				return err
			}

			nomination = result
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf(
			"create unsold nomination workflow: %w",
			err,
		)
	}

	return nomination, nil
}
