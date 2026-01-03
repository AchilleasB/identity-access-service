package ports

import (
	"context"
)

type CreateBabyEvent struct {
	UserID     string `json:"user_id"`
	LastName   string `json:"last_name"`
	RoomNumber string `json:"room_number"`
}

type BabyEventPublisher interface {
	PublishBabyCreated(ctx context.Context, evt CreateBabyEvent) error
}
