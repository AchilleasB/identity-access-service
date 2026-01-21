// Package relay_integration_tests contains integration tests for the outbox relay.
// These tests verify the relay works correctly with real PostgreSQL and RabbitMQ.
//
// RELAY INTEGRATION TEST ARCHITECTURE:
// ═══════════════════════════════════════════════════════════════════════════════
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                    INTEGRATION TEST ENVIRONMENT                          │
//	│                                                                          │
//	│    ┌────────────┐      ┌────────────┐      ┌────────────┐               │
//	│    │ Test Case  │─────▶│   Relay    │─────▶│  RabbitMQ  │               │
//	│    │            │      │            │      │  (real)    │               │
//	│    └────────────┘      └────────────┘      └────────────┘               │
//	│          │                   │                    │                      │
//	│          ▼                   ▼                    ▼                      │
//	│    ┌────────────┐      ┌────────────┐      ┌────────────┐               │
//	│    │ PostgreSQL │◀─────│  LISTEN/   │      │  Verify    │               │
//	│    │   (real)   │      │  NOTIFY    │      │  Messages  │               │
//	│    └────────────┘      └────────────┘      └────────────┘               │
//	└─────────────────────────────────────────────────────────────────────────┘
//
// RUNNING THESE TESTS:
// 1. Start test infrastructure: docker-compose -f docker-compose.test.yaml up
// 2. Set environment variables:
//   - TEST_DB_CONNECTION_STRING
//   - TEST_RABBITMQ_URL
//
// 3. Run: go test -tags=integration ./test/relay-integration-tests/...
package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/messaging"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/outbox"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
)

var (
	testDB       *sql.DB
	testDBURL    string
	testRabbitMQ *messaging.RabbitMQBroker
)

// TestMain sets up the integration test environment.
func TestMain(m *testing.M) {
	// Check for integration test configuration
	testDBURL = os.Getenv("TEST_DB_CONNECTION_STRING")
	if testDBURL == "" {
		fmt.Println("Skipping relay integration tests: TEST_DB_CONNECTION_STRING not set")
		fmt.Println("Run with docker-compose and environment variables")
		os.Exit(0)
	}

	rabbitURL := os.Getenv("TEST_RABBITMQ_URL")
	if rabbitURL == "" {
		fmt.Println("Skipping relay integration tests: TEST_RABBITMQ_URL not set")
		os.Exit(0)
	}

	// Connect to test database
	var err error
	testDB, err = sql.Open("postgres", testDBURL)
	if err != nil {
		fmt.Printf("Failed to connect to test database: %v\n", err)
		os.Exit(1)
	}
	defer testDB.Close()

	// Verify database connection
	if err := testDB.Ping(); err != nil {
		fmt.Printf("Failed to ping test database: %v\n", err)
		os.Exit(1)
	}

	// Connect to RabbitMQ
	testRabbitMQ, err = messaging.NewRabbitMQBroker(rabbitURL, "test_babies")
	if err != nil {
		fmt.Printf("Failed to connect to RabbitMQ: %v\n", err)
		os.Exit(1)
	}
	defer testRabbitMQ.Close()

	// Setup test schema
	if err := setupRelayTestSchema(testDB); err != nil {
		fmt.Printf("Failed to setup test schema: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupRelayTestData(testDB)

	os.Exit(code)
}

// setupRelayTestSchema creates the necessary tables and triggers for relay testing.
func setupRelayTestSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS outbox_events (
			id VARCHAR(36) PRIMARY KEY,
			aggregate_type VARCHAR(50) NOT NULL,
			aggregate_id VARCHAR(36) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			processed_at TIMESTAMP
		);

		-- Create NOTIFY function
		CREATE OR REPLACE FUNCTION notify_outbox_insert()
		RETURNS TRIGGER AS $$
		BEGIN
			PERFORM pg_notify('outbox_channel', NEW.id::text);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		-- Create trigger (drop if exists first)
		DROP TRIGGER IF EXISTS outbox_notify_trigger ON outbox_events;
		CREATE TRIGGER outbox_notify_trigger
		AFTER INSERT ON outbox_events
		FOR EACH ROW
		EXECUTE FUNCTION notify_outbox_insert();
	`
	_, err := db.Exec(schema)
	return err
}

// cleanupRelayTestData removes all test data.
func cleanupRelayTestData(db *sql.DB) {
	_, _ = db.Exec("DELETE FROM outbox_events")
}

// TestIntegration_RelayProcessesEvent tests end-to-end event processing.
func TestIntegration_RelayProcessesEvent(t *testing.T) {
	if testDB == nil || testRabbitMQ == nil {
		t.Skip("Integration tests require database and RabbitMQ")
	}

	cleanupRelayTestData(testDB)

	// Set environment for event type matching
	os.Setenv("BABY_QUEUE_NAME", "test_babies")
	defer os.Unsetenv("BABY_QUEUE_NAME")

	// Create relay
	relay := outbox.NewRelay(testDB, testDBURL, testRabbitMQ)

	// Start relay in background
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		_ = relay.Start(ctx)
	}()

	// Give relay time to start listening
	time.Sleep(100 * time.Millisecond)

	// Insert an outbox event
	event := ports.CreateBabyEvent{
		UserID:     "test-parent-123",
		LastName:   "IntegrationTest",
		RoomNumber: "IT-101",
	}
	payload, _ := json.Marshal(event)

	eventID := uuid.New().String()
	_, err := testDB.Exec(`
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, eventID, "parent", "test-parent-123", "test_babies", payload, time.Now())

	if err != nil {
		t.Fatalf("failed to insert outbox event: %v", err)
	}

	// Wait for relay to process
	time.Sleep(500 * time.Millisecond)

	// Verify event was marked as processed
	var processedAt sql.NullTime
	err = testDB.QueryRow("SELECT processed_at FROM outbox_events WHERE id = $1", eventID).Scan(&processedAt)
	if err != nil {
		t.Fatalf("failed to query event: %v", err)
	}

	if !processedAt.Valid {
		t.Error("event should be marked as processed")
	}
}

