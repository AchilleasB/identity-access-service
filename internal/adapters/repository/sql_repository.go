package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/domain"
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	"github.com/google/uuid"
	"github.com/sony/gobreaker"
)

type SQLRepository struct {
	db *sql.DB
	cb *gobreaker.CircuitBreaker
}

var _ ports.UserRepository = (*SQLRepository)(nil)

func NewSQLRepository(db *sql.DB) *SQLRepository {
	// Configure circuit breaker for database operations
	cb := config.NewCircuitBreaker("PostgreSQL")

	return &SQLRepository{
		db: db,
		cb: cb,
	}
}

func (r *SQLRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	result, err := r.cb.Execute(func() (interface{}, error) {
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
	})
	if err != nil {
		return nil, err
	}
	return result.(*domain.User), nil
}

func (r *SQLRepository) CreateParent(ctx context.Context, parent domain.Parent, outboxPayload []byte) (*domain.Parent, error) {
	result, err := r.cb.Execute(func() (interface{}, error) {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer func() { _ = tx.Rollback() }()

		_, err = tx.ExecContext(ctx,
			"INSERT INTO users (id, email, role, first_name, last_name, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
			parent.ID, parent.Email, parent.Role, parent.FirstName, parent.LastName, parent.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		_, err = tx.ExecContext(ctx,
			"INSERT INTO parents (user_id, room_number, status) VALUES ($1, $2, $3)",
			parent.ID, parent.RoomNumber, parent.Status,
		)
		if err != nil {
			return nil, err
		}

		if len(outboxPayload) > 0 {
			var tmp map[string]any
			if err := json.Unmarshal(outboxPayload, &tmp); err != nil {
				return nil, err
			}

			_, err = tx.ExecContext(ctx,
				"INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
				uuid.NewString(),
				"parent",
				parent.ID,
				"babies",
				outboxPayload,
				time.Now(),
			)
			if err != nil {
				return nil, err
			}
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &parent, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*domain.Parent), nil
}

func (r *SQLRepository) CreateAdmin(ctx context.Context, user domain.User) (*domain.User, error) {
	_, err := r.cb.Execute(func() (interface{}, error) {
		_, err := r.db.ExecContext(ctx,
			"INSERT INTO users (id, email, role, first_name, last_name, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
			user.ID, user.Email, user.Role, user.FirstName, user.LastName, user.CreatedAt,
		)
		return nil, err
	})
	return &user, err
}

func (r *SQLRepository) UpdateParentStatus(ctx context.Context, parentID string) error {
	_, err := r.cb.Execute(func() (interface{}, error) {
		_, err := r.db.ExecContext(ctx,
			"UPDATE parents SET status = 'Discharged' WHERE user_id = $1",
			parentID,
		)
		return nil, err
	})
	return err
}

func (r *SQLRepository) GetParentStatus(ctx context.Context, parentID string) (string, error) {
	result, err := r.cb.Execute(func() (interface{}, error) {
		var status string
		err := r.db.QueryRowContext(
			ctx,
			"SELECT status FROM parents WHERE user_id = $1",
			parentID,
		).Scan(&status)
		if err != nil {
			return "", err
		}
		return status, nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}
