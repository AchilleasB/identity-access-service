package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	redis "github.com/redis/go-redis/v9"

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

	// Apply middleware chain: CORS -> Metrics
	corsRouter := middleware.CORSMiddleware(cfg.CORSAllowedOrigins)(mux)
	loggedRouter := middleware.MetricsMiddleware(corsRouter)

	// Create HTTP server with timeouts
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggedRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on :%s", cfg.Port)
		log.Printf("CORS allowed origins: %v", cfg.CORSAllowedOrigins)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start server: %s\n", err)
		}
	}()

	// Setup signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	sig := <-quit
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		log.Printf("Error closing Redis: %v", err)
	}

	log.Println("Server gracefully stopped")
}
