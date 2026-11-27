package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type RoomParticipantHandler struct {
	participantService *services.RoomParticipantService
}

func NewRoomParticipantHandler(
	participantService *services.RoomParticipantService,
) *RoomParticipantHandler {
	return &RoomParticipantHandler{
		participantService: participantService,
	}
}

type JoinRoomRequest struct {
	RoomCode string `json:"room_code"`
}

func (h *RoomParticipantHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {

	userID, ok := middleware.UserIDFromContext(r.Context())

	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req JoinRoomRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	roomCode := strings.TrimSpace(req.RoomCode)

	if roomCode == "" {
		http.Error(w, "Room code is required", http.StatusBadRequest)
		return
	}

	participant, err := h.participantService.JoinRoom(
		r.Context(),
		roomCode,
		userID,
	)

	if err != nil {

		switch {
		case errors.Is(err, services.ErrInvalidRoomCode):
			http.Error(w, "Invalid room code", http.StatusNotFound)

		case errors.Is(err, services.ErrRoomClosed),
			errors.Is(err, services.ErrRoomFull):
			http.Error(w, "Room is full or locked", http.StatusConflict)

		default:
			http.Error(w, "Failed to join room", http.StatusInternalServerError)
		}

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":     "Joined room successfully",
		"participant": participant,
	})
}

func (h *RoomParticipantHandler) GetRoomParticipants(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		roomID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve participants", http.StatusInternalServerError)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"count":        len(participants),
		"participants": participants,
	})
}

func (h *RoomParticipantHandler) GetParticipantByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	participantID := strings.TrimSpace(chi.URLParam(r, "id"))
	if participantID == "" {
		http.Error(w, "Participant ID is required", http.StatusBadRequest)
		return
	}

	participant, err := h.participantService.GetParticipantByID(
		r.Context(),
		participantID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Participant not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve participant", http.StatusInternalServerError)
		return
	}

	// Only members of the participant's room can view this record.
	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		participant.RoomID,
	)
	if err != nil {
		http.Error(w, "Failed to verify room membership", http.StatusInternalServerError)
		return
	}

	isMember := false
	for _, roomParticipant := range participants {
		if roomParticipant.UserID == userID {
			isMember = true
			break
		}
	}

	if !isMember {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"participant": participant,
	})
}

type UpdateConnectionStatusRequest struct {
	Status string `json:"status"`
}

func (h *RoomParticipantHandler) UpdateConnectionStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	participantID := strings.TrimSpace(chi.URLParam(r, "id"))
	if participantID == "" {
		http.Error(w, "Participant ID is required", http.StatusBadRequest)
		return
	}

	participant, err := h.participantService.GetParticipantByID(
		r.Context(),
		participantID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve participant", http.StatusInternalServerError)
		return
	}
	if participant == nil {
		http.Error(w, "Participant not found", http.StatusNotFound)
		return
	}

	if participant.UserID != userID {
		http.Error(w, "You can only update your own connection status", http.StatusForbidden)
		return
	}

	var req UpdateConnectionStatusRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))

	if req.Status != "CONNECTED" && req.Status != "DISCONNECTED" {
		http.Error(w, "Status must be CONNECTED or DISCONNECTED", http.StatusBadRequest)
		return
	}

	if err := h.participantService.UpdateConnectionStatus(
		r.Context(),
		participantID,
		req.Status,
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Connection status updated",
		"status":  req.Status,
	})
}
