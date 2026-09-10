package user

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/database"
)

// user.Insert

func TestStore_Insert(t *testing.T) {
	store := newTestStore(t)

	want := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "hashed-password",
		Role:         RoleAdmin,
		Disabled:     false,
	}

	if err := store.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := store.ByID(want.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
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
	store := newTestStore(t)

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
		t.Fatalf("first Insert() error = %v", err)
	}

	err := store.Insert(second)

	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("second Insert() error = %v, want ErrUserExists", err)
	}
}

func TestStore_InsertConcurrentDuplicate(t *testing.T) {
	store := newTestStore(t)

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
				t.Errorf("Insert() error = %v", err)
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

// user.List

func TestStore_List(t *testing.T) {
	store := newTestStore(t)

	users := []User{
		{
			ID:           uuid.NewV7(),
			Username:     "admin",
			PasswordHash: "admin-hash",
			Role:         RoleAdmin,
		},
		{
			ID:           uuid.NewV7(),
			Username:     "viewer",
			PasswordHash: "viewer-hash",
			Role:         RoleViewer,
		},
	}

	for _, user := range users {
		if err := store.Insert(user); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(got) != len(users) {
		t.Fatalf("List() returned %d users, want %d", len(got), len(users))
	}

	for i := range users {
		if got[i].ID != users[i].ID {
			t.Errorf("user[%d].ID = %v, want %v", i, got[i].ID, users[i].ID)
		}

		if got[i].Username != users[i].Username {
			t.Errorf("user[%d].Username = %q, want %q", i, got[i].Username, users[i].Username)
		}

		if got[i].PasswordHash != users[i].PasswordHash {
			t.Errorf("user[%d].PasswordHash = %q, want %q", i, got[i].PasswordHash, users[i].PasswordHash)
		}

		if got[i].Role != users[i].Role {
			t.Errorf("user[%d].Role = %q, want %q", i, got[i].Role, users[i].Role)
		}

		if got[i].Disabled != users[i].Disabled {
			t.Errorf("user[%d].Disabled = %v, want %v", i, got[i].Disabled, users[i].Disabled)
		}

		if got[i].CreatedAt.IsZero() {
			t.Errorf("user[%d].CreatedAt is zero", i)
		}
	}
}

// user.ByID

func TestStore_ByID(t *testing.T) {
	store := newTestStore(t)

	want := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "hashed-password",
		Role:         RoleAdmin,
		Disabled:     true,
	}

	if err := store.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := store.ByID(want.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
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
}

func TestStore_ByIDNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.ByID(uuid.NewV7())

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ByID() error = %v, want ErrUserNotFound", err)
	}
}

// user.ByUsername

func TestStore_ByUsername(t *testing.T) {
	store := newTestStore(t)

	want := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "hashed-password",
		Role:         RoleAdmin,
	}

	if err := store.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	got, err := store.ByUsername(want.Username)
	if err != nil {
		t.Fatalf("ByUsername() error = %v", err)
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
}

func TestStore_ByUsernameNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.ByUsername("missing")

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ByUsername() error = %v, want ErrUserNotFound", err)
	}
}

// user.SetDisabled

func TestStore_SetDisabled(t *testing.T) {
	store := newTestStore(t)

	user := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := store.Insert(user); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if err := store.SetDisabled(user.ID, true); err != nil {
		t.Fatalf("SetDisabled() error = %v", err)
	}

	got, err := store.ByID(user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}

	if !got.Disabled {
		t.Error("Disabled = false, want true")
	}

	if err := store.SetDisabled(user.ID, false); err != nil {
		t.Fatalf("SetDisabled() error = %v", err)
	}

	got, err = store.ByID(user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}

	if got.Disabled {
		t.Error("Disabled = true, want false")
	}
}

func TestStore_SetDisabledNotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.SetDisabled(uuid.NewV7(), true)

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SetDisabled() error = %v, want ErrUserNotFound", err)
	}
}

func TestStore_SetRole(t *testing.T) {
	store := newTestStore(t)

	account := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := store.Insert(account); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if err := store.SetRole(account.ID, RoleAdmin); err != nil {
		t.Fatalf("SetRole(%v, %q) error = %v", account.ID, RoleAdmin, err)
	}

	got, err := store.ByID(account.ID)
	if err != nil {
		t.Fatalf("ByID(%v) error = %v", account.ID, err)
	}

	if got.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q", got.Role, RoleAdmin)
	}
}

func TestStore_SetRoleNotFound(t *testing.T) {
	id := uuid.NewV7()
	store := newTestStore(t)

	err := store.SetRole(id, RoleAdmin)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("SetRole(%v, %q) error = %v, want ErrUserNotFound", id, RoleAdmin, err)
	}
}

// helper

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

	if err := database.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	if _, err := pool.Exec(ctx, `TRUNCATE TABLE sessions, users`); err != nil {
		t.Fatalf("TRUNCATE sessions, users error = %v", err)
	}

	return NewStore(pool)
}
