package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/middleware"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/repository"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
)

func main() {

	cfg := config.Load()
	ctx := context.Background()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewSQLRepository(db)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Println("Authenticated with Redis successfully")

	authService := services.NewAuthService(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURL,
		userRepo,
		cfg.JWTPrivateKey,
		redisClient,
	)

	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTPublicKey, redisClient)
	registrationService := services.NewRegistrationService(userRepo)

	authHandler := handler.NewAuthHandler(authService)
	registrationHandler := handler.NewRegistrationHandler(registrationService)
	healthHandler := handler.NewHealthHandler(db)

	mux := http.NewServeMux()

	// Health endpoints (OpenShift compatible)
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/health/ready", healthHandler.Ready)
	mux.HandleFunc("/health/live", healthHandler.Live)

	// API endpoints
	mux.HandleFunc("/login", authHandler.Login)
	mux.HandleFunc("/auth/google/callback", authHandler.LoginCallback)

	mux.Handle("/register",
		authMiddleware.RequireRole([]string{"ADMIN"}, http.HandlerFunc(registrationHandler.Register)),
	)

	mux.Handle("/logout",
		authMiddleware.RequireRole([]string{"ADMIN", "PARENT"}, http.HandlerFunc(authHandler.Logout)),
	)

	mux.Handle("/discharge",
		authMiddleware.RequireRole([]string{"ADMIN"}, http.HandlerFunc(authHandler.DischargeParent)),
	)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
