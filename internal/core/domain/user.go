package domain

import "time"

type Role string

const (
	RoleAdmin  Role = "ADMIN"
	RoleParent Role = "PARENT"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type Parent struct {
	User
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	RoomNumber string `json:"room_number"`
}
