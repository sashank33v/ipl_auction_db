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

type TeamHandler struct {
	teamService        *services.TeamService
	participantService *services.RoomParticipantService
}

func NewTeamHandler(
	teamService *services.TeamService,
	participantService *services.RoomParticipantService,
) *TeamHandler {
	return &TeamHandler{
		teamService:        teamService,
		participantService: participantService,
	}
}

type CreateTeamRequest struct {
	RoomID        string `json:"room_id"`
	FranchiseCode string `json:"franchise_code"`
}

func (h *TeamHandler) CreateTeam(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req CreateTeamRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.RoomID = strings.TrimSpace(req.RoomID)
	req.FranchiseCode = strings.ToUpper(
		strings.TrimSpace(req.FranchiseCode),
	)

	if req.RoomID == "" || req.FranchiseCode == "" {
		http.Error(w, "Room ID and franchise code are required", http.StatusBadRequest)
		return
	}

	participants, err := h.participantService.GetRoomParticipants(
		r.Context(),
		req.RoomID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve room participants", http.StatusInternalServerError)
		return
	}

	var participantID string

	for _, participant := range participants {
		if participant.UserID == userID {
			participantID = participant.ID
			break
		}
	}

	if participantID == "" {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	team := &repository.Team{
		RoomID:        req.RoomID,
		ParticipantID: participantID,
		FranchiseCode: req.FranchiseCode,
	}

	if err := h.teamService.CreateTeam(r.Context(), team); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"message": "Team created successfully",
		"team":    team,
	})
}

func (h *TeamHandler) GetTeamByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	teamID := strings.TrimSpace(chi.URLParam(r, "id"))
	if teamID == "" {
		http.Error(w, "Team ID is required", http.StatusBadRequest)
		return
	}

	team, err := h.teamService.GetTeamByID(r.Context(), teamID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Team not found", http.StatusNotFound)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"team": team,
	})
}
func (h *TeamHandler) GetTeamByParticipant(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "roomID"))
	participantID := strings.TrimSpace(chi.URLParam(r, "participantID"))

	if roomID == "" || participantID == "" {
		http.Error(w, "Room ID and participant ID are required", http.StatusBadRequest)
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
	isTargetParticipant := false

	for _, participant := range participants {
		if participant.UserID == userID {
			isMember = true
		}

		if participant.ID == participantID {
			isTargetParticipant = true
		}
	}

	if !isMember {
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	if !isTargetParticipant {
		http.Error(w, "Participant not found in this room", http.StatusNotFound)
		return
	}

	team, err := h.teamService.GetTeamByParticipant(
		r.Context(),
		roomID,
		participantID,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve team", http.StatusInternalServerError)
		return
	}

	if team == nil {
		http.Error(w, "Participant has not selected a team", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"team": team,
	})
}

func (h *TeamHandler) GetTeamsByRoom(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "roomID"))
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

	teams, err := h.teamService.GetTeamsByRoom(r.Context(), roomID)
	if err != nil {
		http.Error(w, "Failed to retrieve room teams", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(teams),
		"teams": teams,
	})
}
