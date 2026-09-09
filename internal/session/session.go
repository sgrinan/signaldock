package session

import (
	"time"

	"uuid"
)

type Session struct {
	TokenHash string
	UserID    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
}
