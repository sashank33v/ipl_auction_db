package api

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/sashank33v/ipl_auction_db/backend/internal/handlers"
	appmiddleware "github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
	"github.com/sashank33v/ipl_auction_db/backend/internal/services"
)

func NewRouter(frontendURL string, authHandler *handlers.AuthHandler, meHandler *handlers.MeHandler, sessionService *services.SessionService) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(appmiddleware.CORS(frontendURL))
	r.Use(appmiddleware.Logger)
	r.Use(appmiddleware.Recovery)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)

		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.Auth(sessionService))
			r.Get("/me", meHandler.GetMe)
		})
	})
	return r
}
