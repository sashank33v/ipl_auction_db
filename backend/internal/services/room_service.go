package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type RoomService struct {
	roomRepo *repository.RoomRepository
}

func NewRoomService(roomRepo *repository.RoomRepository) *RoomService {
	return &RoomService{
		roomRepo: roomRepo,
	}
}

func (s *RoomService) CreateRoom(
	ctx context.Context,
	room *repository.Room,
) error {
	if room.HostUserID == "" {
		return errors.New("host user ID is required")
	}

	if room.Name == "" {
		return errors.New("room name is required")
	}

	if room.MaxParticipants < 4 || room.MaxParticipants > 10 {
		return errors.New("maximum participants must be between 4 and 10")
	}

	if room.StartingPurse == "" {
		room.StartingPurse = "120.00"
	}

	if room.MinSquadSize == 0 {
		room.MinSquadSize = 18
	}

	if room.MaxSquadSize == 0 {
		room.MaxSquadSize = 25
	}

	if room.MaxOverseasPlayers == 0 {
		room.MaxOverseasPlayers = 8
	}

	if room.MinBidIncrement == "" {
		room.MinBidIncrement = "0.10"
	}

	if room.MinSquadSize > room.MaxSquadSize {
		return errors.New("minimum squad size cannot exceed maximum squad size")
	}

	if room.MaxSquadSize < 18 || room.MaxSquadSize > 25 {
		return errors.New("maximum squad size must be between 18 and 25")
	}

	if room.MaxOverseasPlayers > room.MaxSquadSize {
		return errors.New("overseas player limit cannot exceed maximum squad size")
	}

	err := s.roomRepo.Create(ctx, room)
	if err != nil {
		return fmt.Errorf("create room: %w", err)
	}

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

	return room, nil
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

	return room, nil
}

func (s *RoomService) GetAllRooms(
	ctx context.Context,
) ([]repository.Room, error) {
	rooms, err := s.roomRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get rooms: %w", err)
	}

	return rooms, nil
}

func (s *RoomService) UpdateRoom(
	ctx context.Context,
	room *repository.Room,
) error {
	if room.ID == "" {
		return errors.New("room ID is required")
	}

	if room.Name == "" {
		return errors.New("room name is required")
	}

	err := s.roomRepo.Update(ctx, room)
	if err != nil {
		return fmt.Errorf("update room: %w", err)
	}

	return nil
}

func (s *RoomService) DeleteRoom(
	ctx context.Context,
	id string,
) error {
	if id == "" {
		return errors.New("room ID is required")
	}

	room, err := s.roomRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get room before deletion: %w", err)
	}

	if room == nil {
		return errors.New("room not found")
	}

	if room.Status != "WAITING" {
		return errors.New("only waiting rooms can be deleted")
	}

	err = s.roomRepo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete room: %w", err)
	}

	return nil
}
