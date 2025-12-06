package repository

import (
	"context"
	"database/sql"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
)

type SQLRepository struct {
	db *sql.DB
}

// Ensure SQLRepository implements ports.UserRepository
var _ ports.UserRepository = (*SQLRepository)(nil)

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	var password string
	err := r.db.QueryRowContext(
		ctx,
		"SELECT id, email, role, password FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email, &user.Role, &password)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *SQLRepository) CreateParent(ctx context.Context, parent domain.Parent) error {
	_, err := r.db.Exec(
		"INSERT INTO parents (id, email, role, first_name, last_name, room_number, access_code) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		parent.ID,
		parent.Email,
		parent.Role,
		parent.FirstName,
		parent.LastName,
		parent.RoomNumber,
		parent.Password)
	return err
}

func (r *SQLRepository) SaveToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(
		"INSERT INTO tokens (user_id, token) VALUES ($1, $2)",
		userID,
		token,
	)
	return err
}
