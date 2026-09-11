package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"uuid"

	"github.com/sgrinan/signaldock/internal/auth"
	"github.com/sgrinan/signaldock/internal/user"
)

func ensureInitialAdmin(repository *user.Repository, logger *slog.Logger) error {
	users, err := repository.List()
	if err != nil {
		return fmt.Errorf("load users error = %w, want nil", err)
	}

	if len(users) != 0 {
		return nil
	}

	username := os.Getenv("SIGNALDOCK_ADMIN_USERNAME")
	password := os.Getenv("SIGNALDOCK_ADMIN_PASSWORD")

	if username == "" || password == "" {
		return errors.New("initial admin credentials are required")
	}

	admin := user.User{
		ID:           uuid.NewV7(),
		Username:     username,
		PasswordHash: auth.HashPassword(password),
		Role:         user.RoleAdmin,
	}

	if err := repository.Insert(admin); err != nil {
		return fmt.Errorf("create initial admin error = %w, want nil", err)
	}

	logger.Info("initial admin created", "username", username)

	return nil
}
