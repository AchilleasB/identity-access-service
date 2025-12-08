package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/handler"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/middleware"
	"github.com/golang-jwt/jwt/v5"
)

// Mock registration service that implements ports.RegistrationService
type mockRegistrationService struct{}

func (m *mockRegistrationService) RegisterParent(ctx context.Context, email, firstName, lastName, roomNumber string) (string, error) {
	// Return a mock access code
	return "TEST-ACCESS-123", nil
}

// Test helpers
func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func createToken(privateKey *rsa.PrivateKey, role string) string {
	claims := jwt.MapClaims{
		"sub":   "user-123",
		"email": "admin@test.com",
		"role":  role,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)
	return tokenString
}

func setupTestServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	privateKey, publicKey := generateTestKeys(t)

	registrationService := &mockRegistrationService{}
	registrationHandler := handler.NewRegistrationHandler(registrationService)
	authMiddleware := middleware.NewAuthMiddleware(publicKey)

	mux := http.NewServeMux()
	mux.Handle("POST /register",
		authMiddleware.RequireRole("ADMIN")(
			http.HandlerFunc(registrationHandler.RegisterParent),
		),
	)

	server := httptest.NewServer(mux)
	return server, privateKey
}

// ==================== TESTS ====================

func TestRegistrationFlow_AdminCanRegisterParent(t *testing.T) {
	server, privateKey := setupTestServer(t)
	defer server.Close()

	adminToken := createToken(privateKey, "ADMIN")

	requestBody := map[string]string{
		"email":       "parent@example.com",
		"first_name":  "John",
		"last_name":   "Doe",
		"room_number": "101",
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", server.URL+"/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var response map[string]string
	json.NewDecoder(resp.Body).Decode(&response)

	if response["access_code"] == "" {
		t.Error("expected access_code in response")
	}
	if response["message"] != "Registration successful" {
		t.Errorf("expected success message, got %s", response["message"])
	}
}

func TestRegistrationFlow_ParentCannotRegisterParent(t *testing.T) {
	server, privateKey := setupTestServer(t)
	defer server.Close()

	parentToken := createToken(privateKey, "PARENT")

	requestBody := map[string]string{
		"email":       "another@example.com",
		"first_name":  "Jane",
		"last_name":   "Doe",
		"room_number": "102",
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", server.URL+"/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+parentToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRegistrationFlow_UnauthenticatedCannotRegister(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	requestBody := map[string]string{
		"email":       "hacker@example.com",
		"first_name":  "Hack",
		"last_name":   "Er",
		"room_number": "999",
	}
	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", server.URL+"/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRegistrationFlow_InvalidPayload(t *testing.T) {
	server, privateKey := setupTestServer(t)
	defer server.Close()

	adminToken := createToken(privateKey, "ADMIN")

	req, _ := http.NewRequest("POST", server.URL+"/register", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
