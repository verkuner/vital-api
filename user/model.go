package user

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user profile stored locally.
type User struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	ProviderID  string     `json:"provider_id" db:"provider_id"`
	Email       string     `json:"email" db:"email"`
	Name        string     `json:"name" db:"name"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty" db:"date_of_birth"`
	AvatarURL   *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Device represents a registered monitoring device.
type Device struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Name      string    `json:"name" db:"name"`
	Type      string    `json:"type" db:"type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
