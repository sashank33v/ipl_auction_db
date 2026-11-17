package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type UnsoldNominationService struct {
	nominationRepo  *repository.UnsoldNominationRepository
	auctionRepo     *repository.AuctionRepository
	teamRepo        *repository.TeamRepository
	roomPlayerRepo  *repository.RoomPlayerRepository
	workflowService *UnsoldNominationWorkflowService
}

func NewUnsoldNominationService(
	nominationRepo *repository.UnsoldNominationRepository,
	auctionRepo *repository.AuctionRepository,
	teamRepo *repository.TeamRepository,
	roomPlayerRepo *repository.RoomPlayerRepository,
	workflowService *UnsoldNominationWorkflowService,
) *UnsoldNominationService {
	return &UnsoldNominationService{
		nominationRepo:  nominationRepo,
		auctionRepo:     auctionRepo,
		teamRepo:        teamRepo,
		roomPlayerRepo:  roomPlayerRepo,
		workflowService: workflowService,
	}
}

func (s *UnsoldNominationService) NominatePlayer(
	ctx context.Context,
	nomination *repository.UnsoldNomination,
) error {
	if nomination == nil {
		return errors.New("nomination is required")
	}

	if nomination.AuctionID == "" ||
		nomination.RoomPlayerID == "" ||
		nomination.TeamID == "" {
		return errors.New("auction ID, room player ID and team ID are required")
	}

	if nomination.RoundNumber < 1 {
		return errors.New("invalid round number")
	}

	auction, err := s.auctionRepo.GetByID(ctx, nomination.AuctionID)
	if err != nil {
		return fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return errors.New("auction not found")
	}

	if auction.Status != "ACTIVE" {
		return errors.New("auction is not active")
	}

	if auction.RoundType != "UNSOLD" {
		return errors.New("auction is not accepting unsold nominations")
	}

	if auction.CurrentRoundNumber != nomination.RoundNumber {
		return errors.New("invalid nomination round")
	}

	team, err := s.teamRepo.GetByID(ctx, nomination.TeamID)
	if err != nil {
		return fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return errors.New("team not found")
	}

	roomPlayer, err := s.roomPlayerRepo.GetByID(
		ctx,
		nomination.RoomPlayerID,
	)
	if err != nil {
		return fmt.Errorf("get room player: %w", err)
	}
	if roomPlayer == nil {
		return errors.New("room player not found")
	}

	if team.RoomID != auction.RoomID ||
		roomPlayer.RoomID != auction.RoomID {
		return errors.New("nomination belongs to a different room")
	}

	if roomPlayer.Status != "UNSOLD" {
		return errors.New("only unsold players can be nominated")
	}

	params := repository.CreateNominationParams{
		AuctionID:    nomination.AuctionID,
		RoomPlayerID: nomination.RoomPlayerID,
		TeamID:       nomination.TeamID,
	}

	_, err = s.workflowService.CreateNomination(ctx, params)
	if err != nil {
		return fmt.Errorf("create nomination: %w", err)
	}

	return nil
}

func (s *UnsoldNominationService) GetNominationsByRound(
	ctx context.Context,
	auctionID string,
	roundNumber int,
) ([]repository.UnsoldNomination, error) {
	if auctionID == "" || roundNumber < 1 {
		return nil, errors.New("valid auction ID and round number are required")
	}

	nominations, err := s.nominationRepo.GetByRound(
		ctx,
		auctionID,
		roundNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("get round nominations: %w", err)
	}

	return nominations, nil
}