// TestIntegration_RelayHealthEndpoints tests relay health HTTP endpoints.
func TestIntegration_RelayHealthEndpoints(t *testing.T) {
	if testDB == nil || testRabbitMQ == nil {
		t.Skip("Integration tests require database and RabbitMQ")
	}

	relay := outbox.NewRelay(testDB, testDBURL, testRabbitMQ)

	t.Run("IsHealthy", func(t *testing.T) {
		// New relay should be healthy
		if !relay.IsHealthy() {
			t.Error("new relay should be healthy")
		}
	})

	// Note: IsReady depends on lastProcessed time and circuit breaker state
	// Full testing requires running the relay for some time
}

// TestIntegration_RelayProcessesUnprocessedOnStartup tests catch-up processing.
func TestIntegration_RelayProcessesUnprocessedOnStartup(t *testing.T) {
	if testDB == nil || testRabbitMQ == nil {
		t.Skip("Integration tests require database and RabbitMQ")
	}

	cleanupRelayTestData(testDB)

	os.Setenv("BABY_QUEUE_NAME", "test_babies")
	defer os.Unsetenv("BABY_QUEUE_NAME")

	// Insert events BEFORE starting relay (simulate backlog)
	for i := 1; i <= 3; i++ {
		event := ports.CreateBabyEvent{
			UserID:     fmt.Sprintf("parent-%d", i),
			LastName:   "BacklogTest",
			RoomNumber: fmt.Sprintf("BL-%d", i),
		}
		payload, _ := json.Marshal(event)

		_, err := testDB.Exec(`
			INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, uuid.New().String(), "parent", fmt.Sprintf("parent-%d", i), "test_babies", payload, time.Now())

		if err != nil {
			t.Fatalf("failed to insert backlog event %d: %v", i, err)
		}
	}

	// Create and start relay
	relay := outbox.NewRelay(testDB, testDBURL, testRabbitMQ)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = relay.Start(ctx)
	}()

	// Wait for catch-up processing
	time.Sleep(2 * time.Second)

	// Verify all events were processed
	var unprocessedCount int
	err := testDB.QueryRow("SELECT COUNT(*) FROM outbox_events WHERE processed_at IS NULL").Scan(&unprocessedCount)
	if err != nil {
		t.Fatalf("failed to count unprocessed events: %v", err)
	}

	if unprocessedCount != 0 {
		t.Errorf("expected 0 unprocessed events, got %d", unprocessedCount)
	}
}

// TestIntegration_RelayHandlesInvalidPayload tests handling of malformed events.
func TestIntegration_RelayHandlesInvalidPayload(t *testing.T) {
	if testDB == nil || testRabbitMQ == nil {
		t.Skip("Integration tests require database and RabbitMQ")
	}

	cleanupRelayTestData(testDB)

	os.Setenv("BABY_QUEUE_NAME", "test_babies")
	defer os.Unsetenv("BABY_QUEUE_NAME")

	// Create invalid JSON payload
	invalidPayload := []byte(`{}`)

	// Insert event with invalid JSON payload
	invalidEventID := uuid.New().String()
	_, err := testDB.Exec(`
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, invalidEventID, "parent", "parent-123", "test_babies", invalidPayload, time.Now())

	if err != nil {
		t.Fatalf("failed to insert invalid event: %v", err)
	}

	relay := outbox.NewRelay(testDB, testDBURL, testRabbitMQ)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		_ = relay.Start(ctx)
	}()

	time.Sleep(1 * time.Second)

	// Invalid events should still be marked as processed (to avoid infinite retries)
	var processedAt sql.NullTime
	err = testDB.QueryRow("SELECT processed_at FROM outbox_events WHERE id = $1", invalidEventID).Scan(&processedAt)
	if err != nil {
		t.Fatalf("failed to query event: %v", err)
	}

	if !processedAt.Valid {
		t.Error("invalid event should be marked as processed to avoid infinite retries")
	}
}

// TestIntegration_RelayRespectsCircuitBreaker tests circuit breaker behavior.
func TestIntegration_RelayRespectsCircuitBreaker(t *testing.T) {
	// This test would require simulating database failures
	// In a real integration test environment, you could:
	// 1. Use toxiproxy to inject network failures
	// 2. Use docker pause to simulate database unavailability
	// 3. Use a test double that can simulate failures

	t.Skip("Circuit breaker testing requires failure injection infrastructure")
}
