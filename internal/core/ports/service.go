package ports

import (
	"context"
)

type AuthService interface {
	Login(ctx context.Context, email string, password string) (string, error)
}

type RegistrationService interface {
	RegisterParent(ctx context.Context, email, firstName, lastName string, roomNumber int) (string, error)
}
