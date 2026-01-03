package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

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
		log.Fatalf("relay: failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("relay: failed to ping database: %v", err)
	}
	log.Println("relay: connected to PostgreSQL")

	message_broker, err := messaging.NewRabbitMQBroker(cfg.RabbitMQURL, cfg.BabyQueueName)
	if err != nil {
		log.Fatalf("relay: failed to create baby publisher: %v", err)
	}
	defer message_broker.Close()
	log.Println("relay: connected to RabbitMQ")

	relay_worker := outbox.NewRelay(db, cfg.DatabaseURL, message_broker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("relay: received signal %v, initiating shutdown...", sig)
		cancel()
	}()

	if err := relay_worker.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("relay: error: %v", err)
	}

	log.Println("relay: shutdown complete")
}
