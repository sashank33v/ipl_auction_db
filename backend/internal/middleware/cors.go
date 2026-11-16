package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

func CORS(frontendURL string) func(next http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins: []string{frontendURL},
		AllowedMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowedHeaders: []string{
			"Accept", "Authorization", "Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
