package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type AuctionEventHandler struct {
	eventService       *services.AuctionEventService
	auctionService     *services.AuctionService
	participantService *services.RoomParticipantService
}

func NewAuctionEventHandler(
	eventService *services.AuctionEventService,
	auctionService *services.AuctionService,
	participantService *services.RoomParticipantService,
) *AuctionEventHandler {
	return &AuctionEventHandler{
		eventService:       eventService,
		auctionService:     auctionService,
		participantService: participantService,
	}
}

func (h *AuctionEventHandler) GetEventBySequence(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "auctionID")
	sequenceValue := chi.URLParam(r, "sequenceNumber")

	if auctionID == "" || sequenceValue == "" {
		http.Error(w, "Auction ID and sequence number are required", http.StatusBadRequest)
		return
	}

	sequenceNumber, err := strconv.ParseInt(sequenceValue, 10, 64)
	if err != nil || sequenceNumber < 1 {
		http.Error(w, "Invalid sequence number", http.StatusBadRequest)
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

	event, err := h.eventService.GetEventBySequence(
		r.Context(),
		auctionID,
		sequenceNumber,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve auction event", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, event)
}

func (h *AuctionEventHandler) GetEventsAfterSequence(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "auctionID")
	sequenceValue := r.URL.Query().Get("after")

	if auctionID == "" || sequenceValue == "" {
		http.Error(w, "Auction ID and after sequence are required", http.StatusBadRequest)
		return
	}

	sequenceNumber, err := strconv.ParseInt(sequenceValue, 10, 64)
	if err != nil || sequenceNumber < 0 {
		http.Error(w, "Invalid sequence number", http.StatusBadRequest)
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

	events, err := h.eventService.GetEventsAfterSequence(
		r.Context(),
		auctionID,
		sequenceNumber,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve auction events", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, events)
}
