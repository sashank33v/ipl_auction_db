package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/auction"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

func (h *RoomPlayerHandler) PrepareAuctionQueue(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	var opts auction.QueueOptions
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		http.Error(w, "Invalid request body; use {} for default order", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		http.Error(w, "Expected one JSON object", http.StatusBadRequest)
		return
	}
	opts, err := auction.ValidateQueueOptions(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	players, err := h.roomPlayerService.PrepareAuctionQueue(r.Context(), chi.URLParam(r, "roomID"), userID, opts)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrQueueRoomNotFound):
			http.Error(w, "Room not found", http.StatusNotFound)
		case errors.Is(err, services.ErrQueueHostOnly):
			http.Error(w, "Only room host can prepare queue", http.StatusForbidden)
		case errors.Is(err, repository.ErrQueueConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			log.Printf("prepare queue: %v", err)
			http.Error(w, "Failed to prepare queue", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(players), "players": players})
}
