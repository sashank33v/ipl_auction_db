package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sashank33v/ipl_auction_db/backend/internal/api"
	"github.com/sashank33v/ipl_auction_db/backend/internal/config"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
	"github.com/sashank33v/ipl_auction_db/backend/internal/handlers"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/rooms"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	db, err := database.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	actors := rooms.NewManager(ctx)
	txManager := database.NewTransactionManager(db)
	bidWorkflowService := services.NewBidWorkflowService(txManager)
	bidWorkflowService.SetCommandManager(actors)
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	participantRepo := repository.NewRoomParticipantRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	playerRepo := repository.NewPlayerRepository(db)
	roomPlayerRepo := repository.NewRoomPlayerRepository(db)
	auctionRepo := repository.NewAuctionRepository(db)
	bidRepo := repository.NewBidRepository(db)
	squadRepo := repository.NewSquadRepository(db)
	nominationRepo := repository.NewUnsoldNominationRepository(db)
	eventRepo := repository.NewAuctionEventRepository(db)

	nominationWorkflowService := services.NewUnsoldNominationWorkflowService(txManager)
	userService := services.NewUserService(userRepo)
	sessionService := services.NewSessionService(sessionRepo)
	roomService := services.NewRoomService(txManager, roomRepo)
	participantService := services.NewRoomParticipantService(txManager, participantRepo, roomRepo)
	teamService := services.NewTeamService(txManager, teamRepo, roomRepo, participantRepo)
	playerService := services.NewPlayerService(playerRepo)
	roomPlayerService := services.NewRoomPlayerService(txManager, roomPlayerRepo, roomRepo, playerRepo)
	auctionService := services.NewAuctionService(txManager, auctionRepo, roomRepo, participantService, teamService)
	auctionService.SetCommandManager(actors)
	bidService := services.NewBidService(bidRepo, auctionRepo)
	squadService := services.NewSquadService(squadRepo, teamRepo)
	nominationService := services.NewUnsoldNominationService(nominationRepo, auctionRepo, teamRepo, roomPlayerRepo, nominationWorkflowService)
	eventService := services.NewAuctionEventService(eventRepo, auctionRepo)
	mainNominationWorkflowService := services.NewMainNominationWorkflowService(txManager)

	authHandler := handlers.NewAuthHandler(userService, sessionService)
	meHandler := handlers.NewMeHandler(userService)
	roomHandler := handlers.NewRoomHandler(roomService)
	participantHandler := handlers.NewRoomParticipantHandler(participantService)
	teamHandler := handlers.NewTeamHandler(teamService, participantService)
	playerHandler := handlers.NewPlayerHandler(playerService)
	roomPlayerHandler := handlers.NewRoomPlayerHandler(roomPlayerService, participantService, roomService)
	auctionHandler := handlers.NewAuctionHandler(auctionService, participantService, roomService)
	bidHandler := handlers.NewBidHandler(bidService, auctionService, participantService, teamService, bidWorkflowService)
	squadHandler := handlers.NewSquadHandler(squadService, teamService, participantService)
	nominationHandler := handlers.NewUnsoldNominationHandler(nominationService, auctionService, participantService)
	eventHandler := handlers.NewAuctionEventHandler(eventService, auctionService, participantService)
	mainNominationHandler := handlers.NewMainNominationHandler(mainNominationWorkflowService)

	realtimeHandler := handlers.NewRealtimeHandler(ctx, repository.NewRealtimeRepository(db), sessionService, bidHandler, cfg.FrontendURL)
	router := api.NewRouter(cfg.FrontendURL, authHandler, meHandler, roomHandler, participantHandler, teamHandler, playerHandler, roomPlayerHandler, auctionHandler, bidHandler, squadHandler, nominationHandler, mainNominationHandler, eventHandler, sessionService, userService, realtimeHandler)

	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}
	engine := services.NewAuctionEngineService(txManager, repository.NewAuctionEngineRepository(db))
	engine.SetCommandManager(actors)
	recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 10*time.Second)
	recovered, err := engine.RecoverOnStartup(recoveryCtx)
	recoveryCancel()
	if err != nil {
		log.Fatalf("Auction recovery gate failed: %v", err)
	}
	log.Printf("Auction engine started; %d interrupted auctions require host resume", recovered)
	engineDone := make(chan struct{})
	go func() { defer close(engineDone); engine.Run(ctx) }()
	go func() {
		log.Printf("Server starting on port http://localhost:%s", cfg.ServerPort)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()
	<-ctx.Done()

	log.Println("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			log.Printf("Forced server close failed: %v", closeErr)
		}
	}

	realtimeHandler.Close()
	<-engineDone
	actors.Close()
	log.Println("Server stopped")
}
