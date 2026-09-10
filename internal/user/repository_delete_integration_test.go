package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestRepository_RemoveByID(t *testing.T) {
	repository := newTestRepository(t)

	account := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := repository.Insert(account); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := repository.RemoveByID(account.ID); err != nil {
		t.Fatalf("RemoveByID(%v) error = %v, want nil", account.ID, err)
	}

	_, err := repository.ByID(account.ID)
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("ByID(%v) error = %v, want ErrUserNotFound", account.ID, err)
	}
}

func TestRepository_RemoveByIDNotFound(t *testing.T) {
	repository := newTestRepository(t)
	id := uuid.NewV7()

	err := repository.RemoveByID(id)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("RemoveByID(%v) error = %v, want ErrUserNotFound", id, err)
	}
}
