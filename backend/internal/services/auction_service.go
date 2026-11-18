package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
)

type AuctionService struct {
	auctionRepo        *repository.AuctionRepository
	roomRepo           *repository.RoomRepository
	participantService *RoomParticipantService
	teamService        *TeamService
}

func NewAuctionService(
	auctionRepo *repository.AuctionRepository,
	roomRepo *repository.RoomRepository,
	participantService *RoomParticipantService,
	teamService *TeamService,
) *AuctionService {
	return &AuctionService{
		auctionRepo:        auctionRepo,
		roomRepo:           roomRepo,
		participantService: participantService,
		teamService:        teamService,
	}
}

func (s *AuctionService) CreateAuction(
	ctx context.Context,
	roomID string,
) (*repository.Auction, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	if room == nil {
		return nil, errors.New("room not found")
	}

	if room.Status != "WAITING" {
		return nil, errors.New("auction can only be created for waiting rooms")
	}

	existing, err := s.auctionRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("check existing auction: %w", err)
	}
	if existing != nil {
		return nil, errors.New("auction already exists for this room")
	}

	auction := &repository.Auction{
		RoomID:             roomID,
		Status:             "WAITING",
		CurrentRoundNumber: 1,
		RoundType:          "MAIN",
	}

	if err := s.auctionRepo.Create(ctx, auction); err != nil {
		return nil, fmt.Errorf("create auction: %w", err)
	}

	return auction, nil
}

func (s *AuctionService) GetAuctionByID(
	ctx context.Context,
	id string,
) (*repository.Auction, error) {
	if id == "" {
		return nil, errors.New("auction ID is required")
	}

	auction, err := s.auctionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return nil, errors.New("auction not found")
	}

	return auction, nil
}

func (s *AuctionService) GetAuctionByRoom(
	ctx context.Context,
	roomID string,
) (*repository.Auction, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	auction, err := s.auctionRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room auction: %w", err)
	}

	return auction, nil
}

func (s *AuctionService) StartAuction(
	ctx context.Context,
	roomID string,
	userID string,
) (*repository.Auction, error) {

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

	if room.HostUserID != userID {
		return nil, errors.New("only room host can start auction")
	}

	if room.Status != "WAITING" {
		return nil, errors.New("room is not in waiting state")
	}

	auction, err := s.auctionRepo.GetByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if auction == nil {
		return nil, errors.New("auction not found")
	}

	if auction.Status != "WAITING" {
		return nil, errors.New("auction is not in waiting state")
	}

	participants, err := s.participantService.GetRoomParticipants(
		ctx,
		roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}

	if len(participants) < 4 {
		return nil, errors.New("minimum 4 participants required")
	}

	teams, err := s.teamService.GetTeamsByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get teams: %w", err)
	}

	if len(teams) < 4 {
		return nil, errors.New("minimum 4 teams required")
	}

	if err := s.auctionRepo.StartAuction(ctx, auction.ID); err != nil {
		return nil, fmt.Errorf("start auction: %w", err)
	}

	auction, err = s.auctionRepo.GetByID(ctx, auction.ID)
	if err != nil {
		return nil, fmt.Errorf("get updated auction: %w", err)
	}

	return auction, nil
}
