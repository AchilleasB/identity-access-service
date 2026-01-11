// Package mocks provides mock implementations of port interfaces for testing.
// In hexagonal architecture, ports define the contracts between the core domain
// and external adapters. Mocks implement these interfaces to enable isolated testing.
package mocks

import (
	"context"
	"errors"
	"sync"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
)

// MockUserRepository implements ports.UserRepository for testing.
// This mock allows us to test services without a real database connection.
//
// How mocking works in hexagonal architecture:
// 1. The service depends on ports.UserRepository interface (not concrete implementation)
// 2. In production, we inject SQLRepository (real database)
// 3. In tests, we inject MockUserRepository (in-memory)
// 4. The service works the same way regardless of which implementation is used
type MockUserRepository struct {
	mu sync.RWMutex

	// In-memory storage for testing
	users   map[string]*domain.User
	parents map[string]*domain.Parent

	// Call tracking for verification
	FindByEmailCalls     []string
	CreateParentCalls    []domain.Parent
	CreateAdminCalls     []domain.User
	UpdateParentCalls    []string
	GetParentStatusCalls []string

	// Error injection for testing error scenarios
	FindByEmailError     error
	CreateParentError    error
	CreateAdminError     error
	UpdateParentError    error
	GetParentStatusError error
}

// Ensure MockUserRepository implements ports.UserRepository at compile time.
// This is a common Go pattern to catch interface mismatches early.
var _ ports.UserRepository = (*MockUserRepository)(nil)

// NewMockUserRepository creates a new mock repository with empty storage.
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:   make(map[string]*domain.User),
		parents: make(map[string]*domain.Parent),
	}
}

// SeedUser adds a user to the mock repository for test setup.
func (m *MockUserRepository) SeedUser(user *domain.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.Email] = user
}

// SeedParent adds a parent to the mock repository for test setup.
func (m *MockUserRepository) SeedParent(parent *domain.Parent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parents[parent.ID] = parent
	m.users[parent.Email] = &parent.User
}

// FindByEmail looks up a user by email address.
// This implements ports.UserRepository.FindByEmail
func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	m.FindByEmailCalls = append(m.FindByEmailCalls, email)
	m.mu.Unlock()

	// Check for injected error
	if m.FindByEmailError != nil {
		return nil, m.FindByEmailError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[email]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// CreateParent creates a new parent record.
// This implements ports.UserRepository.CreateParent
// CreateParent stores a parent entity in the mock repository for testing purposes.
// It records the function call by appending the parent to CreateParentCalls slice.
// If CreateParentError is set, it returns that error instead of storing the parent.
// Otherwise, it stores the parent in the parents map and the associated user in the users map,
// then returns a pointer to the stored parent.
func (m *MockUserRepository) CreateParent(ctx context.Context, parent domain.Parent, outboxPayload []byte) (*domain.Parent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateParentCalls = append(m.CreateParentCalls, parent)

	if m.CreateParentError != nil {
		return nil, m.CreateParentError
	}

	m.parents[parent.ID] = &parent
	m.users[parent.Email] = &parent.User
	return &parent, nil
}

// CreateAdmin creates a new admin record.
// This implements ports.UserRepository.CreateAdmin
func (m *MockUserRepository) CreateAdmin(ctx context.Context, user domain.User) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateAdminCalls = append(m.CreateAdminCalls, user)

	if m.CreateAdminError != nil {
		return nil, m.CreateAdminError
	}

	m.users[user.Email] = &user
	return &user, nil
}

// UpdateParentStatus updates a parent's status to discharged.
// This implements ports.UserRepository.UpdateParentStatus
func (m *MockUserRepository) UpdateParentStatus(ctx context.Context, parentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateParentCalls = append(m.UpdateParentCalls, parentID)

	if m.UpdateParentError != nil {
		return m.UpdateParentError
	}

	if parent, ok := m.parents[parentID]; ok {
		parent.Status = domain.ParentDischarged
	}
	return nil
}

// GetParentStatus retrieves a parent's current status.
// This implements ports.UserRepository.GetParentStatus
func (m *MockUserRepository) GetParentStatus(ctx context.Context, parentID string) (string, error) {
	m.mu.Lock()
	m.GetParentStatusCalls = append(m.GetParentStatusCalls, parentID)
	m.mu.Unlock()

	if m.GetParentStatusError != nil {
		return "", m.GetParentStatusError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if parent, ok := m.parents[parentID]; ok {
		return string(parent.Status), nil
	}
	return "", errors.New("parent not found")
}

// Reset clears all stored data and call tracking.
// Use this between tests to ensure isolation.
func (m *MockUserRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.users = make(map[string]*domain.User)
	m.parents = make(map[string]*domain.Parent)
	m.FindByEmailCalls = nil
	m.CreateParentCalls = nil
	m.CreateAdminCalls = nil
	m.UpdateParentCalls = nil
	m.GetParentStatusCalls = nil
	m.FindByEmailError = nil
	m.CreateParentError = nil
	m.CreateAdminError = nil
	m.UpdateParentError = nil
	m.GetParentStatusError = nil
}
