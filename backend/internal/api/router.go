package api

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"
	appmiddleware "github.com/sashank33v/ipl_auction_db/backend/internal/middleware"
)

func NewRouter(frontendURL string) http.Handler {
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

	return r
}
