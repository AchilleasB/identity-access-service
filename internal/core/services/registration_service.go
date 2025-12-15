package services

import (
	"context"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/google/uuid"
)

type RegistrationService struct {
	userRepo ports.UserRepository
}

func NewRegistrationService(userRepo ports.UserRepository) *RegistrationService {
	return &RegistrationService{userRepo: userRepo}
}

func (s *RegistrationService) RegisterParent(
	ctx context.Context,
	email, firstName, lastName, roomNumber string,
) (string, error) {

	parent := domain.Parent{
		User: domain.User{
			ID:        uuid.NewString(),
			Email:     email,
			Role:      domain.RoleParent,
			CreatedAt: time.Now(),
			FirstName: firstName,
			LastName:  lastName,
		},
		RoomNumber: roomNumber,
	}

	_, err := s.userRepo.CreateParent(ctx, parent)
	if err != nil {
		return "Registration failed", err
	}

	return "Parent registered successfully", nil
}

func (s *RegistrationService) RegisterAdmin(
	ctx context.Context,
	email, firstName, lastName string,
) (string, error) {

	user := domain.User{
		ID:        uuid.NewString(),
		Email:     email,
		Role:      domain.RoleParent,
		CreatedAt: time.Now(),
		FirstName: firstName,
		LastName:  lastName,
	}

	_, err := s.userRepo.CreateAdmin(ctx, user)
	if err != nil {
		return "Registration failed", err
	}

	return "Admin registered successfully", nil
}
