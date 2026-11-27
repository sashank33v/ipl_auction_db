package api

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/handlers"
	appmiddleware "github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

func NewRouter(frontendURL string, authHandler *handlers.AuthHandler, meHandler *handlers.MeHandler, roomHandler *handlers.RoomHandler, participantHandler *handlers.RoomParticipantHandler, teamHandler *handlers.TeamHandler, playerHandler *handlers.PlayerHandler, roomPlayerHandler *handlers.RoomPlayerHandler, auctionHandler *handlers.AuctionHandler, bidHandler *handlers.BidHandler, squadHandler *handlers.SquadHandler, nominationHandler *handlers.UnsoldNominationHandler, mainNominationHandler *handlers.MainNominationHandler, eventHandler *handlers.AuctionEventHandler, sessionService *services.SessionService, userService *services.UserService, realtimeHandler *handlers.RealtimeHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(appmiddleware.CORS(frontendURL))
	r.Use(appmiddleware.Logger)
	r.Use(appmiddleware.Recovery)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	r.Route("/auth", func(r chi.Router) {
		r.Use(chimiddleware.Timeout(30 * time.Second))
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)
	})

	// A WebSocket connection must not inherit the REST 30-second timeout.
	r.With(appmiddleware.Auth(sessionService)).Get("/ws/auctions/{auctionID}", realtimeHandler.ServeHTTP)
	r.Group(func(r chi.Router) {
		r.Use(chimiddleware.Timeout(30 * time.Second))
		r.Use(appmiddleware.Auth(sessionService))

		r.Get("/auth/me", meHandler.GetMe)

		r.Route("/rooms", func(r chi.Router) {
			r.Post("/", roomHandler.CreateRoom)
			r.Get("/", roomHandler.GetAllRooms)
			r.Get("/{id}", roomHandler.GetRoomByID)
			r.Put("/{id}", roomHandler.UpdateRoom)
			r.Delete("/{id}", roomHandler.DeleteRoom)

			r.Post("/join", participantHandler.JoinRoom)
			r.Get("/{id}/participants", participantHandler.GetRoomParticipants)

			r.Get("/{roomID}/teams", teamHandler.GetTeamsByRoom)
			r.Get(
				"/{roomID}/participants/{participantID}/team",
				teamHandler.GetTeamByParticipant,
			)

			r.Post("/players", roomPlayerHandler.AddPlayerToRoom)
			r.Get("/{roomID}/players", roomPlayerHandler.GetPlayersByRoom)
			r.Post("/{roomID}/players/queue", roomPlayerHandler.PrepareAuctionQueue)
			r.Get(
				"/{roomID}/players/status/{status}",
				roomPlayerHandler.GetPlayersByStatus,
			)
			r.Get(
				"/{roomID}/players/category/{category}",
				roomPlayerHandler.GetAvailablePlayersByCategory,
			)

			r.Post("/{roomID}/auction", auctionHandler.CreateAuction)
			r.Get("/{roomID}/auction", auctionHandler.GetAuctionByRoom)

			r.Get("/{roomID}/squads", squadHandler.GetSquadsByRoom)
			r.Post("/{roomID}/auction/start", auctionHandler.StartAuction)
		})

		r.Route("/participants", func(r chi.Router) {
			r.Get("/{id}", participantHandler.GetParticipantByID)
			r.Put(
				"/{id}/connection-status",
				participantHandler.UpdateConnectionStatus,
			)
		})

		r.Route("/teams", func(r chi.Router) {
			r.Post("/", teamHandler.CreateTeam)
			r.Get("/{id}", teamHandler.GetTeamByID)
			r.Get("/{teamID}/squad", squadHandler.GetSquadByTeam)
		})

		r.Route("/players", func(r chi.Router) {
			r.Get("/", playerHandler.GetAllPlayers)
			r.Get("/{id}", playerHandler.GetPlayerByID)
			// Legacy write routes are retained for compatibility, but require admin permission.
			r.With(appmiddleware.AdminOnly(userService.IsAdmin)).Post("/", playerHandler.CreatePlayer)
			r.With(appmiddleware.AdminOnly(userService.IsAdmin)).Put("/{id}", playerHandler.UpdatePlayer)
			r.With(appmiddleware.AdminOnly(userService.IsAdmin)).Delete("/{id}", playerHandler.DeletePlayer)
		})

		r.Route("/admin/players", func(r chi.Router) {
			r.Use(appmiddleware.AdminOnly(userService.IsAdmin))
			r.Post("/", playerHandler.CreatePlayer)
			r.Get("/", playerHandler.GetAllPlayers)
			r.Get("/{id}", playerHandler.GetPlayerByID)
			r.Put("/{id}", playerHandler.UpdatePlayer)
			r.Delete("/{id}", playerHandler.DeletePlayer)
		})

		r.Route("/room-players", func(r chi.Router) {
			r.Get("/{id}", roomPlayerHandler.GetRoomPlayerByID)
		})

		r.Route("/auctions", func(r chi.Router) {
			r.Get("/{id}", auctionHandler.GetAuctionByID)

			r.Post("/{roomID}", auctionHandler.CreateAuction)

			r.Get("/{auctionID}/bids", bidHandler.GetAuctionBids)
			r.Post("/{auctionID}/bids", bidHandler.PlaceBid)
			r.Get(
				"/{auctionID}/bids/request/{requestID}",
				bidHandler.GetBidByRequestID,
			)

			r.Get(
				"/{auctionID}/unsold/rounds/{roundNumber}",
				nominationHandler.GetNominationsByRound,
			)
			r.Get(
				"/{auctionID}/unsold/rounds/{roundNumber}/teams/{teamID}",
				nominationHandler.GetNominationsByTeamAndRound,
			)

			r.Get(
				"/{auctionID}/events/{sequenceNumber}",
				eventHandler.GetEventBySequence,
			)
			r.Get(
				"/{auctionID}/events",
				eventHandler.GetEventsAfterSequence,
			)
			// Main queue nominations are now server-driven.
			r.Post("/{auctionID}/recover/resume", auctionHandler.ResumeRecoveredAuction)
		})
		r.Get("/bids/{id}", bidHandler.GetBidByID)

		r.Route("/squads", func(r chi.Router) {
			r.Get("/{id}", squadHandler.GetSquadByID)
		})
	})

	return r
}
