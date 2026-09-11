package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestRepository_UpdateAccess(t *testing.T) {
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

	if err := repository.UpdateAccess(ctx, account.ID, RoleAdmin, true); err != nil {
		t.Fatalf("UpdateAccess() error = %v, want nil", err)
	}

	got, err := repository.ByID(ctx, account.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	if got.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q", got.Role, RoleAdmin)
	}

	if !got.Disabled {
		t.Error("Disabled = false, want true")
	}
}

func TestRepository_UpdateAccessNotFound(t *testing.T) {
	ctx := t.Context()

	repository := newTestRepository(t)

	err := repository.UpdateAccess(ctx, uuid.NewV7(), RoleAdmin, true)

	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("UpdateAccess() error = %v, want ErrUserNotFound", err)
	}
}
