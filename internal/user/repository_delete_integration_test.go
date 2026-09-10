package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestRepository_RemoveByID(t *testing.T) {
	store := newTestRepository(t)

	account := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := store.Insert(account); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := store.RemoveByID(account.ID); err != nil {
		t.Fatalf("RemoveByID(%v) error = %v, want nil", account.ID, err)
	}

	_, err := store.ByID(account.ID)
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("ByID(%v) error = %v, want ErrUserNotFound", account.ID, err)
	}
}

func TestRepository_RemoveByIDNotFound(t *testing.T) {
	store := newTestRepository(t)
	id := uuid.NewV7()

	err := store.RemoveByID(id)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("RemoveByID(%v) error = %v, want ErrUserNotFound", id, err)
	}
}
