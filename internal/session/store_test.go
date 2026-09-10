package session

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/database"
)

// session.Insert

func TestStore_Insert(t *testing.T) {
	store := newTestStore(t)
	userID := createTestUser(t, store)

	want := Session{
		TokenHash: "token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := store.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := store.ByTokenHash(want.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash(%q) error = %v", want.TokenHash, err)
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

func TestStore_InsertDuplicate(t *testing.T) {
	store := newTestStore(t)
	userID := createTestUser(t, store)

	session := Session{
		TokenHash: "duplicate-token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}

	if err := store.Insert(session); err != nil {
		t.Fatalf("first Insert() error = %v", err)
	}

	err := store.Insert(session)

	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("second Insert() error = %v, want ErrSessionExists", err)
	}
}

// session.ByTokenHash

func TestStore_ByTokenHash(t *testing.T) {
	store := newTestStore(t)
	userID := createTestUser(t, store)

	want := Session{
		TokenHash: "token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := store.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := store.ByTokenHash(want.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash(%q) error = %v", want.TokenHash, err)
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

func TestStore_ByTokenHashNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.ByTokenHash("missing-token-hash")

	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ByTokenHash(%q) error = %v, want ErrSessionNotFound", "missing-token-hash", err)
	}
}

// session.Delete

func TestStore_Delete(t *testing.T) {
	store := newTestStore(t)
	userID := createTestUser(t, store)

	session := Session{
		TokenHash: "token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := store.Insert(session); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if err := store.Delete(session.TokenHash); err != nil {
		t.Fatalf("Delete(%q) error = %v", session.TokenHash, err)
	}

	_, err := store.ByTokenHash(session.TokenHash)

	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ByTokenHash(%q) after Delete() error = %v, want ErrSessionNotFound", session.TokenHash, err)
	}
}

func TestStore_DeleteNotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.Delete("missing-token-hash")

	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Delete(%q) error = %v, want ErrSessionNotFound", "missing-token-hash", err)
	}
}

// session.DeleteExpired

func TestStore_DeleteExpired(t *testing.T) {
	store := newTestStore(t)
	userID := createTestUser(t, store)

	expired := Session{
		TokenHash: "expired-token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond),
	}

	active := Session{
		TokenHash: "active-token-hash",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond),
	}

	if err := store.Insert(expired); err != nil {
		t.Fatalf("Insert(expired) error = %v", err)
	}

	if err := store.Insert(active); err != nil {
		t.Fatalf("Insert(active) error = %v", err)
	}

	if err := store.DeleteExpired(); err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}

	_, err := store.ByTokenHash(expired.TokenHash)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("ByTokenHash(%q) error = %v, want ErrSessionNotFound", expired.TokenHash, err)
	}

	got, err := store.ByTokenHash(active.TokenHash)
	if err != nil {
		t.Fatalf("ByTokenHash(%q) error = %v", active.TokenHash, err)
	}

	if got.TokenHash != active.TokenHash {
		t.Errorf("TokenHash = %q, want %q", got.TokenHash, active.TokenHash)
	}
}

// helpers

func newTestStore(t *testing.T) *Store {
	t.Helper()

	databaseURL := os.Getenv("SIGNALDOCK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("SIGNALDOCK_TEST_DATABASE_URL is required")
	}

	ctx := context.Background()

	pool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresPool() error = %v", err)
	}

	t.Cleanup(pool.Close)

	if err := database.EnsureSchema(ctx, pool); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	if _, err := pool.Exec(ctx, `TRUNCATE TABLE sessions, users`); err != nil {
		t.Fatalf("TRUNCATE sessions, users error = %v", err)
	}

	return NewStore(pool)
}

func createTestUser(t *testing.T, store *Store) uuid.UUID {
	t.Helper()

	id := uuid.NewV7()

	const query = `
		INSERT INTO users (id, username, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`

	if _, err := store.pool.Exec(context.Background(), query, id, "test-user", "hashed-password", "viewer"); err != nil {
		t.Fatalf("insert test user error = %v", err)
	}

	return id
}
