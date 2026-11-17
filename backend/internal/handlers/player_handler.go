package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type PlayerHandler struct {
	playerService *services.PlayerService
}

func NewPlayerHandler(playerService *services.PlayerService) *PlayerHandler {
	return &PlayerHandler{
		playerService: playerService,
	}
}

type CreatePlayerRequest struct {
	FullName        string `json:"full_name"`
	Country         string `json:"country"`
	Role            string `json:"role"`
	IsOverseas      bool   `json:"is_overseas"`
	BasePrice       string `json:"base_price"`
	ProfileImageURL string `json:"profile_image_url"`
}

func (h *PlayerHandler) CreatePlayer(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req CreatePlayerRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	player := &repository.Player{
		FullName:        strings.TrimSpace(req.FullName),
		Country:         strings.TrimSpace(req.Country),
		Role:            strings.ToUpper(strings.TrimSpace(req.Role)),
		IsOverseas:      req.IsOverseas,
		BasePrice:       strings.TrimSpace(req.BasePrice),
		ProfileImageURL: strings.TrimSpace(req.ProfileImageURL),
	}

	if err := h.playerService.CreatePlayer(r.Context(), player); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Player created successfully",
		"player":  player,
	})
}

func (h *PlayerHandler) GetPlayerByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	playerID := strings.TrimSpace(chi.URLParam(r, "id"))

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	player, err := h.playerService.GetPlayerByID(r.Context(), playerID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Player not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve player", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"player": player,
	})
}

func (h *PlayerHandler) GetAllPlayers(
	w http.ResponseWriter,
	r *http.Request,
) {
	players, err := h.playerService.GetAllPlayers(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve players", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(players),
		"players": players,
	})
}

type UpdatePlayerRequest struct {
	FullName        string `json:"full_name"`
	Country         string `json:"country"`
	Role            string `json:"role"`
	IsOverseas      bool   `json:"is_overseas"`
	BasePrice       string `json:"base_price"`
	ProfileImageURL string `json:"profile_image_url"`
}

func (h *PlayerHandler) UpdatePlayer(
	w http.ResponseWriter,
	r *http.Request,
) {
	playerID := strings.TrimSpace(chi.URLParam(r, "id"))
	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	var req UpdatePlayerRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	player := &repository.Player{
		ID:              playerID,
		FullName:        strings.TrimSpace(req.FullName),
		Country:         strings.TrimSpace(req.Country),
		Role:            strings.ToUpper(strings.TrimSpace(req.Role)),
		IsOverseas:      req.IsOverseas,
		BasePrice:       strings.TrimSpace(req.BasePrice),
		ProfileImageURL: strings.TrimSpace(req.ProfileImageURL),
	}

	if err := h.playerService.UpdatePlayer(r.Context(), player); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Player not found", http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Player updated successfully",
		"player":  player,
	})
}

func (h *PlayerHandler) DeletePlayer(
	w http.ResponseWriter,
	r *http.Request,
) {
	playerID := strings.TrimSpace(chi.URLParam(r, "id"))

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	if err := h.playerService.DeletePlayer(r.Context(), playerID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Player not found", http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   "Player deactivated successfully",
		"player_id": playerID,
	})
}
