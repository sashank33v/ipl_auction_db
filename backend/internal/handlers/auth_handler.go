package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sashank33v/ipl_auction_db/backend/internal/repository"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "ipl_session"
	sessionDuration   = 30 * 24 * time.Hour
)

type AuthHandler struct {
	userService    *services.UserService
	sessionService *services.SessionService
}

func NewAuthHandler(
	userService *services.UserService,
	sessionService *services.SessionService,
) *AuthHandler {
	return &AuthHandler{
		userService:    userService,
		sessionService: sessionService,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Message string `json:"message"`
	User    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		IsAdmin  bool   `json:"is_admin"`
	} `json:"user"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "Username, email and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		http.Error(w, "Password must contain at least 8 characters", http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		return
	}

	user := &repository.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(passwordHash),
	}

	if err := h.userService.CreateUser(r.Context(), user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "Email or username already exists", http.StatusConflict)
			return
		}

		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	writeAuthResponse(w, http.StatusCreated, "User registered successfully", user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.userService.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	rawToken := hex.EncodeToString(tokenBytes)

	tokenHashBytes := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(tokenHashBytes[:])

	expiresAt := time.Now().Add(sessionDuration)

	session := &repository.Session{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := h.sessionService.CreateSession(r.Context(), session); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	writeAuthResponse(w, http.StatusOK, "Login successful", user)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeMessage(w, http.StatusOK, "Logged out successfully")
		return
	}

	tokenHashBytes := sha256.Sum256([]byte(cookie.Value))
	tokenHash := hex.EncodeToString(tokenHashBytes[:])

	session, err := h.sessionService.GetSessionByTokenHash(
		r.Context(),
		tokenHash,
	)
	if err != nil {
		http.Error(w, "Failed to retrieve session", http.StatusInternalServerError)
		return
	}

	if err := h.sessionService.RevokeSession(
		r.Context(),
		session.ID,
	); err != nil {
		http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
		return
	}

	writeMessage(w, http.StatusOK, "Logged out successfully")
}

func writeAuthResponse(
	w http.ResponseWriter,
	status int,
	message string,
	user *repository.User,
) {
	var response AuthResponse

	response.Message = message
	response.User.ID = user.ID
	response.User.Username = user.Username
	response.User.Email = user.Email
	response.User.IsAdmin = user.IsAdmin

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

func writeMessage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": message,
	})
}
