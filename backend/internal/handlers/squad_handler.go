package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

type SquadHandler struct {
	squadService       *services.SquadService
	teamService        *services.TeamService
	participantService *services.RoomParticipantService
}

func NewSquadHandler(
	squadService *services.SquadService,
	teamService *services.TeamService,
	participantService *services.RoomParticipantService,
) *SquadHandler {
	return &SquadHandler{
		squadService:       squadService,
		teamService:        teamService,
		participantService: participantService,
	}
}

func (h *SquadHandler) GetSquadByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	squadID := chi.URLParam(r, "id")
	if squadID == "" {
		http.Error(w, "Squad ID is required", http.StatusBadRequest)
		return
	}

	squad, err := h.squadService.GetSquadByID(
		r.Context(),
		squadID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if strings.Contains(err.Error(), "required") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "Failed to retrieve squad", http.StatusInternalServerError)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		squad.RoomID,
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

	writeJSON(w, http.StatusOK, squad)
}

func (h *SquadHandler) GetSquadByTeam(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "teamID")
	if teamID == "" {
		http.Error(w, "Team ID is required", http.StatusBadRequest)
		return
	}

	team, err := h.teamService.GetTeamByID(
		r.Context(),
		teamID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to retrieve team", http.StatusInternalServerError)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		team.RoomID,
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

	squad, err := h.squadService.GetSquadByTeam(
		r.Context(),
		teamID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve team squad", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, squad)
}

func (h *SquadHandler) GetSquadsByRoom(
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

	squads, err := h.squadService.GetSquadsByRoom(
		r.Context(),
		roomID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve room squads", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, squads)
}
