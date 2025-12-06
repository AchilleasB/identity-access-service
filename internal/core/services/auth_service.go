package services

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo   ports.UserRepository
	privateKey *rsa.PrivateKey
}

func NewAuthService(repo ports.UserRepository, privateKey *rsa.PrivateKey) *AuthService {
	return &AuthService{
		userRepo:   repo,
		privateKey: privateKey,
	}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid email")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid password")
	}

	token, err := s.generateJWT(user.ID, user.Role)
	if err != nil {
		return "", err
	}

	tokenHash := hashToken(token)
	if err := s.userRepo.SaveToken(ctx, user.ID, tokenHash); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	tokenHash := hashToken(token)
	return s.userRepo.BlacklistToken(ctx, tokenHash)
}

func (s *AuthService) generateJWT(uid string, role domain.Role) (string, error) {
	claims := jwt.MapClaims{
		"sub":  uid,
		"role": role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "baby-kliniek-ias",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
