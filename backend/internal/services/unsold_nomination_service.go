package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type UnsoldNominationService struct {
	nominationRepo *repository.UnsoldNominationRepository
	auctionRepo    *repository.AuctionRepository
	teamRepo       *repository.TeamRepository
	roomPlayerRepo *repository.RoomPlayerRepository
}

func NewUnsoldNominationService(
	nominationRepo *repository.UnsoldNominationRepository,
	auctionRepo *repository.AuctionRepository,
	teamRepo *repository.TeamRepository,
	roomPlayerRepo *repository.RoomPlayerRepository,
) *UnsoldNominationService {
	return &UnsoldNominationService{
		nominationRepo: nominationRepo,
		auctionRepo:    auctionRepo,
		teamRepo:       teamRepo,
		roomPlayerRepo: roomPlayerRepo,
	}
}

func (s *UnsoldNominationService) NominatePlayer(
	ctx context.Context,
	nomination *repository.UnsoldNomination,
) error {
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

	if auction.Status != "UNSOLD_ROUND" {
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
		ctx, nomination.RoomPlayerID,
	)
	if err != nil {
		return fmt.Errorf("get room player: %w", err)
	}
	if roomPlayer == nil {
		return errors.New("room player not found")
	}

	if roomPlayer.RoomID != auction.RoomID ||
		team.RoomID != auction.RoomID {
		return errors.New("nomination belongs to a different room")
	}

	if roomPlayer.Status != "UNSOLD" {
		return errors.New("only unsold players can be nominated")
	}

	existing, err := s.nominationRepo.GetByTeamAndRound(
		ctx,
		nomination.AuctionID,
		nomination.TeamID,
		nomination.RoundNumber,
	)
	if err != nil {
		return fmt.Errorf("get existing nominations: %w", err)
	}

	if len(existing) >= 6 {
		return errors.New("maximum six nominations allowed per team per round")
	}

	for _, item := range existing {
		if item.RoomPlayerID == nomination.RoomPlayerID {
			return errors.New("player already nominated by this team")
		}
	}

	if err := s.nominationRepo.Create(ctx, nomination); err != nil {
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
		ctx, auctionID, roundNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("get round nominations: %w", err)
	}

	return nominations, nil
}
