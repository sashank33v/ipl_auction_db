package middleware

import (
	"context"
	"net/http"
)

// AdminOnly must run after Auth. Authorization is checked on every request.
func AdminOnly(isAdmin func(context.Context, string) (bool, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok || userID == "" {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			allowed, err := isAdmin(r.Context(), userID)
			if err != nil {
				http.Error(w, "Failed to verify administrator permission", http.StatusInternalServerError)
				return
			}
			if !allowed {
				http.Error(w, "Administrator permission required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
