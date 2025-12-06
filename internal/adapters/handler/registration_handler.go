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
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	RoomNumber string `json:"room_number"`
}

type RegistrationResponse struct {
	Message    string `json:"message"`
	AccessCode string `json:"access_code"`
}

func (h *RegistrationHandler) RegisterParent(w http.ResponseWriter, r *http.Request) {
	var req RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	accessCode, err := h.registrationService.RegisterParent(
		r.Context(),
		req.Email,
		req.FirstName,
		req.LastName,
		req.RoomNumber,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(RegistrationResponse{
		Message:    "Registration successful",
		AccessCode: accessCode,
	})
}
