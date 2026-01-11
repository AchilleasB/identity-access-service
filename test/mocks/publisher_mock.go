package mocks

import (
	"context"
	"sync"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
)

// MockBabyEventPublisher implements ports.BabyEventPublisher for testing.
// This mock allows us to test the outbox relay without a real RabbitMQ connection.
//
// In the hexagonal architecture:
// - ports.BabyEventPublisher is the port (interface)
// - RabbitMQBroker is the real adapter (production)
// - MockBabyEventPublisher is the test adapter (testing)
type MockBabyEventPublisher struct {
	mu sync.RWMutex

	// Track published events for verification
	PublishedEvents []ports.CreateBabyEvent

	// Error injection for testing error scenarios
	PublishError error

	// Track number of calls
	PublishCallCount int
}

// Ensure MockBabyEventPublisher implements ports.BabyEventPublisher at compile time.
var _ ports.BabyEventPublisher = (*MockBabyEventPublisher)(nil)

// NewMockBabyEventPublisher creates a new mock publisher.
func NewMockBabyEventPublisher() *MockBabyEventPublisher {
	return &MockBabyEventPublisher{
		PublishedEvents: make([]ports.CreateBabyEvent, 0),
	}
}

// PublishBabyCreated captures published events for verification.
// This implements ports.BabyEventPublisher.PublishBabyCreated
func (m *MockBabyEventPublisher) PublishBabyCreated(ctx context.Context, evt ports.CreateBabyEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishCallCount++

	if m.PublishError != nil {
		return m.PublishError
	}

	m.PublishedEvents = append(m.PublishedEvents, evt)
	return nil
}

// GetPublishedEvents returns all events that were published.
func (m *MockBabyEventPublisher) GetPublishedEvents() []ports.CreateBabyEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent race conditions
	events := make([]ports.CreateBabyEvent, len(m.PublishedEvents))
	copy(events, m.PublishedEvents)
	return events
}

// GetPublishCount returns the number of times PublishBabyCreated was called.
func (m *MockBabyEventPublisher) GetPublishCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.PublishCallCount
}

// Reset clears all tracking data.
func (m *MockBabyEventPublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedEvents = make([]ports.CreateBabyEvent, 0)
	m.PublishError = nil
	m.PublishCallCount = 0
}
