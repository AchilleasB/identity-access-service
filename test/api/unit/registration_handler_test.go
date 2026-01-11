package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/test/mocks"
)

// TestRegistrationHandler tests the HTTP handler layer.
// This demonstrates testing the "driving adapter" (HTTP handler) side
// of the hexagonal architecture.
//
// HANDLER TESTING PATTERN:
// ═══════════════════════════════════════════════════════════════════════════════
//
//	HTTP Request ──▶ Handler ──▶ Service (via port) ──▶ Mock Repository
//	                    │
//	                    ▼
//	HTTP Response (validated by test)
//
// We use httptest.NewRecorder() to capture the response without starting a server.

// TestRegistrationHandler_Register_Success tests successful registration.
func TestRegistrationHandler_Register_Success(t *testing.T) {
	// ARRANGE
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	// Create request body
	body := map[string]string{
		"email":       "parent@example.com",
		"role":        "PARENT",
		"first_name":  "John",
		"last_name":   "Doe",
		"room_number": "101",
	}
	jsonBody, _ := json.Marshal(body)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rec := httptest.NewRecorder()

	// ACT
	h.Register(rec, req)

	// ASSERT
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["message"] != "Parent registered successfully" {
		t.Errorf("unexpected message: %s", response["message"])
	}

	// Verify repository was called
	if len(mockRepo.CreateParentCalls) != 1 {
		t.Errorf("expected 1 CreateParent call, got %d", len(mockRepo.CreateParentCalls))
	}
}

// TestRegistrationHandler_Register_AdminRole tests admin registration.
func TestRegistrationHandler_Register_AdminRole(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	body := map[string]string{
		"email":      "admin@baby-kliniek.nl",
		"role":       "ADMIN",
		"first_name": "Admin",
		"last_name":  "User",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if len(mockRepo.CreateAdminCalls) != 1 {
		t.Errorf("expected 1 CreateAdmin call, got %d", len(mockRepo.CreateAdminCalls))
	}
}

// TestRegistrationHandler_Register_InvalidMethod tests method not allowed.
func TestRegistrationHandler_Register_InvalidMethod(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	// Test with GET instead of POST
	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

// TestRegistrationHandler_Register_InvalidJSON tests malformed JSON handling.
func TestRegistrationHandler_Register_InvalidJSON(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	// Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestRegistrationHandler_Register_UnsupportedRole tests invalid role handling.
func TestRegistrationHandler_Register_UnsupportedRole(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	body := map[string]string{
		"email":      "user@example.com",
		"role":       "INVALID_ROLE",
		"first_name": "Test",
		"last_name":  "User",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestRegistrationHandler_Register_DatabaseError tests database error handling.
func TestRegistrationHandler_Register_DatabaseError(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	mockRepo.CreateParentError = context.DeadlineExceeded
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	body := map[string]string{
		"email":       "parent@example.com",
		"role":        "PARENT",
		"first_name":  "John",
		"last_name":   "Doe",
		"room_number": "101",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// TestRegistrationHandler_ContentTypeValidation ensures Content-Type is set correctly.
func TestRegistrationHandler_ContentTypeValidation(t *testing.T) {
	mockRepo := mocks.NewMockUserRepository()
	service := services.NewRegistrationService(mockRepo)
	h := handler.NewRegistrationHandler(service)

	body := map[string]string{
		"email":       "parent@example.com",
		"role":        "PARENT",
		"first_name":  "John",
		"last_name":   "Doe",
		"room_number": "101",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}
}
