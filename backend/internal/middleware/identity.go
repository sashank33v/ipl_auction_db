package middleware

import "context"

type contextKey string

const userIDContextKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}
