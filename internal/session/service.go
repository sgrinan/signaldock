package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"uuid"
)

const sessionDuration = 24 * time.Hour

type sessionRepository interface {
	Insert(context.Context, Session) error
	ByTokenHash(context.Context, string) (Session, error)
	Delete(context.Context, string) error
}

// Service manages session creation, validation, and deletion.
type Service struct {
	repository sessionRepository
}

// NewService returns a Service backed by repository.
func NewService(repository sessionRepository) *Service {
	return &Service{
		repository: repository,
	}
}

// Create creates a session for userID and returns the raw session token.
func (s *Service) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	token := generateToken()
	hash := hashToken(token)

	session := Session{
		TokenHash: hash,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	if err := s.repository.Insert(ctx, session); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

// Validate returns the session associated with token.
// Expired sessions are deleted and reported as ErrSessionExpired.
func (s *Service) Validate(ctx context.Context, token string) (Session, error) {
	hash := hashToken(token)

	session, err := s.repository.ByTokenHash(ctx, hash)
	if err != nil {
		return Session{}, fmt.Errorf("validate session: %w", err)
	}

	if !time.Now().Before(session.ExpiresAt) {
		if err := s.repository.Delete(ctx, hash); err != nil {
			return Session{}, fmt.Errorf("delete expired session: %w", err)
		}

		return Session{}, ErrSessionExpired
	}

	return session, nil
}

// Delete removes the session associated with token.
// Deleting an already missing session succeeds.
func (s *Service) Delete(ctx context.Context, token string) error {
	hash := hashToken(token)

	if err := s.repository.Delete(ctx, hash); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}

		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}
