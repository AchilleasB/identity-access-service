package services

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type RegistrationService struct {
	userRepo ports.UserRepository
}

func NewRegistrationService(userRepo ports.UserRepository) *RegistrationService {
	return &RegistrationService{userRepo: userRepo}
}

func (s *RegistrationService) RegisterParent(
	ctx context.Context,
	email, firstName, lastName string,
	roomNumber int,
) (string, error) {
	// Generate and hash the access code before storing
	accessCode, err := s.generateAccessCode()
	if err != nil {
		return "", err
	}
	hashedCode, err := bcrypt.GenerateFromPassword([]byte(accessCode), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	parent := domain.Parent{
		User: domain.User{
			ID:        uuid.NewString(),
			Email:     email,
			Role:      domain.RoleParent,
			Password:  string(hashedCode),
			CreatedAt: time.Now(),
		},
		FirstName:  firstName,
		LastName:   lastName,
		RoomNumber: roomNumber,
	}
	if err := s.userRepo.CreateParent(ctx, parent); err != nil {
		return "", err
	}

	return accessCode, nil
}

func (s *RegistrationService) generateAccessCode() (string, error) {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"

	result := make([]byte, 4)

	// generate letter-digit interchangeably
	num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
	if err != nil {
		return "", err
	}
	result[0] = letters[num.Int64()]

	num, err = rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
	if err != nil {
		return "", err
	}
	result[1] = digits[num.Int64()]

	num, err = rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
	if err != nil {
		return "", err
	}
	result[2] = letters[num.Int64()]

	num, err = rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
	if err != nil {
		return "", err
	}
	result[3] = digits[num.Int64()]

	return string(result), nil
}
