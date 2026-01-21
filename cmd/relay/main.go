package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/messaging"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/outbox"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
)

func main() {
	log.Println("Starting outbox relay service...")

	cfg := config.LoadRelayConfig()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Printf("relay: ERROR - failed to open database: %v", err)
	} else {
		defer db.Close()
		log.Println("relay: database connection initialized - circuit breaker will validate on first operation")
	}

	message_broker, err := messaging.NewRabbitMQBroker(cfg.RabbitMQURL, cfg.BabyQueueName)
	if err != nil {
		log.Printf("relay: WARNING - failed to create baby publisher: %v", err)
	} else {
		defer message_broker.Close()
		log.Println("relay: connected to RabbitMQ")
	}

	relay_worker := outbox.NewRelay(db, cfg.DatabaseURL, message_broker)

	// Start health check HTTP server
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := "UP"
		httpStatus := http.StatusOK

		if !relay_worker.IsHealthy() {
			status = "DOWN"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    status,
			"component": "outbox-relay",
		})
	})

	healthServer := &http.Server{
		Addr:    ":8090",
		Handler: healthMux,
	}

	go func() {
		log.Println("relay: starting health check server on :8090")
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("relay: health server error: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to capture fatal errors from relay worker
	errChan := make(chan error, 1)

	// Start relay worker in background goroutine
	go func() {
		log.Println("relay: starting event processing worker...")
		if err := relay_worker.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("relay: worker error: %v", err)
			errChan <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or fatal error
	select {
	case sig := <-sigChan:
		log.Printf("relay: received signal %v, initiating shutdown...", sig)
		cancel()

	case err := <-errChan:
		log.Printf("relay: fatal error, shutting down: %v", err)
		cancel()
	}

	// Shutdown health server gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("relay: error shutting down health server: %v", err)
	}

	log.Println("relay: shutdown complete")
}
