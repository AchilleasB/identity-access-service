package config

import (
	"log"
	"time"

	"github.com/sony/gobreaker"
)

// NewCircuitBreaker creates a circuit breaker with standard settings.
// The name parameter uniquely identifies the circuit breaker instance.
func NewCircuitBreaker(name string) *gobreaker.CircuitBreaker {
	var timeout time.Duration

	// Use different timeouts for different dependencies
	// This aligns with health check timeouts (5s) to prevent race conditions
	switch {
	case name == "Redis-Auth":
		timeout = time.Second * 5 // Align with health check timeout
	case name == "PostgreSQL", name == "MongoDB", name == "Relay-PostgreSQL":
		timeout = time.Second * 10 // Database operations need slightly more time
	default:
		timeout = time.Second * 30 // RabbitMQ and other operations
	}

	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    time.Second * 10,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open circuit after 3 consecutive failures
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("[CRITICAL] Circuit Breaker %s: %s -> %s", name, from, to)
		},
	})
}
