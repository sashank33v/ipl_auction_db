package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type RoomPlayerHandler struct {
	roomPlayerService  *services.RoomPlayerService
	participantService *services.RoomParticipantService
	roomService        *services.RoomService
}

func NewRoomPlayerHandler(
	roomPlayerService *services.RoomPlayerService,
	participantService *services.RoomParticipantService,
	roomService *services.RoomService,
) *RoomPlayerHandler {
	return &RoomPlayerHandler{
		roomPlayerService:  roomPlayerService,
		participantService: participantService,
		roomService:        roomService,
	}
}

type AddPlayerToRoomRequest struct {
	RoomID   string `json:"room_id"`
	PlayerID string `json:"player_id"`
}

func (h *RoomPlayerHandler) AddPlayerToRoom(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req AddPlayerToRoomRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.RoomID = strings.TrimSpace(req.RoomID)
	req.PlayerID = strings.TrimSpace(req.PlayerID)

	if req.RoomID == "" || req.PlayerID == "" {
		http.Error(w, "Room ID and player ID are required", http.StatusBadRequest)
		return
	}

	room, err := h.roomService.GetRoomByID(r.Context(), req.RoomID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Room not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve room", http.StatusInternalServerError)
		return
	}

	if room.HostUserID != userID {
		http.Error(w, "Only the room host can add players", http.StatusForbidden)
		return
	}

	if err := h.roomPlayerService.AddPlayerToRoom(
		r.Context(),
		req.RoomID,
		req.PlayerID,
	); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if strings.Contains(err.Error(), "already exists") ||
			strings.Contains(err.Error(), "after auction starts") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message":   "Player added to room successfully",
		"room_id":   req.RoomID,
		"player_id": req.PlayerID,
	})
}

func (h *RoomPlayerHandler) GetRoomPlayerByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomPlayerID := chi.URLParam(r, "id")
	if roomPlayerID == "" {
		http.Error(w, "Room player ID is required", http.StatusBadRequest)
		return
	}

	roomPlayer, err := h.roomPlayerService.GetRoomPlayerByID(
		r.Context(),
		roomPlayerID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve room player", http.StatusInternalServerError)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		roomPlayer.RoomID,
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

	writeJSON(w, http.StatusOK, roomPlayer)
}

func (h *RoomPlayerHandler) GetPlayersByRoom(
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

	players, err := h.roomPlayerService.GetPlayersByRoom(
		r.Context(),
		roomID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve room players", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, players)
}

func (h *RoomPlayerHandler) GetPlayersByStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomID := chi.URLParam(r, "roomID")
	status := chi.URLParam(r, "status")

	if roomID == "" || status == "" {
		http.Error(w, "Room ID and status are required", http.StatusBadRequest)
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

	players, err := h.roomPlayerService.GetPlayersByStatus(
		r.Context(),
		roomID,
		status,
	)
	if err != nil {
		if strings.Contains(err.Error(), "invalid player status") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "Failed to retrieve players", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, players)
}
