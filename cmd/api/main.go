package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/middleware"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/repository"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
)

func main() {

	// Check for "migrate" argument to run migrations
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrations()
		os.Exit(0)
	}

	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	var userRepo ports.UserRepository = repository.NewSQLRepository(db)

	authService := services.NewGoogleOAuthService(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURL,
		userRepo,
		cfg.JWTPrivateKey,
	)
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTPublicKey)
	registrationService := services.NewRegistrationService(userRepo)

	oauthHandler := handler.NewOAuthHandler(authService)
	registrationHandler := handler.NewRegistrationHandler(registrationService)
	healthHandler := handler.NewHealthHandler(db)

	mux := http.NewServeMux()

	// Health endpoints (OpenShift compatible)
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/health/ready", healthHandler.Ready)
	mux.HandleFunc("/health/live", healthHandler.Live)

	// API endpoints
	mux.HandleFunc("/login", oauthHandler.Login)
	mux.HandleFunc("/auth/google/callback", oauthHandler.Callback)

	mux.Handle("/register",
		authMiddleware.RequireRole("ADMIN", http.HandlerFunc(registrationHandler.Register)),
	)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

func runMigrations() {
	// 1. Get the DB connection string from the environment variable (passed via Secret)
	dbURL := os.Getenv("DB_CONNECTION_STRING")

	// 2. Create the migrator instance
	// Note: Migrations are embedded in the Docker image at /migrations
	m, err := migrate.New("file:///migrations", dbURL)
	if err != nil {
		log.Fatalf("Migration initialization failed: %v", err)
	}

	// 3. Apply all pending migrations
	log.Println("Applying database migrations...")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migrations applied successfully!")
}
