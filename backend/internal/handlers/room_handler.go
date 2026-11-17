package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type RoomHandler struct {
	roomService *services.RoomService
}

func NewRoomHandler(roomService *services.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
}

type CreateRoomRequest struct {
	Name               string `json:"name"`
	MaxParticipants    int    `json:"max_participants"`
	StartingPurse      string `json:"starting_purse"`
	MinSquadSize       int    `json:"min_squad_size"`
	MaxSquadSize       int    `json:"max_squad_size"`
	MaxOverseasPlayers int    `json:"max_overseas_players"`
	MinBidIncrement    string `json:"min_bid_increment"`
}

type UpdateRoomRequest struct {
	Name            string `json:"name"`
	MaxParticipants int    `json:"max_participants"`
}

func generateRoomCode() (string, error) {
	bytes := make([]byte, 6)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(bytes)), nil
}

func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req CreateRoomRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, "Room name is required", http.StatusBadRequest)
		return
	}

	roomCode, err := generateRoomCode()
	if err != nil {
		http.Error(w, "Failed to generate room code", http.StatusInternalServerError)
		return
	}

	room := &repository.Room{
		RoomCode:           roomCode,
		HostUserID:         userID,
		Name:               req.Name,
		Status:             "WAITING",
		MaxParticipants:    req.MaxParticipants,
		StartingPurse:      req.StartingPurse,
		MinSquadSize:       req.MinSquadSize,
		MaxSquadSize:       req.MaxSquadSize,
		MaxOverseasPlayers: req.MaxOverseasPlayers,
		MinBidIncrement:    req.MinBidIncrement,
	}

	if err := h.roomService.CreateRoom(r.Context(), room); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Room created successfully",
		"room":    room,
	})
}

func (h *RoomHandler) GetRoomByID(w http.ResponseWriter, r *http.Request) {
	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}

	room, err := h.roomService.GetRoomByID(r.Context(), roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (h *RoomHandler) GetRoomByCode(w http.ResponseWriter, r *http.Request) {
	roomCode := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "code")))
	if roomCode == "" {
		http.Error(w, "Room code is required", http.StatusBadRequest)
		return
	}

	room, err := h.roomService.GetRoomByCode(r.Context(), roomCode)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"room": room})
}

func (h *RoomHandler) GetAllRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.roomService.GetAllRooms(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve rooms", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(rooms),
		"rooms": rooms,
	})
}

func (h *RoomHandler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
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

	existingRoom, err := h.roomService.GetRoomByID(r.Context(), roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if existingRoom.HostUserID != userID {
		http.Error(w, "Only the room host can update this room", http.StatusForbidden)
		return
	}

	var req UpdateRoomRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	updatedRoom := &repository.Room{
		ID:              roomID,
		Name:            strings.TrimSpace(req.Name),
		MaxParticipants: req.MaxParticipants,
	}

	if err := h.roomService.UpdateRoom(r.Context(), updatedRoom); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Room updated successfully",
		"room":    updatedRoom,
	})
}

func (h *RoomHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
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

	room, err := h.roomService.GetRoomByID(r.Context(), roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	if room.HostUserID != userID {
		http.Error(w, "Only the room host can delete this room", http.StatusForbidden)
		return
	}

	if err := h.roomService.DeleteRoom(r.Context(), roomID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Room deleted successfully",
		"room_id": roomID,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
