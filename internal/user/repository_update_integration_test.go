package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestRepository_SetDisabled(t *testing.T) {
	repository := newTestRepository(t)

	user := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := repository.Insert(user); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := repository.SetDisabled(user.ID, true); err != nil {
		t.Fatalf("SetDisabled() error = %v, want nil", err)
	}

	got, err := repository.ByID(user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if !got.Disabled {
		t.Error("Disabled = false, want true")
	}

	if err := repository.SetDisabled(user.ID, false); err != nil {
		t.Fatalf("SetDisabled() error = %v, want nil", err)
	}

	got, err = repository.ByID(user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if got.Disabled {
		t.Error("Disabled = true, want false")
	}
}

func TestRepository_SetDisabledNotFound(t *testing.T) {
	repository := newTestRepository(t)

	err := repository.SetDisabled(uuid.NewV7(), true)

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SetDisabled() error = %v, want ErrUserNotFound", err)
	}
}

func TestRepository_SetRole(t *testing.T) {
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

	if err := repository.SetRole(account.ID, RoleAdmin); err != nil {
		t.Fatalf("SetRole(%v, %q) error = %v, want nil", account.ID, RoleAdmin, err)
	}

	got, err := repository.ByID(account.ID)
	if err != nil {
		t.Fatalf("ByID(%v) error = %v, want nil", account.ID, err)
	}

	if got.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q, want nil", got.Role, RoleAdmin)
	}
}

func TestRepository_SetRoleNotFound(t *testing.T) {
	id := uuid.NewV7()
	repository := newTestRepository(t)

	err := repository.SetRole(id, RoleAdmin)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("SetRole(%v, %q) error = %v, want ErrUserNotFound", id, RoleAdmin, err)
	}
}
