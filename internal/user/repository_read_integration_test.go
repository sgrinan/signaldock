package user

import (
	"errors"
	"testing"

	"uuid"
)

func TestRepository_List(t *testing.T) {
	repository := newTestRepository(t)

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
		if err := repository.Insert(user); err != nil {
			t.Fatalf("Insert() error = %v, want nil", err)
		}
	}

	got, err := repository.List()
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
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

func TestRepository_ByID(t *testing.T) {
	repository := newTestRepository(t)

	want := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "hashed-password",
		Role:         RoleAdmin,
		Disabled:     true,
	}

	if err := repository.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	got, err := repository.ByID(want.ID)
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
}

func TestRepository_ByIDNotFound(t *testing.T) {
	repository := newTestRepository(t)

	_, err := repository.ByID(uuid.NewV7())

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ByID() error = %v, want ErrUserNotFound", err)
	}
}

func TestRepository_ByUsername(t *testing.T) {
	repository := newTestRepository(t)

	want := User{
		ID:           uuid.NewV7(),
		Username:     "admin",
		PasswordHash: "hashed-password",
		Role:         RoleAdmin,
	}

	if err := repository.Insert(want); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	got, err := repository.ByUsername(want.Username)
	if err != nil {
		t.Fatalf("ByUsername() error = %v, want nil", err)
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

func TestRepository_ByUsernameNotFound(t *testing.T) {
	repository := newTestRepository(t)

	_, err := repository.ByUsername("missing")

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("ByUsername() error = %v, want ErrUserNotFound", err)
	}
}
