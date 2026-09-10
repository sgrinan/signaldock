package user

import (
	"time"

	"uuid"
)

// Role identifies a user's authorization level.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

// User represents a SignalDock account.
type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Role         Role
	Disabled     bool
	CreatedAt    time.Time
}
