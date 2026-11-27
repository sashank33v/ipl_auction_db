package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type RoomPlayerService struct {
	txManager      *database.TransactionManager
	roomPlayerRepo *repository.RoomPlayerRepository
	roomRepo       *repository.RoomRepository
	playerRepo     *repository.PlayerRepository
}

func NewRoomPlayerService(
	txManager *database.TransactionManager,
	roomPlayerRepo *repository.RoomPlayerRepository,
	roomRepo *repository.RoomRepository,
	playerRepo *repository.PlayerRepository,
) *RoomPlayerService {
	return &RoomPlayerService{
		txManager:      txManager,
		roomPlayerRepo: roomPlayerRepo,
		roomRepo:       roomRepo,
		playerRepo:     playerRepo,
	}
}

func (s *RoomPlayerService) AddPlayerToRoom(ctx context.Context, roomID, playerID string) error {
	if s.txManager == nil {
		return errors.New("transaction manager is not configured")
	}
	return s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		scoped := &RoomPlayerService{
			roomPlayerRepo: repository.NewRoomPlayerRepository(tx),
			roomRepo:       repository.NewRoomRepository(tx),
			playerRepo:     repository.NewPlayerRepository(tx),
		}
		return scoped.addPlayerToRoomLocked(ctx, roomID, playerID)
	})
}

func (s *RoomPlayerService) addPlayerToRoomLocked(
	ctx context.Context,
	roomID string,
	playerID string,
) error {
	if roomID == "" || playerID == "" {
		return errors.New("room ID and player ID are required")
	}

	room, err := s.roomRepo.GetByIDForUpdate(ctx, roomID)
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

	basePrice, err := strconv.ParseFloat(player.BasePrice, 64)
	if err != nil || basePrice <= 0 {
		return errors.New("player base price must be positive")
	}

	existing, err := s.roomPlayerRepo.GetByRoomAndPlayer(
		ctx,
		roomID,
		playerID,
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
	if player == nil {
		return nil, errors.New("room player not found")
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
		ctx,
		roomID,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("get players by status: %w", err)
	}

	return players, nil
}

func (s *RoomPlayerService) GetAvailablePlayersByCategory(
	ctx context.Context,
	roomID string,
	category string,
) ([]repository.RoomPlayerQueueItem, error) {

	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	category = strings.ToUpper(strings.TrimSpace(category))

	switch category {
	case "MARQUEE",
		"CAPPED_BATTER",
		"CAPPED_BOWLER",
		"CAPPED_ALL_ROUNDER",
		"CAPPED_WICKETKEEPER",
		"UNCAPPED_BATTER",
		"UNCAPPED_BOWLER",
		"UNCAPPED_ALL_ROUNDER",
		"UNCAPPED_WICKETKEEPER",
		"ASSOCIATE_ALL_ROUNDER",
		"REFERENCE_BATTER",
		"REFERENCE_BOWLER",
		"REFERENCE_ALL_ROUNDER",
		"REFERENCE_WICKETKEEPER":
	default:
		return nil, errors.New("invalid auction category")
	}

	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}

	if room == nil {
		return nil, errors.New("room not found")
	}

	players, err := s.roomPlayerRepo.GetAvailablePlayersByCategory(
		ctx,
		roomID,
		category,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get available players by category: %w",
			err,
		)
	}

	return players, nil
}
