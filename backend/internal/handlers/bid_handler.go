package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type BidHandler struct {
	bidService         *services.BidService
	auctionService     *services.AuctionService
	participantService *services.RoomParticipantService
}

func NewBidHandler(
	bidService *services.BidService,
	auctionService *services.AuctionService,
	participantService *services.RoomParticipantService,
) *BidHandler {
	return &BidHandler{
		bidService:         bidService,
		auctionService:     auctionService,
		participantService: participantService,
	}
}

func (h *BidHandler) GetBidByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	bidID := chi.URLParam(r, "id")
	if bidID == "" {
		http.Error(w, "Bid ID is required", http.StatusBadRequest)
		return
	}

	bid, err := h.bidService.GetBidByID(r.Context(), bidID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if strings.Contains(err.Error(), "required") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "Failed to retrieve bid", http.StatusInternalServerError)
		return
	}

	auction, err := h.auctionService.GetAuctionByID(
		r.Context(),
		bid.AuctionID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve auction", http.StatusInternalServerError)
		return
	}

	if auction == nil {
		http.Error(w, "Auction not found", http.StatusNotFound)
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

	writeJSON(w, http.StatusOK, bid)
}

func (h *BidHandler) GetBidByRequestID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "auctionID")
	requestID := chi.URLParam(r, "requestID")

	if auctionID == "" || requestID == "" {
		http.Error(w, "Auction ID and request ID are required", http.StatusBadRequest)
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

	bid, err := h.bidService.GetBidByRequestID(
		r.Context(),
		auctionID,
		requestID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve bid", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, bid)
}

func (h *BidHandler) GetAuctionBids(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "auctionID")
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

	bids, err := h.bidService.GetAuctionBids(
		r.Context(),
		auctionID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve auction bids", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, bids)
}
