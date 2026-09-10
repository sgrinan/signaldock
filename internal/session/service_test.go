package session

import (
	"errors"
	"testing"
	"time"

	"uuid"
)

type fakeSessionStore struct {
	insertFunc      func(Session) error
	byTokenHashFunc func(string) (Session, error)
	deleteFunc      func(string) error
}

func (f *fakeSessionStore) Insert(session Session) error {
	if f.insertFunc != nil {
		return f.insertFunc(session)
	}

	return nil
}

func (f *fakeSessionStore) ByTokenHash(tokenHash string) (Session, error) {
	if f.byTokenHashFunc != nil {
		return f.byTokenHashFunc(tokenHash)
	}

	return Session{}, ErrSessionNotFound
}

func (f *fakeSessionStore) Delete(tokenHash string) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(tokenHash)
	}

	return nil
}

func TestService_Create(t *testing.T) {
	userID := uuid.NewV7()

	var inserted Session

	store := &fakeSessionStore{
		insertFunc: func(session Session) error {
			inserted = session
			return nil
		},
	}

	service := NewService(store)

	token, err := service.Create(userID)
	if err != nil {
		t.Fatalf("Create(%v) error = %v", userID, err)
	}

	if token == "" {
		t.Fatal("Create() token = empty string, want non-empty token")
	}

	if inserted.TokenHash != HashToken(token) {
		t.Errorf("inserted TokenHash = %q, want %q", inserted.TokenHash, HashToken(token))
	}

	if inserted.UserID != userID {
		t.Errorf("inserted UserID = %v, want %v", inserted.UserID, userID)
	}

	if inserted.ExpiresAt.IsZero() {
		t.Error("inserted ExpiresAt.IsZero() = true, want false")
	}
}

func TestService_Validate(t *testing.T) {
	token := GenerateToken()

	want := Session{
		TokenHash: HashToken(token),
		UserID:    uuid.NewV7(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	store := &fakeSessionStore{
		byTokenHashFunc: func(tokenHash string) (Session, error) {
			if tokenHash != want.TokenHash {
				t.Errorf("ByTokenHash(%q), want %q", tokenHash, want.TokenHash)
			}

			return want, nil
		},
	}

	service := NewService(store)

	got, err := service.Validate(token)
	if err != nil {
		t.Fatalf("Validate(token) error = %v", err)
	}

	if got != want {
		t.Errorf("Validate(token) = %+v, want %+v", got, want)
	}
}

func TestService_ValidateExpired(t *testing.T) {
	token := GenerateToken()
	tokenHash := HashToken(token)

	session := Session{
		TokenHash: tokenHash,
		UserID:    uuid.NewV7(),
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}

	deleted := false

	store := &fakeSessionStore{
		byTokenHashFunc: func(string) (Session, error) {
			return session, nil
		},
		deleteFunc: func(got string) error {
			if got != tokenHash {
				t.Errorf("Delete(%q), want %q", got, tokenHash)
			}

			deleted = true
			return nil
		},
	}

	service := NewService(store)

	_, err := service.Validate(token)

	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("Validate(token) error = %v, want ErrSessionExpired", err)
	}

	if !deleted {
		t.Error("Validate(token) did not delete expired session")
	}
}

func TestService_ValidateStoreError(t *testing.T) {
	wantErr := errors.New("database error")

	store := &fakeSessionStore{
		byTokenHashFunc: func(string) (Session, error) {
			return Session{}, wantErr
		},
	}

	service := NewService(store)

	_, err := service.Validate("token")

	if !errors.Is(err, wantErr) {
		t.Errorf("Validate(token) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestService_Delete(t *testing.T) {
	token := GenerateToken()
	wantHash := HashToken(token)

	store := &fakeSessionStore{
		deleteFunc: func(tokenHash string) error {
			if tokenHash != wantHash {
				t.Errorf("Delete(%q), want %q", tokenHash, wantHash)
			}

			return nil
		},
	}

	service := NewService(store)

	if err := service.Delete(token); err != nil {
		t.Fatalf("Delete(token) error = %v", err)
	}
}

func TestService_DeleteNotFound(t *testing.T) {
	store := &fakeSessionStore{
		deleteFunc: func(string) error {
			return ErrSessionNotFound
		},
	}

	service := NewService(store)

	if err := service.Delete("token"); err != nil {
		t.Errorf("Delete(token) error = %v, want nil", err)
	}
}
