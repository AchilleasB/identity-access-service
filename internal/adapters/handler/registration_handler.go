package handler

import (
	"encoding/json"
	"net/http"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
)

type RegistrationHandler struct {
	registrationService ports.RegistrationService
}

func NewRegistrationHandler(registration ports.RegistrationService) *RegistrationHandler {
	return &RegistrationHandler{registrationService: registration}
}

type RegistrationRequest struct {
	Email      string `json:"email"`
	Role       string `json:"role"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	RoomNumber string `json:"room_number,omitempty"`
}

type RegistrationResponse struct {
	Message string `json:"message"`
}

func (h *RegistrationHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var message string
	var err error

	switch req.Role {
	case "PARENT":
		message, err = h.registrationService.RegisterParent(r.Context(), req.Email, req.FirstName, req.LastName, req.RoomNumber)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "ADMIN":
		message, err = h.registrationService.RegisterAdmin(r.Context(), req.Email, req.FirstName, req.LastName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Unsupported role", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(RegistrationResponse{
		Message: message,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
