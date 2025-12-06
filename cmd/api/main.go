package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/lib/pq"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/repository"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
)

func main() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	var userRepo ports.UserRepository = repository.NewSQLRepository(db)

	authService := services.NewAuthService(userRepo, cfg.JWTPrivateKey)
	registrationService := services.NewRegistrationService(userRepo)

	authHandler := handler.NewAuthHandler(authService)
	registrationHandler := handler.NewRegistrationHandler(registrationService)
	healthHandler := handler.NewHealthHandler(db)

	// Health endpoints (OpenShift compatible)
	http.HandleFunc("/health", healthHandler.Health)      // Detailed health
	http.HandleFunc("/health/ready", healthHandler.Ready) // Readiness probe
	http.HandleFunc("/health/live", healthHandler.Live)   // Liveness probe

	// API endpoints
	http.HandleFunc("/login", authHandler.Login)
	http.HandleFunc("/register", registrationHandler.RegisterParent)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
