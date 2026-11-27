package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	auctionrules "github.com/sashank33v/ipl_auction_db/backend/internal/auction"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/rooms"
)

type AuctionService struct {
	actors             *rooms.Manager
	txManager          *database.TransactionManager
	auctionRepo        *repository.AuctionRepository
	roomPlayerRepo     *repository.RoomPlayerRepository
	roomRepo           *repository.RoomRepository
	participantService *RoomParticipantService
	teamService        *TeamService
}

func NewAuctionService(
	txManager *database.TransactionManager,
	auctionRepo *repository.AuctionRepository,
	roomRepo *repository.RoomRepository,
	participantService *RoomParticipantService,
	teamService *TeamService,
) *AuctionService {
	return &AuctionService{
		txManager:          txManager,
		auctionRepo:        auctionRepo,
		roomRepo:           roomRepo,
		participantService: participantService,
		teamService:        teamService,
	}
}

func (s *AuctionService) CreateAuction(ctx context.Context, roomID string) (*repository.Auction, error) {
	if s.txManager == nil {
		return nil, errors.New("transaction manager is not configured")
	}
	var result *repository.Auction
	err := s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		scoped := &AuctionService{
			auctionRepo: repository.NewAuctionRepository(tx),
			roomRepo:    repository.NewRoomRepository(tx),
		}
		var err error
		result, err = scoped.createAuctionLocked(ctx, roomID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *AuctionService) createAuctionLocked(
	ctx context.Context,
	roomID string,
) (*repository.Auction, error) {
	if roomID == "" {
		return nil, errors.New("room ID is required")
	}

	room, err := s.roomRepo.GetByIDForUpdate(ctx, roomID)
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

func (s *AuctionService) StartAuction(ctx context.Context, roomID, userID string) (*repository.Auction, error) {
	if s.txManager == nil {
		return nil, errors.New("transaction manager is not configured")
	}
	var result *repository.Auction
	err := s.txManager.WithinTransaction(ctx, func(tx pgx.Tx) error {
		scoped := &AuctionService{
			auctionRepo:        repository.NewAuctionRepository(tx),
			roomPlayerRepo:     repository.NewRoomPlayerRepository(tx),
			roomRepo:           repository.NewRoomRepository(tx),
			participantService: &RoomParticipantService{participantRepo: repository.NewRoomParticipantRepository(tx)},
			teamService:        &TeamService{teamRepo: repository.NewTeamRepository(tx)},
		}
		var err error
		result, err = scoped.startAuctionLocked(ctx, roomID, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *AuctionService) startAuctionLocked(
	ctx context.Context,
	roomID string,
	userID string,
) (*repository.Auction, error) {

	if roomID == "" || userID == "" {
		return nil, errors.New("room ID and user ID are required")
	}

	room, err := s.roomRepo.GetByIDForUpdate(ctx, roomID)
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

	if len(participants) < 4 || len(participants) > room.MaxParticipants {
		return nil, errors.New("participant count must be within room capacity (minimum 4)")
	}

	hostJoined := false
	for _, participant := range participants {
		if participant.UserID == userID {
			hostJoined = true
			break
		}
	}
	if !hostJoined {
		return nil, errors.New("host must join and select a franchise before starting")
	}

	teams, err := s.teamService.GetTeamsByRoom(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("get teams: %w", err)
	}

	if len(teams) != len(participants) {
		return nil, errors.New("every participant must select a franchise before starting")
	}

	if _, err := s.roomPlayerRepo.EnsureAuctionQueue(ctx, roomID, auctionrules.QueueOptions{}, false); err != nil {
		return nil, fmt.Errorf("prepare auction queue: %w", err)
	}

	if err := s.roomRepo.MarkInProgress(ctx, roomID); err != nil {
		return nil, fmt.Errorf("lock room: %w", err)
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
