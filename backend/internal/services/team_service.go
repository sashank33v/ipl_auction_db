package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type TeamService struct {
	teamRepo        *repository.TeamRepository
	roomRepo        *repository.RoomRepository
	participantRepo *repository.RoomParticipantRepository
}

func NewTeamService(
	teamRepo *repository.TeamRepository,
	roomRepo *repository.RoomRepository,
	participantRepo *repository.RoomParticipantRepository,
) *TeamService {
	return &TeamService{
		teamRepo:        teamRepo,
		roomRepo:        roomRepo,
		participantRepo: participantRepo,
	}
}

func (s *TeamService) CreateTeam(
	ctx context.Context,
	team *repository.Team,
) error {
	if team.RoomID == "" || team.ParticipantID == "" {
		return errors.New("room ID and participant ID are required")
	}

	if strings.TrimSpace(team.FranchiseCode) == "" ||
		strings.TrimSpace(team.FranchiseName) == "" {
		return errors.New("franchise code and name are required")
	}

	room, err := s.roomRepo.GetByID(ctx, team.RoomID)
	if err != nil {
		return fmt.Errorf("get room: %w", err)
	}

	if room == nil {
		return errors.New("room not found")
	}

	if room.Status != "WAITING" {
		return errors.New("team selection is closed")
	}

	participant, err := s.participantRepo.GetByID(
		ctx, team.ParticipantID,
	)
	if err != nil {
		return fmt.Errorf("get participant: %w", err)
	}

	if participant == nil || participant.RoomID != team.RoomID {
		return errors.New("participant does not belong to this room")
	}

	existing, err := s.teamRepo.GetByParticipant(
		ctx, team.RoomID, team.ParticipantID,
	)
	if err != nil {
		return fmt.Errorf("check participant team: %w", err)
	}

	if existing != nil {
		return errors.New("participant has already selected a team")
	}

	teams, err := s.teamRepo.GetByRoom(ctx, team.RoomID)
	if err != nil {
		return fmt.Errorf("get room teams: %w", err)
	}

	if len(teams) >= room.MaxParticipants {
		return errors.New("all team slots are already assigned")
	}

	team.RemainingPurse = room.StartingPurse

	if err := s.teamRepo.Create(ctx, team); err != nil {
		return fmt.Errorf("create team: %w", err)
	}

	return nil
}

func (s *TeamService) GetTeamByID(
	ctx context.Context,
	id string,
) (*repository.Team, error) {
	if id == "" {
		return nil, errors.New("team ID is required")
	}

	team, err := s.teamRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}

	return team, nil
}

func (s *TeamService) GetTeamByParticipant(
	ctx context.Context,
	roomID string,
	participantID string,
) (*repository.Team, error) {
	if roomID == "" || participantID == "" {
		return nil, errors.New("room ID and participant ID are required")
	}

	team, err := s.teamRepo.GetByParticipant(
		ctx, roomID, participantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get participant team: %w", err)
	}

	return team, nil
}

func (s *TeamService) GetTeamsByRoom(
	ctx context.Context,
	roomID string,
) ([]repository.Team, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	teams, err := s.teamRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room teams: %w", err)
	}

	return teams, nil
}
