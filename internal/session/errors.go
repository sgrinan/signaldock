package session

import "errors"

var (
	ErrSessionExists   = errors.New("session already exists")
	ErrSessionNotFound = errors.New("session not found")
)
