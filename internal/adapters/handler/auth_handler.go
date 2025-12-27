package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/adapters/middleware"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(auth *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: auth}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("Login endpoint hit: %s %s", r.Method, r.URL.Path)

	state, err := h.authService.GenerateState()
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
	})

	log.Printf("State cookie set: %s", state)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"redirect_url": h.authService.GetAuthURL(state),
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (h *AuthHandler) LoginCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("Method: %s", r.Method)
	log.Printf("URL: %s", r.URL.String())
	log.Printf("Headers: %v", r.Header)

	stateCookie, err := r.Cookie("auth_state")
	if err != nil {
		log.Printf("Missing state cookie: %v", err)
		log.Printf("All cookies: %v", r.Cookies())
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam != stateCookie.Value {
		log.Printf("State mismatch")
		http.Error(w, "invalid state", http.StatusForbidden)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "auth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := h.authService.Authenticate(r.Context(), code)
	if err != nil {
		log.Printf("Auth failed: %v", err)
		http.Error(w, "authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged in successfully!",
		"token":   token,
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tokenString, ok := r.Context().Value(middleware.TokenKey).(string)
	if !ok || tokenString == "" {
		http.Error(w, "missing token in context", http.StatusUnauthorized)
		return
	}

	err := h.authService.Logout(r.Context(), tokenString)
	if err != nil {
		log.Printf("Logout failed: %v", err)
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "logged out successfully"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (h *AuthHandler) DischargeParent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		ParentId string `json:"parent_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if payload.ParentId == "" {
		http.Error(w, "missing parent_id", http.StatusBadRequest)
		return
	}

	err := h.authService.DischargeParent(r.Context(), payload.ParentId)
	if err != nil {
		log.Printf("Discharge parent failed: %v %v", payload.ParentId, err)
		http.Error(w, "discharge parent failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(
		map[string]string{"message": "parent discharged successfully"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
