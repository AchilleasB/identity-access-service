package unit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
)

// TestHealthHandler tests the health check endpoints.
// Note: Health handlers have special requirements because they need
// actual database and Redis connections to check health.
// For unit tests, we test the handler logic with nil dependencies.
// For proper health checks, use integration tests.

// TestHealthHandler_Health_ProcessCheck tests the basic health endpoint.
func TestHealthHandler_Health_ProcessCheck(t *testing.T) {
	// Create handler with nil dependencies (tests process health only)
	h := handler.NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response handler.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "UP" {
		t.Errorf("expected status 'UP', got %q", response.Status)
	}

	if _, ok := response.Checks["process"]; !ok {
		t.Error("expected 'process' check in response")
	}
}

// TestHealthHandler_Health_InvalidMethod tests method validation.
func TestHealthHandler_Health_InvalidMethod(t *testing.T) {
	h := handler.NewHealthHandler(nil, nil)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rec := httptest.NewRecorder()

			h.Health(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d for %s, got %d", http.StatusMethodNotAllowed, method, rec.Code)
			}
		})
	}
}

// TestHealthHandler_Live tests the liveness endpoint.
func TestHealthHandler_Live(t *testing.T) {
	h := handler.NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()

	h.Live(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestHealthHandler_Ready_NoDependencies tests ready endpoint without real dependencies.
// Note: This will show DOWN status because db and redis are nil.
func TestHealthHandler_Ready_NoDependencies(t *testing.T) {
	h := handler.NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	// Without dependencies, ready check will fail
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

// TestHealthHandler_ContentType ensures JSON content type is set.
func TestHealthHandler_ContentType(t *testing.T) {
	h := handler.NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}
}

// TestHealthHandler_UptimeIncreases verifies uptime is tracked.
func TestHealthHandler_UptimeIncreases(t *testing.T) {
	h := handler.NewHealthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	var response handler.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}
