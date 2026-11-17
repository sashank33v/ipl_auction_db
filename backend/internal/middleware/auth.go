package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

const sessionCookieName = "ipl_session"

type contextKey string

const userIDContextKey contextKey = "user_id"

func Auth(sessionService *services.SessionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			tokenHashBytes := sha256.Sum256([]byte(cookie.Value))
			tokenHash := hex.EncodeToString(tokenHashBytes[:])

			session, err := sessionService.GetSessionByTokenHash(
				r.Context(),
				tokenHash,
			)
			if err != nil || session == nil {
				http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
				return
			}

			if err := sessionService.UpdateLastUsed(
				r.Context(),
				session.ID,
			); err != nil {
				http.Error(w, "Failed to validate session", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(
				r.Context(),
				userIDContextKey,
				session.UserID,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}
