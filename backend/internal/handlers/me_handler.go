package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type MeHandler struct {
	userService *services.UserService
}

func NewMeHandler(userService *services.UserService) *MeHandler {
	return &MeHandler{
		userService: userService,
	}
}

func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	user, err := h.userService.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	response := map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"is_admin": user.IsAdmin,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
