package handlers

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type BidHandler struct {
	bidService         *services.BidService
	auctionService     *services.AuctionService
	participantService *services.RoomParticipantService
	teamService        *services.TeamService
	bidWorkflowService *services.BidWorkflowService
}

func NewBidHandler(
	bidService *services.BidService,
	auctionService *services.AuctionService,
	participantService *services.RoomParticipantService,
	teamService *services.TeamService,
	bidWorkflowService *services.BidWorkflowService,
) *BidHandler {
	return &BidHandler{
		bidService:         bidService,
		auctionService:     auctionService,
		participantService: participantService,
		teamService:        teamService,
		bidWorkflowService: bidWorkflowService,
	}
}

type PlaceBidRequest struct {
	RoomPlayerID string `json:"room_player_id"`
	BidAmount    string `json:"bid_amount"`
	RequestID    string `json:"request_id"`
}

// PlaceBid accepts a bid from the authenticated user's own team.
func (h *BidHandler) PlaceBid(
	w http.ResponseWriter,
	r *http.Request,
) {
	// 1. Get authenticated user
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// 2. Read auction ID
	auctionID := chi.URLParam(r, "auctionID")
	if auctionID == "" {
		http.Error(w, "Auction ID is required", http.StatusBadRequest)
		return
	}

	// 3. Decode request
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req PlaceBidRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "Request must contain only one JSON object", http.StatusBadRequest)
		return
	}

	req.RoomPlayerID = strings.TrimSpace(req.RoomPlayerID)
	req.BidAmount = strings.TrimSpace(req.BidAmount)
	req.RequestID = strings.TrimSpace(req.RequestID)

	if req.RoomPlayerID == "" ||
		req.BidAmount == "" ||
		req.RequestID == "" {
		http.Error(
			w,
			"Room player ID, bid amount and request ID are required",
			http.StatusBadRequest,
		)
		return
	}

	// 4. Validate numeric bid amount
	amount, err := strconv.ParseFloat(req.BidAmount, 64)
	if err != nil || amount <= 0 ||
		math.IsNaN(amount) || math.IsInf(amount, 0) {
		http.Error(w, "Invalid bid amount", http.StatusBadRequest)
		return
	}

	// 5. Retrieve auction
	auction, err := h.auctionService.GetAuctionByID(
		r.Context(),
		auctionID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Auction not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve auction", http.StatusInternalServerError)
		return
	}

	if auction == nil {
		http.Error(w, "Auction not found", http.StatusNotFound)
		return
	}

	// 6. Find authenticated user's room participant
	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		auction.RoomID,
	)
	if err != nil {
		http.Error(w, "Failed to verify room membership", http.StatusInternalServerError)
		return
	}

	var participantID string

	for _, participant := range participants {
		if participant.UserID == userID {
			participantID = participant.ID
			break
		}
	}

	if participantID == "" {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	// 7. Resolve the authenticated participant's team
	team, err := h.teamService.GetTeamByParticipant(
		r.Context(),
		auction.RoomID,
		participantID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve your team", http.StatusInternalServerError)
		return
	}

	if team == nil {
		http.Error(w, "Select a team before placing bids", http.StatusConflict)
		return
	}

	// 8. Build server-controlled event payload
	eventPayload, err := json.Marshal(struct {
		RoomPlayerID string `json:"room_player_id"`
		TeamID       string `json:"team_id"`
		Amount       string `json:"amount"`
	}{
		RoomPlayerID: req.RoomPlayerID,
		TeamID:       team.ID,
		Amount:       req.BidAmount,
	})
	if err != nil {
		http.Error(w, "Failed to prepare bid event", http.StatusInternalServerError)
		return
	}

	// 9. Execute transactional bid workflow
	bid, err := h.bidWorkflowService.AcceptBid(
		r.Context(),
		repository.AcceptBidParams{
			AuctionID:    auctionID,
			RoomPlayerID: req.RoomPlayerID,
			TeamID:       team.ID,
			BidAmount:    req.BidAmount,
			RequestID:    req.RequestID,
			EventPayload: eventPayload,
		},
	)
	if err != nil {
		message := err.Error()

		if strings.Contains(message, "auction not found") {
			http.Error(w, "Auction not found", http.StatusNotFound)
			return
		}

		if strings.Contains(message, "requires a transaction") ||
			strings.Contains(message, "not configured") {
			http.Error(w, "Bid workflow is unavailable", http.StatusInternalServerError)
			return
		}

		if strings.Contains(message, "accept bid workflow:") {
			http.Error(w, "Bid rejected: "+message, http.StatusConflict)
			return
		}

		http.Error(w, "Failed to place bid", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, bid)
}

// Existing GET handlers remain unchanged.

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

	auction, err := h.auctionService.GetAuctionByID(r.Context(), bid.AuctionID)
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
