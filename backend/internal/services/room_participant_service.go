package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type RoomParticipantService struct {
	participantRepo *repository.RoomParticipantRepository
	roomRepo        *repository.RoomRepository
}

func NewRoomParticipantService(
	participantRepo *repository.RoomParticipantRepository,
	roomRepo *repository.RoomRepository,
) *RoomParticipantService {
	return &RoomParticipantService{
		participantRepo: participantRepo,
		roomRepo:        roomRepo,
	}
}

func (s *RoomParticipantService) JoinRoom(
	ctx context.Context,
	roomID string,
	userID string,
) (*repository.RoomParticipant, error) {
	if roomID == "" || userID == "" {
		return nil, errors.New("room ID and user ID are required")
	}

	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}

	if room == nil {
		return nil, errors.New("room not found")
	}

	if room.Status != "WAITING" {
		return nil, errors.New("room is not accepting participants")
	}

	existing, err := s.participantRepo.GetByRoomAndUser(
		ctx, roomID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("check participant: %w", err)
	}

	if existing != nil {
		return existing, nil
	}

	participants, err := s.participantRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room participants: %w", err)
	}

	if len(participants) >= room.MaxParticipants {
		return nil, errors.New("room is full")
	}

	participant := &repository.RoomParticipant{
		RoomID:           roomID,
		UserID:           userID,
		ConnectionStatus: "DISCONNECTED",
	}

	if err := s.participantRepo.Create(ctx, participant); err != nil {
		return nil, fmt.Errorf("join room: %w", err)
	}

	return participant, nil
}

func (s *RoomParticipantService) GetParticipantByID(
	ctx context.Context,
	id string,
) (*repository.RoomParticipant, error) {
	if id == "" {
		return nil, errors.New("participant ID is required")
	}

	participant, err := s.participantRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get participant: %w", err)
	}

	return participant, nil
}

func (s *RoomParticipantService) GetRoomParticipants(
	ctx context.Context,
	roomID string,
) ([]repository.RoomParticipant, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	participants, err := s.participantRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room participants: %w", err)
	}

	return participants, nil
}

func (s *RoomParticipantService) UpdateConnectionStatus(
	ctx context.Context,
	participantID string,
	status string,
) error {
	if participantID == "" {
		return errors.New("participant ID is required")
	}

	if status != "CONNECTED" && status != "DISCONNECTED" {
		return errors.New("invalid connection status")
	}

	return s.participantRepo.UpdateConnectionStatus(
		ctx, participantID, status,
	)
}
