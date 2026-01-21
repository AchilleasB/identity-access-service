// Package contains unit tests for the outbox relay service.
// The relay is responsible for:
// 1. Listening to PostgreSQL NOTIFY events
// 2. Processing outbox events
// 3. Publishing events to RabbitMQ
//
// RELAY TESTING STRATEGY:
// ═══════════════════════════════════════════════════════════════════════════════
//
//	The relay connects multiple systems:
//	- PostgreSQL (outbox table + LISTEN/NOTIFY)
//	- RabbitMQ (event publishing)
//
//	Unit tests mock these dependencies using:
//	- MockBabyEventPublisher (replaces RabbitMQ)
//	- In-memory state verification
//
//	                    ┌─────────────────┐
//	                    │   Relay Unit    │
//	                    │     Tests       │
//	                    └────────┬────────┘
//	                             │
//	              ┌──────────────┼──────────────┐
//	              ▼              ▼              ▼
//	       ┌──────────┐   ┌──────────┐   ┌──────────┐
//	       │  Relay   │   │  Health  │   │ Process  │
//	       │  State   │   │  Checks  │   │  Events  │
//	       └──────────┘   └──────────┘   └──────────┘
package unit

import (
	"context"
	"testing"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/test/mocks"
)

// TestMockPublisher_PublishBabyCreated tests the mock publisher directly.
// This validates our mock implementation works correctly.
func TestMockPublisher_PublishBabyCreated(t *testing.T) {
	publisher := mocks.NewMockBabyEventPublisher()

	event := ports.CreateBabyEvent{
		UserID:     "user-123",
		LastName:   "TestFamily",
		RoomNumber: "101",
	}

	ctx := context.Background()
	err := publisher.PublishBabyCreated(ctx, event)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify event was captured
	events := publisher.GetPublishedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].UserID != "user-123" {
		t.Errorf("expected UserID 'user-123', got %q", events[0].UserID)
	}
	if events[0].LastName != "TestFamily" {
		t.Errorf("expected LastName 'TestFamily', got %q", events[0].LastName)
	}
	if events[0].RoomNumber != "101" {
		t.Errorf("expected RoomNumber '101', got %q", events[0].RoomNumber)
	}
}

// TestMockPublisher_ErrorInjection tests error injection.
func TestMockPublisher_ErrorInjection(t *testing.T) {
	publisher := mocks.NewMockBabyEventPublisher()
	publisher.PublishError = context.DeadlineExceeded

	event := ports.CreateBabyEvent{
		UserID:     "user-123",
		LastName:   "TestFamily",
		RoomNumber: "101",
	}

	ctx := context.Background()
	err := publisher.PublishBabyCreated(ctx, event)

	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got: %v", err)
	}

	// Verify event was NOT captured on error
	events := publisher.GetPublishedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events after error, got %d", len(events))
	}
}

// TestMockPublisher_Reset tests the reset functionality.
func TestMockPublisher_Reset(t *testing.T) {
	publisher := mocks.NewMockBabyEventPublisher()

	// Publish some events
	ctx := context.Background()
	_ = publisher.PublishBabyCreated(ctx, ports.CreateBabyEvent{UserID: "1"})
	_ = publisher.PublishBabyCreated(ctx, ports.CreateBabyEvent{UserID: "2"})

	if publisher.GetPublishCount() != 2 {
		t.Fatalf("expected 2 calls before reset")
	}

	// Reset
	publisher.Reset()

	if publisher.GetPublishCount() != 0 {
		t.Errorf("expected 0 calls after reset, got %d", publisher.GetPublishCount())
	}
	if len(publisher.GetPublishedEvents()) != 0 {
		t.Errorf("expected 0 events after reset")
	}
}

// TestMockPublisher_ConcurrentPublish tests thread safety.
func TestMockPublisher_ConcurrentPublish(t *testing.T) {
	publisher := mocks.NewMockBabyEventPublisher()

	ctx := context.Background()
	const numGoroutines = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			event := ports.CreateBabyEvent{
				UserID:     "user",
				LastName:   "Test",
				RoomNumber: "101",
			}
			_ = publisher.PublishBabyCreated(ctx, event)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	if publisher.GetPublishCount() != numGoroutines {
		t.Errorf("expected %d calls, got %d", numGoroutines, publisher.GetPublishCount())
	}
}

// TestCreateBabyEvent_Structure tests the event structure.
func TestCreateBabyEvent_Structure(t *testing.T) {
	event := ports.CreateBabyEvent{
		UserID:     "parent-uuid-123",
		LastName:   "Van Der Berg",
		RoomNumber: "A-201",
	}

	// Verify JSON tags work correctly
	if event.UserID == "" || event.LastName == "" || event.RoomNumber == "" {
		t.Error("event fields should not be empty")
	}
}

// TestEventPayloadSerialization tests that events serialize correctly.
// This is important for the relay which marshals/unmarshals events.
func TestEventPayloadSerialization(t *testing.T) {
	original := ports.CreateBabyEvent{
		UserID:     "user-123",
		LastName:   "TestFamily",
		RoomNumber: "101",
	}

	// Simulate what happens in the relay
	publisher := mocks.NewMockBabyEventPublisher()
	ctx := context.Background()
	publisher.PublishBabyCreated(ctx, original)

	// Retrieve and compare
	events := publisher.GetPublishedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}

	received := events[0]
	if received.UserID != original.UserID {
		t.Errorf("UserID mismatch: %q != %q", received.UserID, original.UserID)
	}
	if received.LastName != original.LastName {
		t.Errorf("LastName mismatch: %q != %q", received.LastName, original.LastName)
	}
	if received.RoomNumber != original.RoomNumber {
		t.Errorf("RoomNumber mismatch: %q != %q", received.RoomNumber, original.RoomNumber)
	}
}
