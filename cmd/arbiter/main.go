package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faizanhussain/arbiter/pkg/api"
	"github.com/faizanhussain/arbiter/pkg/auth"
	"github.com/faizanhussain/arbiter/pkg/store"
)

func main() {
	// Database path from env or default
	dbPath := os.Getenv("ARBITER_DB_PATH")
	if dbPath == "" {
		dbPath = "arbiter.db"
	}

	// Port from env or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// JWT secret from env or auto-generated
	jwtSecret := os.Getenv("ARBITER_JWT_SECRET")

	// Open database
	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer s.Close()

	// Seed example rules on first run
	if err := s.Seed(context.Background()); err != nil {
		log.Printf("Warning: seed failed: %v", err)
	}

	// Seed default admin user if no users exist
	if err := s.SeedAdmin(context.Background()); err != nil {
		log.Printf("Warning: admin seed failed: %v", err)
	}

	// Auth config
	authCfg := auth.NewConfig(jwtSecret)

	// Create router with embedded web assets
	webFS := getWebFS()
	router := api.NewRouter(s, authCfg, webFS)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("Arbiter running on http://localhost:%s", port)
	if webFS != nil {
		log.Printf("Dashboard: http://localhost:%s/", port)
	}
	log.Printf("Default login: admin / admin (change immediately)")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
