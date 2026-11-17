package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type AuctionHandler struct {
	auctionService     *services.AuctionService
	participantService *services.RoomParticipantService
	roomService        *services.RoomService
}

func NewAuctionHandler(
	auctionService *services.AuctionService,
	participantService *services.RoomParticipantService,
	roomService *services.RoomService,
) *AuctionHandler {
	return &AuctionHandler{
		auctionService:     auctionService,
		participantService: participantService,
		roomService:        roomService,
	}
}

func (h *AuctionHandler) GetAuctionByRoom(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomID := chi.URLParam(r, "roomID")
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		roomID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve room participants", http.StatusInternalServerError)
		return
	}

	isMember := false
	for _, participant := range participants {
		if participant.UserID == userID {
			isMember = true
			break
		}
	}

	if !isMember {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	auction, err := h.auctionService.GetAuctionByRoom(
		r.Context(),
		roomID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve auction", http.StatusInternalServerError)
		return
	}

	if auction == nil {
		http.Error(w, "Auction not found for this room", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, auction)
}

func (h *AuctionHandler) GetAuctionByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "id")
	if auctionID == "" {
		http.Error(w, "Auction ID is required", http.StatusBadRequest)
		return
	}

	auction, err := h.auctionService.GetAuctionByID(
		r.Context(),
		auctionID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve auction", http.StatusInternalServerError)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		auction.RoomID,
	)
	if err != nil {
		http.Error(w, "Failed to verify room membership", http.StatusInternalServerError)
		return
	}

	isMember := false
	for _, participant := range participants {
		if participant.UserID == userID {
			isMember = true
			break
		}
	}

	if !isMember {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, auction)
}

func (h *AuctionHandler) CreateAuction(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomID := chi.URLParam(r, "roomID")
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}

	room, err := h.roomService.GetRoomByID(r.Context(), roomID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Room not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve room", http.StatusInternalServerError)
		return
	}

	if room.HostUserID != userID {
		http.Error(w, "Only the room host can create the auction", http.StatusForbidden)
		return
	}

	auction, err := h.auctionService.CreateAuction(
		r.Context(),
		roomID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") ||
			strings.Contains(err.Error(), "waiting rooms") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		http.Error(w, "Failed to create auction", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, auction)
}
