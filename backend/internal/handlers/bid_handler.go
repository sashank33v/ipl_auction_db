package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/sashank33v/ipl_auction_db/backend/internal/rooms"
	"io"
	"math"
	"net/http"
	"regexp"
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

var bidUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var bidMoney = regexp.MustCompile(`^[0-9]{1,4}(\.[0-9]{1,2})?$`)

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

	bid, status, err := h.placeAuthenticatedBid(r.Context(), userID, auctionID, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, bid)
}

// Shared REST and WebSocket bid path. Team identity is resolved on the server.
func (h *BidHandler) placeAuthenticatedBid(ctx context.Context, userID, auctionID string, req PlaceBidRequest) (*repository.Bid, int, error) {
	req.RoomPlayerID = strings.ToLower(strings.TrimSpace(req.RoomPlayerID))
	req.BidAmount = strings.TrimSpace(req.BidAmount)
	req.RequestID = strings.ToLower(strings.TrimSpace(req.RequestID))

	if req.RoomPlayerID == "" ||
		req.BidAmount == "" ||
		req.RequestID == "" {
		return nil, http.StatusBadRequest, errors.New("Room player ID, bid amount and request ID are required")
	}

	if !bidUUID.MatchString(auctionID) || !bidUUID.MatchString(req.RoomPlayerID) || !bidUUID.MatchString(req.RequestID) {
		return nil, http.StatusBadRequest, errors.New("Auction, room player and request IDs must be UUIDs")
	}
	if !bidMoney.MatchString(req.BidAmount) {
		return nil, http.StatusBadRequest, errors.New("Bid amount must be a decimal in crores with at most two decimal places")
	}
	// 4. Validate numeric bid amount
	amount, err := strconv.ParseFloat(req.BidAmount, 64)
	if err != nil || amount <= 0 ||
		math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, http.StatusBadRequest, errors.New("Invalid bid amount")
	}

	// 5. Retrieve auction
	auction, err := h.auctionService.GetAuctionByID(
		ctx,
		auctionID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, http.StatusNotFound, errors.New("Auction not found")
		}

		return nil, http.StatusInternalServerError, errors.New("Failed to retrieve auction")
	}

	if auction == nil {
		return nil, http.StatusNotFound, errors.New("Auction not found")
	}

	// 6. Find authenticated user's room participant
	participants, err := h.participantService.GetRoomParticipants(
		ctx,
		auction.RoomID,
	)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("Failed to verify room membership")
	}

	var participantID string

	for _, participant := range participants {
		if participant.UserID == userID {
			participantID = participant.ID
			break
		}
	}

	if participantID == "" {
		return nil, http.StatusForbidden, errors.New("You are not a member of this room")
	}

	// 7. Resolve the authenticated participant's team
	team, err := h.teamService.GetTeamByParticipant(
		ctx,
		auction.RoomID,
		participantID,
	)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("Failed to retrieve your team")
	}

	if team == nil {
		return nil, http.StatusConflict, errors.New("Select a team before placing bids")
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
		return nil, http.StatusInternalServerError, errors.New("Failed to prepare bid event")
	}

	// 9. Execute transactional bid workflow
	bid, err := h.bidWorkflowService.AcceptBid(
		ctx,
		repository.AcceptBidParams{
			AuctionID:    auction.ID,
			RoomPlayerID: req.RoomPlayerID,
			TeamID:       team.ID,
			BidAmount:    req.BidAmount,
			RequestID:    req.RequestID,
			EventPayload: eventPayload,
		},
	)
	if err != nil {
		if errors.Is(err, rooms.ErrBusy) {
			return nil, http.StatusServiceUnavailable, errors.New("Room is busy; retry with the same request ID")
		}
		message := err.Error()

		if strings.Contains(message, "auction not found") {
			return nil, http.StatusNotFound, errors.New("Auction not found")
		}

		if strings.Contains(message, "requires a transaction") ||
			strings.Contains(message, "not configured") {
			return nil, http.StatusInternalServerError, errors.New("Bid workflow is unavailable")
		}

		if strings.Contains(message, "accept bid workflow:") {
			return nil, http.StatusConflict, errors.New("Bid rejected by auction rules")
		}

		return nil, http.StatusInternalServerError, errors.New("Failed to place bid")
	}

	return bid, http.StatusOK, nil
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
