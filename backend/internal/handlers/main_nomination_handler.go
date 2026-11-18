package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type MainNominationHandler struct {
	nominationService *services.MainNominationWorkflowService
}

func NewMainNominationHandler(
	nominationService *services.MainNominationWorkflowService,
) *MainNominationHandler {
	return &MainNominationHandler{
		nominationService: nominationService,
	}
}

type MainNominationRequest struct {
	RoomPlayerID string `json:"room_player_id"`
}

func (h *MainNominationHandler) NominatePlayer(
	w http.ResponseWriter,
	r *http.Request,
) {
	// 1. Get authenticated user.
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(
			w,
			"Authentication required",
			http.StatusUnauthorized,
		)
		return
	}

	// 2. Get auction ID from URL.
	auctionID := chi.URLParam(r, "auctionID")
	if auctionID == "" {
		http.Error(
			w,
			"Auction ID is required",
			http.StatusBadRequest,
		)
		return
	}

	// 3. Decode request body.
	var req MainNominationRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(
			w,
			"Invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if strings.TrimSpace(req.RoomPlayerID) == "" {
		http.Error(
			w,
			"Room player ID is required",
			http.StatusBadRequest,
		)
		return
	}

	// 4. Prepare nomination parameters.
	params := repository.MainNominationParams{
		AuctionID: auctionID,
		PlayerID:  req.RoomPlayerID,
		UserID:    userID,
	}

	// 5. Execute nomination workflow.
	result, err := h.nominationService.NominatePlayer(
		r.Context(),
		params,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if strings.Contains(err.Error(), "only the room host") {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		if strings.Contains(err.Error(), "required") ||
			strings.Contains(err.Error(), "not active") ||
			strings.Contains(err.Error(), "not accepting") ||
			strings.Contains(err.Error(), "already in auction") ||
			strings.Contains(err.Error(), "available") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(
			w,
			"Failed to nominate player",
			http.StatusInternalServerError,
		)
		return
	}

	// 6. Return successful nomination.
	writeJSON(
		w,
		http.StatusOK,
		result,
	)
}
