package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
		log.Printf("Warning: Redis is not available yet: %v. App will continue and retry later.", err)
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
	healthHandler := handler.NewHealthHandler(db, redisClient)

	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health endpoints (OpenShift compatible)
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/health/ready", healthHandler.Ready)
	mux.HandleFunc("/health/live", healthHandler.Live)

	// API endpoints
	mux.HandleFunc("GET /login", authHandler.Login)
	mux.HandleFunc("GET /auth/google/callback", authHandler.LoginCallback)

	mux.Handle("POST /register",
		authMiddleware.RequireRole([]string{"ADMIN"}, http.HandlerFunc(registrationHandler.Register)),
	)

	mux.Handle("POST /logout",
		authMiddleware.RequireRole([]string{"ADMIN", "PARENT"}, http.HandlerFunc(authHandler.Logout)),
	)

	mux.Handle("POST /discharge",
		authMiddleware.RequireRole([]string{"ADMIN"}, http.HandlerFunc(authHandler.DischargeParent)),
	)

	// Wrap the mux with the metrics middleware
	loggedRouter := middleware.MetricsMiddleware(mux)

	log.Printf("Starting server on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, loggedRouter); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
