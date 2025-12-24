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

var _ ports.UserRepository = (*SQLRepository)(nil)

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.QueryRowContext(
		ctx,
		"SELECT id, email, role, first_name, last_name, created_at FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email, &user.Role, &user.FirstName, &user.LastName, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *SQLRepository) CreateParent(ctx context.Context, parent domain.Parent) (*domain.Parent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO users (id, email, role, first_name, last_name, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		parent.ID, parent.Email, parent.Role, parent.FirstName, parent.LastName, parent.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO parents (user_id, room_number, status) VALUES ($1, $2, $3)",
		parent.ID, parent.RoomNumber, string(parent.Status),
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &parent, nil
}

func (r *SQLRepository) CreateAdmin(ctx context.Context, user domain.User) (*domain.User, error) {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO users (id, email, role, first_name, last_name, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		user.ID, user.Email, user.Role, user.FirstName, user.LastName, user.CreatedAt,
	)
	return &user, err
}

func (r *SQLRepository) SaveToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(
		"INSERT INTO tokens (user_id, token_hash, created_at, expires_at) VALUES ($1, $2, NOW(), NOW() + INTERVAL '7 days')",
		userID,
		token,
	)
	return err
}

func (r *SQLRepository) BlacklistToken(ctx context.Context, tokenHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM tokens WHERE token_hash = $1", tokenHash)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO token_blacklist (token_hash, blacklisted_at) 
         VALUES ($1, NOW()) 
         ON CONFLICT (token_hash) DO NOTHING`,
		tokenHash,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
