package mocks

import (
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
)

// CreateTestEvent creates a sample event for testing.
func CreateTestEvent() ports.CreateBabyEvent {
	return ports.CreateBabyEvent{
		UserID:     "test-user-id",
		LastName:   "TestFamily",
		RoomNumber: "TEST-101",
	}
}

// CreateTestEventWithData creates a customized test event.
func CreateTestEventWithData(userID, lastName, roomNumber string) ports.CreateBabyEvent {
	return ports.CreateBabyEvent{
		UserID:     userID,
		LastName:   lastName,
		RoomNumber: roomNumber,
	}
}
