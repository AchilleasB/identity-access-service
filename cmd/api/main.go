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
	// Load configuration from environment
	cfg := config.Load()

	// Initialize the database connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	// Initialize the repository
	var userRepo ports.UserRepository = repository.NewSQLRepository(db)

	// Initialize the services
	authService, err := services.NewAuthService(userRepo, cfg.JWTPrivateKey)
	if err != nil {
		log.Fatalf("failed to initialize auth service: %v", err)
	}

	registrationService := services.NewRegistrationService(userRepo)

	// Initialize the handlers
	authHandler := handler.NewAuthHandler(authService)
	registrationHandler := handler.NewRegistrationHandler(registrationService)

	// Set up HTTP routes
	http.HandleFunc("/login", authHandler.Login)
	http.HandleFunc("/register", registrationHandler.RegisterParent)

	// Start the HTTP server
	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
