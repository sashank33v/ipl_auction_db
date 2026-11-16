package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type SquadService struct {
	squadRepo *repository.SquadRepository
	teamRepo  *repository.TeamRepository
}

func NewSquadService(
	squadRepo *repository.SquadRepository,
	teamRepo *repository.TeamRepository,
) *SquadService {
	return &SquadService{
		squadRepo: squadRepo,
		teamRepo:  teamRepo,
	}
}

func (s *SquadService) GetSquadByID(
	ctx context.Context,
	id string,
) (*repository.SquadPlayer, error) {
	if id == "" {
		return nil, errors.New("squad ID is required")
	}

	player, err := s.squadRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get squad player: %w", err)
	}

	return player, nil
}

func (s *SquadService) GetSquadByTeam(
	ctx context.Context,
	teamID string,
) ([]repository.SquadPlayer, error) {
	if teamID == "" {
		return nil, errors.New("team ID is required")
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	if team == nil {
		return nil, errors.New("team not found")
	}

	players, err := s.squadRepo.GetByTeam(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team squad: %w", err)
	}

	return players, nil
}

func (s *SquadService) GetSquadsByRoom(
	ctx context.Context,
	roomID string,
) ([]repository.SquadPlayer, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	players, err := s.squadRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room squads: %w", err)
	}

	return players, nil
}
