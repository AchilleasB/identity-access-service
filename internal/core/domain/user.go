package domain

import "time"

type Role string

const (
	RoleAdmin  Role = "ADMIN"
	RoleParent Role = "PARENT"
)

type ParentStatus string

const (
	ParentActive     ParentStatus = "Active"
	ParentDischarged ParentStatus = "Discharged"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	CreatedAt time.Time `json:"created_at"`
}

type ParentStatus string

const (
	StatusActive     ParentStatus = "Active"
	StatusDischarged ParentStatus = "Discharged"
)

type Parent struct {
	User
	RoomNumber string       `json:"room_number"`
	Status     ParentStatus `json:"status"`
}
