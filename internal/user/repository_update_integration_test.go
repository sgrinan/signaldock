package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestStore_SetDisabled(t *testing.T) {
	store := newTestRepository(t)

	user := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := store.Insert(user); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := store.SetDisabled(user.ID, true); err != nil {
		t.Fatalf("SetDisabled() error = %v, want nil", err)
	}

	got, err := store.ByID(user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if !got.Disabled {
		t.Error("Disabled = false, want true")
	}

	if err := store.SetDisabled(user.ID, false); err != nil {
		t.Fatalf("SetDisabled() error = %v, want nil", err)
	}

	got, err = store.ByID(user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if got.Disabled {
		t.Error("Disabled = true, want false")
	}
}

func TestStore_SetDisabledNotFound(t *testing.T) {
	store := newTestRepository(t)

	err := store.SetDisabled(uuid.NewV7(), true)

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SetDisabled() error = %v, want ErrUserNotFound", err)
	}
}

func TestStore_SetRole(t *testing.T) {
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

	if err := store.SetRole(account.ID, RoleAdmin); err != nil {
		t.Fatalf("SetRole(%v, %q) error = %v, want nil", account.ID, RoleAdmin, err)
	}

	got, err := store.ByID(account.ID)
	if err != nil {
		t.Fatalf("ByID(%v) error = %v, want nil", account.ID, err)
	}

	if got.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q, want nil", got.Role, RoleAdmin)
	}
}

func TestStore_SetRoleNotFound(t *testing.T) {
	id := uuid.NewV7()
	store := newTestRepository(t)

	err := store.SetRole(id, RoleAdmin)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("SetRole(%v, %q) error = %v, want ErrUserNotFound", id, RoleAdmin, err)
	}
}
