package session

import (
	"errors"
	"fmt"
	"time"

	"uuid"
)

const sessionDuration = 24 * time.Hour

type sessionRepository interface {
	Insert(Session) error
	ByTokenHash(string) (Session, error)
	Delete(string) error
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
func (s *Service) Create(userID uuid.UUID) (string, error) {
	token := GenerateToken()
	hash := HashToken(token)

	session := Session{
		TokenHash: hash,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	if err := s.repository.Insert(session); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

// Validate returns the session associated with token.
// Expired sessions are deleted and reported as ErrSessionExpired.
func (s *Service) Validate(token string) (Session, error) {
	hash := HashToken(token)

	session, err := s.repository.ByTokenHash(hash)
	if err != nil {
		return Session{}, fmt.Errorf("validate session: %w", err)
	}

	if !time.Now().Before(session.ExpiresAt) {
		if err := s.repository.Delete(hash); err != nil {
			return Session{}, fmt.Errorf("delete expired session: %w", err)
		}

		return Session{}, ErrSessionExpired
	}

	return session, nil
}

// Delete removes the session associated with token.
// Deleting an already missing session succeeds.
func (s *Service) Delete(token string) error {
	hash := HashToken(token)

	if err := s.repository.Delete(hash); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}

		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}
