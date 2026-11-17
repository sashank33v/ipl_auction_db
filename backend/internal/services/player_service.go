package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

func validatePlayer(player *repository.Player) error {
	if player == nil {
		return errors.New("player is required")
	}

	player.FullName = strings.TrimSpace(player.FullName)
	player.Country = strings.TrimSpace(player.Country)
	player.Role = strings.ToUpper(strings.TrimSpace(player.Role))

	if player.FullName == "" {
		return errors.New("player name is required")
	}

	if player.Country == "" {
		return errors.New("player country is required")
	}

	if player.Role == "" {
		return errors.New("player role is required")
	}

	switch player.Role {
	case "BATSMAN", "BOWLER", "ALL_ROUNDER", "WICKET_KEEPER":
	default:
		return errors.New("invalid player role")
	}

	if player.BasePrice == "" {
		return errors.New("player base price is required")
	}

	basePrice, err := strconv.ParseFloat(player.BasePrice, 64)
	if err != nil || basePrice <= 0 {
		return errors.New("player base price must be a positive number")
	}

	if strings.EqualFold(player.Country, "India") {
		player.Country = "India"
		if player.IsOverseas {
			return errors.New("Indian players cannot be marked as overseas")
		}
	} else if !player.IsOverseas {
		return errors.New("non-Indian players must be marked as overseas")
	}

	return nil
}

func (s *PlayerService) CreatePlayer(
	ctx context.Context,
	player *repository.Player,
) error {
	if err := validatePlayer(player); err != nil {
		return err
	}

	if err := s.playerRepo.Create(ctx, player); err != nil {
		return fmt.Errorf("create player: %w", err)
	}

	return nil
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
	if player == nil {
		return nil, errors.New("player not found")
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
	if player == nil {
		return errors.New("player is required")
	}

	if player.ID == "" {
		return errors.New("player ID is required")
	}

	if err := validatePlayer(player); err != nil {
		return err
	}

	existing, err := s.playerRepo.GetByID(ctx, player.ID)
	if err != nil {
		return fmt.Errorf("get player before update: %w", err)
	}
	if existing == nil {
		return errors.New("player not found")
	}

	if err := s.playerRepo.Update(ctx, player); err != nil {
		return fmt.Errorf("update player: %w", err)
	}

	return nil
}

func (s *PlayerService) DeletePlayer(
	ctx context.Context,
	id string,
) error {
	if id == "" {
		return errors.New("player ID is required")
	}

	existing, err := s.playerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get player before deletion: %w", err)
	}
	if existing == nil {
		return errors.New("player not found")
	}

	if err := s.playerRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deactivate player: %w", err)
	}

	return nil
}
