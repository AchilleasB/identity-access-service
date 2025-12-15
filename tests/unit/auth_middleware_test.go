package tests

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/middleware"
	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func createTestToken(privateKey *rsa.PrivateKey, role string, expired bool) string {
	exp := time.Now().Add(time.Hour)
	if expired {
		exp = time.Now().Add(-time.Hour)
	}

	claims := jwt.MapClaims{
		"sub":   "user-123",
		"email": "test@example.com",
		"role":  role,
		"exp":   exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)
	return tokenString
}

func TestRequireRole_NoAuthHeader(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	middleware := NewAuthMiddleware(publicKey)

	handler := middleware.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/register", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_InvalidHeaderFormat(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	middleware := NewAuthMiddleware(publicKey)

	handler := middleware.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/register", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_InvalidToken(t *testing.T) {
	_, publicKey := generateTestKeys(t)
	middleware := NewAuthMiddleware(publicKey)

	handler := middleware.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/register", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	middleware := NewAuthMiddleware(publicKey)

	token := createTestToken(privateKey, "ADMIN", true) // expired

	handler := middleware.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/register", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_WrongRole(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	middleware := NewAuthMiddleware(publicKey)

	token := createTestToken(privateKey, "PARENT", false) // wrong role

	handler := middleware.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/register", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_ValidAdminToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	middleware := NewAuthMiddleware(publicKey)

	token := createTestToken(privateKey, "ADMIN", false) // valid admin

	handlerCalled := false
	handler := middleware.RequireRole("ADMIN", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Verify claims are in context
		claims, ok := r.Context().Value(RoleKey).(string)
		if !ok {
			t.Error("claims not found in context")
		}
		if claims != "ADMIN" {
			t.Errorf("expected role ADMIN, got %s", claims)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/register", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}
