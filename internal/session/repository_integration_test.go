package session

import (
	"errors"
	"testing"
	"time"
)

func TestRepository_Insert(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)
	userID := createTestUser(t, repository)

	want := Session{
		TokenHash: "token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := repository.Insert(ctx, want); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	got, err := repository.ByTokenHash(ctx, want.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash(%q) error = %v, want nil", want.TokenHash, err)
	}

	if got.TokenHash != want.TokenHash {
		t.Errorf("TokenHash = %q, want %q, want nil", got.TokenHash, want.TokenHash)
	}

	if got.UserID != want.UserID {
		t.Errorf("UserID = %v, want %v", got.UserID, want.UserID)
	}

	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt.IsZero() = true, want false")
	}
}

func TestRepository_InsertDuplicate(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)
	userID := createTestUser(t, repository)

	session := Session{
		TokenHash: "duplicate-token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := repository.Insert(ctx, session); err != nil {
		t.Fatalf("first Insert() error = %v, want nil", err)
	}

	err := repository.Insert(ctx, session)

	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("second Insert() error = %v, want ErrSessionExists", err)
	}
}

func TestRepository_ByTokenHash(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)
	userID := createTestUser(t, repository)

	want := Session{
		TokenHash: "token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := repository.Insert(ctx, want); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	got, err := repository.ByTokenHash(ctx, want.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash(%q) error = %v, want nil", want.TokenHash, err)
	}

	if got.TokenHash != want.TokenHash {
		t.Errorf("TokenHash = %q, want %q", got.TokenHash, want.TokenHash)
	}

	if got.UserID != want.UserID {
		t.Errorf("UserID = %v, want %v", got.UserID, want.UserID)
	}

	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt.IsZero() = true, want false")
	}
}

func TestRepository_ByTokenHashNotFound(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	_, err := repository.ByTokenHash(ctx, "missing-token-hash")

	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ByTokenHash(%q) error = %v, want ErrSessionNotFound", "missing-token-hash", err)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)
	userID := createTestUser(t, repository)

	session := Session{
		TokenHash: "token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := repository.Insert(ctx, session); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := repository.Delete(ctx, session.TokenHash); err != nil {
		t.Fatalf("Delete(%q) error = %v, want nil", session.TokenHash, err)
	}

	_, err := repository.ByTokenHash(ctx, session.TokenHash)

	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ByTokenHash(%q) after Delete() error = %v, want ErrSessionNotFound", session.TokenHash, err)
	}
}

func TestRepository_DeleteNotFound(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	err := repository.Delete(ctx, "missing-token-hash")

	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Delete(%q) error = %v, want ErrSessionNotFound", "missing-token-hash", err)
	}
}

func TestRepository_DeleteExpired(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)
	userID := createTestUser(t, repository)

	expired := Session{
		TokenHash: "expired-token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().Add(-time.Hour).Truncate(time.Microsecond),
	}

	active := Session{
		TokenHash: "active-token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := repository.Insert(ctx, expired); err != nil {
		t.Fatalf("Insert(expired) error = %v, want nil", err)
	}

	if err := repository.Insert(ctx, active); err != nil {
		t.Fatalf("Insert(active) error = %v, want nil", err)
	}

	if err := repository.DeleteExpired(ctx); err != nil {
		t.Fatalf("DeleteExpired() error = %v, want nil", err)
	}

	_, err := repository.ByTokenHash(ctx, expired.TokenHash)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ByTokenHash(%q) error = %v, want ErrSessionNotFound", expired.TokenHash, err)
	}

	got, err := repository.ByTokenHash(ctx, active.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash(%q) error = %v, want nil", active.TokenHash, err)
	}

	if got.TokenHash != active.TokenHash {
		t.Errorf("TokenHash = %q, want %q", got.TokenHash, active.TokenHash)
	}
}
