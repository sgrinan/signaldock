package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestRepository_SetDisabled(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	user := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := repository.Insert(ctx, user); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := repository.SetDisabled(ctx, user.ID, true); err != nil {
		t.Fatalf("SetDisabled() error = %v, want nil", err)
	}

	got, err := repository.ByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if !got.Disabled {
		t.Error("Disabled = false, want true")
	}

	if err := repository.SetDisabled(ctx, user.ID, false); err != nil {
		t.Fatalf("SetDisabled() error = %v, want nil", err)
	}

	got, err = repository.ByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if got.Disabled {
		t.Error("Disabled = true, want false")
	}
}

func TestRepository_SetDisabledNotFound(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	err := repository.SetDisabled(ctx, uuid.NewV7(), true)

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SetDisabled() error = %v, want ErrUserNotFound", err)
	}
}

func TestRepository_SetRole(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	account := User{
		ID:           uuid.NewV7(),
		Username:     "viewer",
		PasswordHash: "hashed-password",
		Role:         RoleViewer,
	}

	if err := repository.Insert(ctx, account); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	if err := repository.SetRole(ctx, account.ID, RoleAdmin); err != nil {
		t.Fatalf("SetRole(%v, %q) error = %v, want nil", account.ID, RoleAdmin, err)
	}

	got, err := repository.ByID(ctx, account.ID)
	if err != nil {
		t.Fatalf("ByID(%v) error = %v, want nil", account.ID, err)
	}

	if got.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q, want nil", got.Role, RoleAdmin)
	}
}

func TestRepository_SetRoleNotFound(t *testing.T) {
	ctx := t.Context()

	id := uuid.NewV7()
	repository := newTestRepository(t)

	err := repository.SetRole(ctx, id, RoleAdmin)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("SetRole(%v, %q) error = %v, want ErrUserNotFound", id, RoleAdmin, err)
	}
}
