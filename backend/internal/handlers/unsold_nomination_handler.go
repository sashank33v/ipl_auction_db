package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type UnsoldNominationHandler struct {
	nominationService  *services.UnsoldNominationService
	auctionService     *services.AuctionService
	participantService *services.RoomParticipantService
}

func NewUnsoldNominationHandler(
	nominationService *services.UnsoldNominationService,
	auctionService *services.AuctionService,
	participantService *services.RoomParticipantService,
) *UnsoldNominationHandler {
	return &UnsoldNominationHandler{
		nominationService:  nominationService,
		auctionService:     auctionService,
		participantService: participantService,
	}
}

func (h *UnsoldNominationHandler) GetNominationsByRound(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "auctionID")
	roundValue := chi.URLParam(r, "roundNumber")

	if auctionID == "" || roundValue == "" {
		http.Error(w, "Auction ID and round number are required", http.StatusBadRequest)
		return
	}

	roundNumber, err := strconv.Atoi(roundValue)
	if err != nil || roundNumber < 1 {
		http.Error(w, "Invalid round number", http.StatusBadRequest)
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

	nominations, err := h.nominationService.GetNominationsByRound(
		r.Context(),
		auctionID,
		roundNumber,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, nominations)
}

func (h *UnsoldNominationHandler) GetNominationsByTeamAndRound(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	auctionID := chi.URLParam(r, "auctionID")
	teamID := chi.URLParam(r, "teamID")
	roundValue := chi.URLParam(r, "roundNumber")

	if auctionID == "" || teamID == "" || roundValue == "" {
		http.Error(w, "Auction ID, team ID and round number are required", http.StatusBadRequest)
		return
	}

	roundNumber, err := strconv.Atoi(roundValue)
	if err != nil || roundNumber < 1 {
		http.Error(w, "Invalid round number", http.StatusBadRequest)
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

	nominations, err := h.nominationService.GetNominationsByRound(
		r.Context(),
		auctionID,
		roundNumber,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve nominations", http.StatusInternalServerError)
		return
	}

	teamNominations := make([]repository.UnsoldNomination, 0)

	for _, nomination := range nominations {
		if nomination.TeamID == teamID {
			teamNominations = append(teamNominations, nomination)
		}
	}

	writeJSON(w, http.StatusOK, teamNominations)
}
