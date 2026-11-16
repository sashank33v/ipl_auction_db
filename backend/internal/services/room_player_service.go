package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type RoomPlayerService struct {
	roomPlayerRepo *repository.RoomPlayerRepository
	roomRepo       *repository.RoomRepository
	playerRepo     *repository.PlayerRepository
}

func NewRoomPlayerService(
	roomPlayerRepo *repository.RoomPlayerRepository,
	roomRepo *repository.RoomRepository,
	playerRepo *repository.PlayerRepository,
) *RoomPlayerService {
	return &RoomPlayerService{
		roomPlayerRepo: roomPlayerRepo,
		roomRepo:       roomRepo,
		playerRepo:     playerRepo,
	}
}

func (s *RoomPlayerService) AddPlayerToRoom(
	ctx context.Context,
	roomID string,
	playerID string,
) error {
	if roomID == "" || playerID == "" {
		return errors.New("room ID and player ID are required")
	}

	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return fmt.Errorf("get room: %w", err)
	}
	if room == nil {
		return errors.New("room not found")
	}
	if room.Status != "WAITING" {
		return errors.New("cannot add players after auction starts")
	}

	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("get player: %w", err)
	}
	if player == nil || !player.IsActive {
		return errors.New("player not found or inactive")
	}

	existing, err := s.roomPlayerRepo.GetByRoomAndPlayer(
		ctx, roomID, playerID,
	)
	if err != nil {
		return fmt.Errorf("check room player: %w", err)
	}
	if existing != nil {
		return errors.New("player already exists in this room")
	}

	roomPlayer := &repository.RoomPlayer{
		RoomID:    roomID,
		PlayerID:  playerID,
		BasePrice: player.BasePrice,
		Status:    "AVAILABLE",
	}

	if err := s.roomPlayerRepo.Create(ctx, roomPlayer); err != nil {
		return fmt.Errorf("add player to room: %w", err)
	}

	return nil
}

func (s *RoomPlayerService) GetRoomPlayerByID(
	ctx context.Context,
	id string,
) (*repository.RoomPlayer, error) {
	if id == "" {
		return nil, errors.New("room player ID is required")
	}

	player, err := s.roomPlayerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get room player: %w", err)
	}

	return player, nil
}

func (s *RoomPlayerService) GetPlayersByRoom(
	ctx context.Context,
	roomID string,
) ([]repository.RoomPlayer, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	players, err := s.roomPlayerRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room players: %w", err)
	}

	return players, nil
}

func (s *RoomPlayerService) GetPlayersByStatus(
	ctx context.Context,
	roomID string,
	status string,
) ([]repository.RoomPlayer, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	status = strings.ToUpper(strings.TrimSpace(status))

	switch status {
	case "AVAILABLE", "IN_AUCTION", "SOLD", "UNSOLD":
	default:
		return nil, errors.New("invalid player status")
	}

	players, err := s.roomPlayerRepo.GetByRoomAndStatus(
		ctx, roomID, status,
	)
	if err != nil {
		return nil, fmt.Errorf("get players by status: %w", err)
	}

	return players, nil
}
