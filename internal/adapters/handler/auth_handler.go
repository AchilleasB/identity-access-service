package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/services"
)

type OAuthHandler struct {
	oauth *services.GoogleOAuthService
}

func NewOAuthHandler(oauth *services.GoogleOAuthService) *OAuthHandler {
	return &OAuthHandler{oauth: oauth}
}

func (h *OAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	log.Printf("Login endpoint hit: %s %s", r.Method, r.URL.Path)

	state, err := h.oauth.GenerateState()
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		// Secure:   false, // Important: false for localhost
		// SameSite: http.SameSiteLaxMode,
	})

	log.Printf("State cookie set: %s", state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"redirect_url": h.oauth.GetAuthURL(state),
	})
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Method: %s", r.Method)
	log.Printf("URL: %s", r.URL.String())
	log.Printf("Headers: %v", r.Header)

	stateCookie, err := r.Cookie("oauth_state")
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
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	log.Printf("Exchanging code...")
	token, err := h.oauth.Authenticate(r.Context(), code)
	if err != nil {
		log.Printf("Auth failed: %v", err)
		http.Error(w, "authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	log.Printf("Success!")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged in successfully!",
		"token":   token,
	})
}
