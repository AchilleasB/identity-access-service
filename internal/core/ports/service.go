package ports

import (
	"context"
)

type AuthService interface {
	Login(ctx context.Context, email string, password string) (string, error)
	Logout(ctx context.Context, token string) error
}

type RegistrationService interface {
	RegisterParent(ctx context.Context, email, firstName, lastName, roomNumber string) (string, error)
	RegisterAdmin(ctx context.Context, email, firstName, lastName string) (string, error)
}
