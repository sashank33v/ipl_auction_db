package main

import (
	"log"
	"net/http"

	"github.com/sashank33v/ipl_auction_db/backend/internal/api"
	"github.com/sashank33v/ipl_auction_db/backend/internal/config"
	"github.com/sashank33v/ipl_auction_db/backend/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	db, err := database.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	router := api.NewRouter()

	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	log.Printf("Server starting on port http://localhost:%s", cfg.ServerPort)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
