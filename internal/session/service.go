package session

import (
	"errors"
	"fmt"
	"time"

	"uuid"
)

const sessionDuration = 24 * time.Hour

type sessionStore interface {
	Insert(Session) error
	ByTokenHash(string) (Session, error)
	Delete(string) error
}

type Service struct {
	store sessionStore
}

func NewService(store sessionStore) *Service {
	return &Service{
		store: store,
	}
}

func (s *Service) Create(userID uuid.UUID) (string, error) {
	token := GenerateToken()
	hash := HashToken(token)

	session := Session{
		TokenHash: hash,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	if err := s.store.Insert(session); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

func (s *Service) Validate(token string) (Session, error) {
	hash := HashToken(token)

	session, err := s.store.ByTokenHash(hash)
	if err != nil {
		return Session{}, fmt.Errorf("validate session: %w", err)
	}

	if !time.Now().UTC().Before(session.ExpiresAt) {
		if err := s.store.Delete(hash); err != nil {
			return Session{}, fmt.Errorf("delete expired session: %w", err)
		}

		return Session{}, ErrSessionExpired
	}

	return session, nil
}

func (s *Service) Delete(token string) error {
	hash := HashToken(token)

	if err := s.store.Delete(hash); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}

		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}
