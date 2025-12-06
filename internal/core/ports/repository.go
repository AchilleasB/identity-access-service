package ports

import (
	"context"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	CreateParent(ctx context.Context, parent domain.Parent) error
	SaveToken(ctx context.Context, userID, token string) error
}
