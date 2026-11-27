package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/auction"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type RoomService struct {
	txManager *database.TransactionManager
	roomRepo  *repository.RoomRepository
}

func NewRoomService(txManager *database.TransactionManager, roomRepo *repository.RoomRepository) *RoomService {
	return &RoomService{
		txManager: txManager,
		roomRepo:  roomRepo,
	}
}

func (s *RoomService) CreateRoom(
	ctx context.Context,
	room *repository.Room,
) error {
	if room == nil {
		return errors.New("room is required")
	}

	room.HostUserID = strings.TrimSpace(room.HostUserID)
	room.Name = strings.TrimSpace(room.Name)

	if room.HostUserID == "" {
		return errors.New("host user ID is required")
	}

	if room.Name == "" {
		return errors.New("room name is required")
	}

	if room.MaxParticipants < 4 || room.MaxParticipants > 10 {
		return errors.New("maximum participants must be between 4 and 10")
	}

	if err := applyApprovedRules(room); err != nil {
		return err
	}

	if err := s.roomRepo.Create(ctx, room); err != nil {
		return fmt.Errorf("create room: %w", err)
	}
	return nil
}

// applyApprovedRules rejects caller overrides and stores canonical rule values.
func applyApprovedRules(room *repository.Room) error {
	if err := auction.ValidateRoomRules(room.StartingPurse, room.MinBidIncrement, room.MinSquadSize, room.MaxSquadSize, room.MaxOverseasPlayers); err != nil {
		return err
	}

	room.StartingPurse = auction.StartingPurse
	room.MinBidIncrement = auction.MinBidIncrement
	room.MinSquadSize = auction.MinSquadSize
	room.MaxSquadSize = auction.MaxSquadSize
	room.MaxOverseasPlayers = auction.MaxOverseasPlayers
	return nil
}

func (s *RoomService) GetRoomByID(
	ctx context.Context,
	id string,
) (*repository.Room, error) {
	if id == "" {
		return nil, errors.New("room ID is required")
	}

	room, err := s.roomRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	if room == nil {
		return nil, errors.New("room not found")
	}

	return room, nil
}

func (s *RoomService) GetRoomForUser(ctx context.Context, id, userID string) (*repository.Room, error) {
	return s.roomRepo.GetByIDForUser(ctx, id, userID)
}

func (s *RoomService) GetRoomByCode(
	ctx context.Context,
	code string,
) (*repository.Room, error) {
	if code == "" {
		return nil, errors.New("room code is required")
	}

	room, err := s.roomRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get room by code: %w", err)
	}
	if room == nil {
		return nil, errors.New("room not found")
	}

	return room, nil
}

func (s *RoomService) GetAllRooms(
	ctx context.Context, userID string,
) ([]repository.Room, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}
	rooms, err := s.roomRepo.GetForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get rooms: %w", err)
	}

	return rooms, nil
}

func (s *RoomService) UpdateRoom(ctx context.Context, room *repository.Room) error {
	if s.txManager == nil {
		return errors.New("transaction manager is not configured")
	}
	return s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		scoped := &RoomService{roomRepo: repository.NewRoomRepository(tx)}
		return scoped.updateRoomLocked(ctx, room, tx)
	})
}

func (s *RoomService) updateRoomLocked(
	ctx context.Context,
	room *repository.Room,
	tx pgx.Tx,
) error {
	if room == nil {
		return errors.New("room is required")
	}

	if room.ID == "" {
		return errors.New("room ID is required")
	}

	room.Name = strings.TrimSpace(room.Name)
	if room.Name == "" {
		return errors.New("room name is required")
	}

	existing, err := s.roomRepo.GetByIDForUpdate(ctx, room.ID)
	if err != nil {
		return fmt.Errorf("get room before update: %w", err)
	}
	if existing == nil {
		return errors.New("room not found")
	}

	if existing.Status != "WAITING" {
		return errors.New("only waiting rooms can be updated")
	}

	if room.MaxParticipants == 0 {
		room.MaxParticipants = existing.MaxParticipants
	}
	if room.MaxParticipants < 4 || room.MaxParticipants > 10 {
		return errors.New("maximum participants must be between 4 and 10")
	}

	participants, err := repository.NewRoomParticipantRepository(tx).GetByRoom(ctx, room.ID)
	if err != nil {
		return err
	}
	participantCount := len(participants)
	if room.MaxParticipants < participantCount {
		return errors.New("capacity cannot be smaller than existing membership")
	}

	if err := s.roomRepo.Update(ctx, room); err != nil {
		return fmt.Errorf("update room: %w", err)
	}

	return nil
}

func (s *RoomService) DeleteRoom(ctx context.Context, id string) error {
	if s.txManager == nil {
		return errors.New("transaction manager is not configured")
	}
	return s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		scoped := &RoomService{roomRepo: repository.NewRoomRepository(tx)}
		return scoped.deleteRoomLocked(ctx, id)
	})
}

func (s *RoomService) deleteRoomLocked(
	ctx context.Context,
	id string,
) error {
	if id == "" {
		return errors.New("room ID is required")
	}

	room, err := s.roomRepo.GetByIDForUpdate(ctx, id)
	if err != nil {
		return fmt.Errorf("get room before deletion: %w", err)
	}
	if room == nil {
		return errors.New("room not found")
	}

	if room.Status != "WAITING" {
		return errors.New("only waiting rooms can be deleted")
	}

	if _, err := s.roomRepo.GetByID(ctx, id); err != nil {
		return fmt.Errorf("verify room before deletion: %w", err)
	}

	if err := s.roomRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete room: %w", err)
	}

	return nil
}
