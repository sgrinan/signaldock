package user

import (
	"time"

	"uuid"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Role         Role
	Disabled     bool
	CreatedAt    time.Time
}
