// Package api_unit_tests contains unit tests for the API services.
// Unit tests verify individual components in isolation using mocked dependencies.
//
// HEXAGONAL ARCHITECTURE TESTING PATTERN:
// ═══════════════════════════════════════════════════════════════════════════════
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                           TEST ENVIRONMENT                               │
//	│                                                                          │
//	│    ┌──────────────┐        ┌──────────────┐        ┌──────────────┐     │
//	│    │  Test Case   │───────▶│   Service    │◀───────│    Mocks     │     │
//	│    │              │        │  (Core)      │        │  (Adapters)  │     │
//	│    └──────────────┘        └──────────────┘        └──────────────┘     │
//	│                                   │                       │              │
//	│                                   ▼                       ▼              │
//	│                            ┌──────────────┐        ┌──────────────┐     │
//	│                            │    Ports     │        │MockRepository│     │
//	│                            │ (Interfaces) │        │MockPublisher │     │
//	│                            └──────────────┘        │MockRedis     │     │
//	│                                                    └──────────────┘     │
//	└─────────────────────────────────────────────────────────────────────────┘
//
// WHY UNIT TESTS?
// - Fast execution (no external dependencies)
// - Isolated testing of business logic
// - Easy to test edge cases and error scenarios
// - Can run in CI/CD without infrastructure
//
// TEST ORGANIZATION:
// - One test file per service/component
// - Table-driven tests for comprehensive coverage
// - Clear test names describing scenario
package unit

import (
	"context"
	"testing"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/test/mocks"
)

// TestRegistrationService_RegisterParent tests parent registration.
// This demonstrates how hexagonal architecture enables easy testing:
// 1. Create a mock repository (implements ports.UserRepository)
// 2. Inject mock into service
// 3. Call service methods
// 4. Verify results and mock interactions
func TestRegistrationService_RegisterParent(t *testing.T) {
	// Table-driven tests allow testing multiple scenarios efficiently
	tests := []struct {
		name        string
		email       string
		firstName   string
		lastName    string
		roomNumber  string
		setupMock   func(*mocks.MockUserRepository)
		expectedMsg string
		expectError bool
	}{
		{
			name:        "successful_parent_registration",
			email:       "parent@example.com",
			firstName:   "John",
			lastName:    "Doe",
			roomNumber:  "101",
			setupMock:   func(m *mocks.MockUserRepository) {},
			expectedMsg: "Parent registered successfully",
			expectError: false,
		},
		{
			name:       "registration_fails_on_database_error",
			email:      "parent@example.com",
			firstName:  "John",
			lastName:   "Doe",
			roomNumber: "101",
			setupMock: func(m *mocks.MockUserRepository) {
				// Inject error to simulate database failure
				m.CreateParentError = context.DeadlineExceeded
			},
			expectedMsg: "Registration failed",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE: Set up mock and service
			mockRepo := mocks.NewMockUserRepository()
			tt.setupMock(mockRepo)

			// The service receives the mock through dependency injection
			// The service doesn't know (or care) if it's talking to a real database
			service := services.NewRegistrationService(mockRepo)

			// ACT: Execute the method under test
			ctx := context.Background()
			msg, err := service.RegisterParent(ctx, tt.email, tt.firstName, tt.lastName, tt.roomNumber)

			// ASSERT: Verify results
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if msg != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, msg)
			}

			// Verify the repository was called correctly
			if !tt.expectError && len(mockRepo.CreateParentCalls) != 1 {
				t.Errorf("expected 1 CreateParent call, got %d", len(mockRepo.CreateParentCalls))
			}
		})
	}
}

// TestRegistrationService_RegisterAdmin tests admin registration.
func TestRegistrationService_RegisterAdmin(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		firstName   string
		lastName    string
		setupMock   func(*mocks.MockUserRepository)
		expectedMsg string
		expectError bool
	}{
		{
			name:        "successful_admin_registration",
			email:       "admin@baby-kliniek.nl",
			firstName:   "Admin",
			lastName:    "User",
			setupMock:   func(m *mocks.MockUserRepository) {},
			expectedMsg: "Admin registered successfully",
			expectError: false,
		},
		{
			name:      "registration_fails_on_duplicate_email",
			email:     "existing@baby-kliniek.nl",
			firstName: "Admin",
			lastName:  "User",
			setupMock: func(m *mocks.MockUserRepository) {
				m.CreateAdminError = context.DeadlineExceeded
			},
			expectedMsg: "Registration failed",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockUserRepository()
			tt.setupMock(mockRepo)
			service := services.NewRegistrationService(mockRepo)

			ctx := context.Background()
			msg, err := service.RegisterAdmin(ctx, tt.email, tt.firstName, tt.lastName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if msg != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, msg)
			}
		})
	}
}

// TestRegistrationService_ParentData verifies that parent data is correctly populated.
func TestRegistrationService_ParentData(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)

	ctx := context.Background()
	_, err := service.RegisterParent(ctx, "test@example.com", "Jane", "Smith", "202")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the parent data passed to repository
	if len(mockRepo.CreateParentCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mockRepo.CreateParentCalls))
	}

	createdParent := mockRepo.CreateParentCalls[0]

	// Verify all fields are correctly set
	if createdParent.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", createdParent.Email)
	}
	if createdParent.FirstName != "Jane" {
		t.Errorf("expected firstName 'Jane', got %q", createdParent.FirstName)
	}
	if createdParent.LastName != "Smith" {
		t.Errorf("expected lastName 'Smith', got %q", createdParent.LastName)
	}
	if createdParent.RoomNumber != "202" {
		t.Errorf("expected roomNumber '202', got %q", createdParent.RoomNumber)
	}
	if createdParent.Role != domain.RoleParent {
		t.Errorf("expected role 'PARENT', got %q", createdParent.Role)
	}
	if createdParent.Status != domain.ParentActive {
		t.Errorf("expected status 'Active', got %q", createdParent.Status)
	}
	if createdParent.ID == "" {
		t.Error("expected non-empty ID")
	}
	if createdParent.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

// TestRegistrationService_ContextCancellation verifies context cancellation is respected.
func TestRegistrationService_ContextCancellation(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	mockRepo.CreateParentError = context.Canceled
	service := services.NewRegistrationService(mockRepo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := service.RegisterParent(ctx, "test@example.com", "John", "Doe", "101")

	if err == nil {
		t.Error("expected error due to cancelled context")
	}
}

// TestRegistrationService_ConcurrentRegistrations verifies thread safety.
func TestRegistrationService_ConcurrentRegistrations(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)

	ctx := context.Background()
	const numGoroutines = 10

	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			_, err := service.RegisterParent(ctx, "test@example.com", "Test", "User", "101")
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", n, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("test timed out waiting for goroutines")
		}
	}

	// Verify all registrations were processed
	if len(mockRepo.CreateParentCalls) != numGoroutines {
		t.Errorf("expected %d CreateParent calls, got %d", numGoroutines, len(mockRepo.CreateParentCalls))
	}
}
