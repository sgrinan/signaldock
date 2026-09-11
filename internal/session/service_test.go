package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"uuid"
)

func TestService_Create(t *testing.T) {
	ctx := t.Context()

	userID := uuid.NewV7()

	var inserted Session

	repository := &fakeSessionRepository{
		insertFunc: func(ctx context.Context, session Session) error {
			inserted = session
			return nil
		},
	}

	service := NewService(repository)

	token, err := service.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create(%v) error = %v, want nil", userID, err)
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
	ctx := t.Context()

	token := GenerateToken()

	want := Session{
		TokenHash: HashToken(token),
		UserID:    uuid.NewV7(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	repository := &fakeSessionRepository{
		byTokenHashFunc: func(ctx context.Context, tokenHash string) (Session, error) {
			if tokenHash != want.TokenHash {
				t.Errorf("ByTokenHash(%q), want %q", tokenHash, want.TokenHash)
			}

			return want, nil
		},
	}

	service := NewService(repository)

	got, err := service.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate(token) error = %v, want nil", err)
	}

	if got != want {
		t.Errorf("Validate(token) = %+v, want %+v", got, want)
	}
}

func TestService_ValidateExpired(t *testing.T) {
	ctx := t.Context()

	token := GenerateToken()
	tokenHash := HashToken(token)

	session := Session{
		TokenHash: tokenHash,
		UserID:    uuid.NewV7(),
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	deleted := false

	repository := &fakeSessionRepository{
		byTokenHashFunc: func(context.Context, string) (Session, error) {
			return session, nil
		},
		deleteFunc: func(ctx context.Context, got string) error {
			if got != tokenHash {
				t.Errorf("Delete(%q), want %q", got, tokenHash)
			}

			deleted = true
			return nil
		},
	}

	service := NewService(repository)

	_, err := service.Validate(ctx, token)

	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("Validate(token) error = %v, want ErrSessionExpired", err)
	}

	if !deleted {
		t.Error("Validate(token) did not delete expired session")
	}
}

func TestService_ValidateRepositoryError(t *testing.T) {
	ctx := t.Context()

	wantErr := errors.New("database error")

	repository := &fakeSessionRepository{
		byTokenHashFunc: func(context.Context, string) (Session, error) {
			return Session{}, wantErr
		},
	}

	service := NewService(repository)

	_, err := service.Validate(ctx, "token")

	if !errors.Is(err, wantErr) {
		t.Errorf("Validate(token) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestService_Delete(t *testing.T) {
	ctx := t.Context()

	token := GenerateToken()
	wantHash := HashToken(token)

	repository := &fakeSessionRepository{
		deleteFunc: func(ctx context.Context, tokenHash string) error {
			if tokenHash != wantHash {
				t.Errorf("Delete(%q), want %q", tokenHash, wantHash)
			}

			return nil
		},
	}

	service := NewService(repository)

	if err := service.Delete(ctx, token); err != nil {
		t.Fatalf("Delete(token) error = %v, want nil", err)
	}
}

func TestService_DeleteNotFound(t *testing.T) {
	ctx := t.Context()

	repository := &fakeSessionRepository{
		deleteFunc: func(context.Context, string) error {
			return ErrSessionNotFound
		},
	}

	service := NewService(repository)

	if err := service.Delete(ctx, "token"); err != nil {
		t.Errorf("Delete(token) error = %v, want nil", err)
	}
}
