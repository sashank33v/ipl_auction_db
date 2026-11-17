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

var allowedFranchises = map[string]string{
	"CSK":  "Chennai Super Kings",
	"DC":   "Delhi Capitals",
	"GT":   "Gujarat Titans",
	"KKR":  "Kolkata Knight Riders",
	"LSG":  "Lucknow Super Giants",
	"MI":   "Mumbai Indians",
	"PBKS": "Punjab Kings",
	"RR":   "Rajasthan Royals",
	"RCB":  "Royal Challengers Bengaluru",
	"SRH":  "Sunrisers Hyderabad",
}

func (s *TeamService) CreateTeam(
	ctx context.Context,
	team *repository.Team,
) error {
	if team == nil {
		return errors.New("team is required")
	}

	if team.RoomID == "" || team.ParticipantID == "" {
		return errors.New("room ID and participant ID are required")
	}

	team.FranchiseCode = strings.ToUpper(
		strings.TrimSpace(team.FranchiseCode),
	)

	if team.FranchiseCode == "" {
		return errors.New("franchise code is required")
	}

	franchiseName, exists := allowedFranchises[team.FranchiseCode]
	if !exists {
		return errors.New("invalid franchise code")
	}

	team.FranchiseName = franchiseName

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
		ctx,
		team.ParticipantID,
	)
	if err != nil {
		return fmt.Errorf("get participant: %w", err)
	}
	if participant == nil || participant.RoomID != team.RoomID {
		return errors.New("participant does not belong to this room")
	}

	existing, err := s.teamRepo.GetByParticipant(
		ctx,
		team.RoomID,
		team.ParticipantID,
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

	if len(teams) >= int(room.MaxParticipants) {
		return errors.New("all team slots are already assigned")
	}

	for _, existingTeam := range teams {
		if existingTeam.FranchiseCode == team.FranchiseCode {
			return errors.New("franchise has already been selected in this room")
		}
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
	if team == nil {
		return nil, errors.New("team not found")
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
		ctx,
		roomID,
		participantID,
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
