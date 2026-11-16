package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type PlayerService struct {
	playerRepo *repository.PlayerRepository
}

func NewPlayerService(playerRepo *repository.PlayerRepository) *PlayerService {
	return &PlayerService{
		playerRepo: playerRepo,
	}
}

func (s *PlayerService) CreatePlayer(
	ctx context.Context,
	player *repository.Player,
) error {
	if strings.TrimSpace(player.FullName) == "" {
		return errors.New("player name is required")
	}

	if strings.TrimSpace(player.Country) == "" {
		return errors.New("player country is required")
	}

	if strings.TrimSpace(player.Role) == "" {
		return errors.New("player role is required")
	}

	if player.BasePrice == "" {
		return errors.New("player base price is required")
	}

	if player.Role != "BATSMAN" &&
		player.Role != "BOWLER" &&
		player.Role != "ALL_ROUNDER" &&
		player.Role != "WICKET_KEEPER" {
		return errors.New("invalid player role")
	}

	if player.IsOverseas && player.Country == "India" {
		return errors.New("Indian players cannot be marked as overseas")
	}

	if !player.IsOverseas && player.Country != "India" {
		return errors.New("non-Indian players must be marked as overseas")
	}

	return s.playerRepo.Create(ctx, player)
}

func (s *PlayerService) GetPlayerByID(
	ctx context.Context,
	id string,
) (*repository.Player, error) {
	if id == "" {
		return nil, errors.New("player ID is required")
	}

	player, err := s.playerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get player: %w", err)
	}

	return player, nil
}

func (s *PlayerService) GetAllPlayers(
	ctx context.Context,
) ([]repository.Player, error) {
	players, err := s.playerRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get players: %w", err)
	}

	return players, nil
}

func (s *PlayerService) UpdatePlayer(
	ctx context.Context,
	player *repository.Player,
) error {
	if player.ID == "" {
		return errors.New("player ID is required")
	}

	if strings.TrimSpace(player.FullName) == "" {
		return errors.New("player name is required")
	}

	if strings.TrimSpace(player.Country) == "" {
		return errors.New("player country is required")
	}

	if strings.TrimSpace(player.Role) == "" {
		return errors.New("player role is required")
	}

	if player.BasePrice == "" {
		return errors.New("player base price is required")
	}

	return s.playerRepo.Update(ctx, player)
}

func (s *PlayerService) DeletePlayer(
	ctx context.Context,
	id string,
) error {
	if id == "" {
		return errors.New("player ID is required")
	}

	return s.playerRepo.Delete(ctx, id)
}
