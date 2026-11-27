package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/rooms"
)

func (h *AuctionHandler) PauseAuction(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, true)
}
func (h *AuctionHandler) ResumeAuction(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, false)
}

func (h *AuctionHandler) setPaused(w http.ResponseWriter, r *http.Request, pause bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	id := strings.ToLower(chi.URLParam(r, "auctionID"))
	if !bidUUID.MatchString(id) {
		http.Error(w, "Auction ID must be a UUID", http.StatusBadRequest)
		return
	}
	state, err := h.auctionService.SetPaused(r.Context(), id, userID, pause)
	if err == nil {
		writeJSON(w, http.StatusOK, state)
		return
	}
	status, message := controlError(err)
	http.Error(w, message, status)
}

// Keep REST and WebSocket control errors consistent.
func controlError(err error) (int, string) {
	switch {
	case errors.Is(err, repository.ErrControlNotFound):
		return http.StatusNotFound, "Auction not found"
	case errors.Is(err, repository.ErrControlHostOnly):
		return http.StatusForbidden, "Only the room host can control the auction"
	case errors.Is(err, repository.ErrControlConflict):
		return http.StatusConflict, "Auction cannot perform this control in its current state"
	case errors.Is(err, rooms.ErrBusy), errors.Is(err, rooms.ErrClosed):
		return http.StatusServiceUnavailable, "Room command queue unavailable"
	default:
		log.Printf("auction control: %v", err)
		return http.StatusInternalServerError, "Failed to control auction"
	}
}
