package session

import (
	"time"

	"uuid"
)

// Session represents an authenticated user session.
type Session struct {
	TokenHash string
	UserID    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
}
