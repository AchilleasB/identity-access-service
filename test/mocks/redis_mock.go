package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockRedisClient provides a minimal mock for Redis operations used in auth.
// It is a simplified mock that sufficiently implements the methods we need for testing,
type MockRedisClient struct {
	mu   sync.RWMutex
	data map[string]mockRedisValue

	// Error injection
	SetError    error
	GetError    error
	DelError    error
	ExistsError error
}

type mockRedisValue struct {
	value     string
	expiresAt time.Time
}

// NewMockRedisClient creates a new mock Redis client.
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]mockRedisValue),
	}
}

// Set stores a value with optional expiration.
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewStatusCmd(ctx)

	if m.SetError != nil {
		cmd.SetErr(m.SetError)
		return cmd
	}

	expiresAt := time.Time{}
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	m.data[key] = mockRedisValue{
		value:     value.(string),
		expiresAt: expiresAt,
	}

	cmd.SetVal("OK")
	return cmd
}

// Get retrieves a value by key.
func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := redis.NewStringCmd(ctx)

	if m.GetError != nil {
		cmd.SetErr(m.GetError)
		return cmd
	}

	val, ok := m.data[key]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}

	// Check expiration
	if !val.expiresAt.IsZero() && time.Now().After(val.expiresAt) {
		cmd.SetErr(redis.Nil)
		return cmd
	}

	cmd.SetVal(val.value)
	return cmd
}

// Del deletes keys.
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewIntCmd(ctx)

	if m.DelError != nil {
		cmd.SetErr(m.DelError)
		return cmd
	}

	var deleted int64
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)
			deleted++
		}
	}

	cmd.SetVal(deleted)
	return cmd
}

// Exists checks if keys exist.
func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := redis.NewIntCmd(ctx)

	if m.ExistsError != nil {
		cmd.SetErr(m.ExistsError)
		return cmd
	}

	var count int64
	for _, key := range keys {
		val, ok := m.data[key]
		if ok && (val.expiresAt.IsZero() || time.Now().Before(val.expiresAt)) {
			count++
		}
	}

	cmd.SetVal(count)
	return cmd
}

// Ping checks connection.
func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("PONG")
	return cmd
}

// Reset clears all data.
func (m *MockRedisClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]mockRedisValue)
	m.SetError = nil
	m.GetError = nil
	m.DelError = nil
	m.ExistsError = nil
}

// SetKey directly sets a key (for test setup).
func (m *MockRedisClient) SetKey(key, value string, expiration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expiresAt := time.Time{}
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	m.data[key] = mockRedisValue{
		value:     value,
		expiresAt: expiresAt,
	}
}

// HasKey checks if a key exists (for test assertions).
func (m *MockRedisClient) HasKey(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.data[key]
	if !ok {
		return false
	}
	if !val.expiresAt.IsZero() && time.Now().After(val.expiresAt) {
		return false
	}
	return true
}
