package user

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"uuid"
)

func TestStore_Insert(t *testing.T) {
	store := newTestRepository(t)

	want := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "hashed-password",
		Role:         RoleAdmin,
		Disabled:     false,
	}

	if err := store.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	got, err := store.ByID(want.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %v, want %v", got.ID, want.ID)
	}

	if got.Username != want.Username {
		t.Errorf("Username = %q, want %q", got.Username, want.Username)
	}

	if got.PasswordHash != want.PasswordHash {
		t.Errorf("PasswordHash = %q, want %q", got.PasswordHash, want.PasswordHash)
	}

	if got.Role != want.Role {
		t.Errorf("Role = %q, want %q", got.Role, want.Role)
	}

	if got.Disabled != want.Disabled {
		t.Errorf("Disabled = %v, want %v", got.Disabled, want.Disabled)
	}

	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestStore_InsertDuplicate(t *testing.T) {
	store := newTestRepository(t)

	first := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "first-hash",
		Role:         RoleAdmin,
	}

	second := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "second-hash",
		Role:         RoleViewer,
	}

	if err := store.Insert(first); err != nil {
		t.Fatalf("first Insert() error = %v, want nil", err)
	}

	err := store.Insert(second)

	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("second Insert() error = %v, want ErrUserExists", err)
	}
}

func TestStore_InsertConcurrentDuplicate(t *testing.T) {
	store := newTestRepository(t)

	const goroutines = 20

	var (
		wg         sync.WaitGroup
		successes  atomic.Int32
		duplicates atomic.Int32
	)

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			user := User{
				ID:           uuid.NewV7(),
				Username:     "admin",
				PasswordHash: "hashed-password",
				Role:         RoleAdmin,
			}

			err := store.Insert(user)

			switch {
			case err == nil:
				successes.Add(1)

			case errors.Is(err, ErrUserExists):
				duplicates.Add(1)

			default:
				t.Errorf("Insert() error = %v, want nil", err)
			}
		}()
	}

	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Errorf("successful inserts = %d, want 1", got)
	}

	if got := duplicates.Load(); got != goroutines-1 {
		t.Errorf("duplicate inserts = %d, want %d", got, goroutines-1)
	}
}
