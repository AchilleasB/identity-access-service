package config

import (
	"log"
	"time"

	"github.com/sony/gobreaker"
)

// NewCircuitBreaker creates a circuit breaker with standard settings.
// The name parameter uniquely identifies the circuit breaker instance.
func NewCircuitBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    time.Second * 10,
		Timeout:     time.Second * 30,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open circuit after 3 consecutive failures
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("[CRITICAL] Circuit Breaker %s: %s -> %s", name, from, to)
		},
	})
}
