package main

import (
	"database/sql"
	"log"
	"net/http"

	_ "github.com/lib/pq"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/middleware"
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
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTPublicKey)
	registrationService := services.NewRegistrationService(userRepo)

	authHandler := handler.NewAuthHandler(authService)
	registrationHandler := handler.NewRegistrationHandler(registrationService)
	healthHandler := handler.NewHealthHandler(db)

	mux := http.NewServeMux()

	// Health endpoints (OpenShift compatible)
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /health/ready", healthHandler.Ready)
	mux.HandleFunc("GET /health/live", healthHandler.Live)

	// API endpoints
	mux.HandleFunc("POST /login", authHandler.Login)
	mux.Handle("POST /register",
		authMiddleware.RequireRole("ADMIN")(
			http.HandlerFunc(registrationHandler.RegisterParent),
		),
	)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
