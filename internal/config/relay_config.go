package config

import "os"

// RelayConfig holds configuration for the outbox relay service.
// This is a minimal config that only includes what the relay needs.
type RelayConfig struct {
	DatabaseURL   string
	RabbitMQURL   string
	BabyQueueName string
}

func LoadRelayConfig() *RelayConfig {
	dbURL := os.Getenv("DB_CONNECTION_STRING")
	if dbURL == "" {
		panic("DB_CONNECTION_STRING environment variable is required")
	}

	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		panic("RABBITMQ_URL environment variable is required")
	}

	babyQueueName := os.Getenv("BABY_QUEUE_NAME")
	if babyQueueName == "" {
		babyQueueName = "babies"
	}

	return &RelayConfig{
		DatabaseURL:   dbURL,
		RabbitMQURL:   rabbitURL,
		BabyQueueName: babyQueueName,
	}
}
