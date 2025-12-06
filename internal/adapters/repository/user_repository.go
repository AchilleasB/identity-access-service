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
		"SELECT id, email, role, password, created_at FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email, &user.Role, &user.Password, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *SQLRepository) CreateParent(ctx context.Context, parent domain.Parent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO users (id, email, role, password, created_at) VALUES ($1, $2, $3, $4, $5)",
		parent.ID,
		parent.Email,
		parent.Role,
		parent.Password,
		parent.CreatedAt,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO parents (user_id, first_name, last_name, room_number) VALUES ($1, $2, $3, $4)",
		parent.ID,
		parent.FirstName,
		parent.LastName,
		parent.RoomNumber,
	)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *SQLRepository) SaveToken(ctx context.Context, userID, token string) error {
	_, err := r.db.Exec(
		"INSERT INTO tokens (user_id, token) VALUES ($1, $2)",
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
