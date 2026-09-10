package session

import (
	"os"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/database"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()

	databaseURL := os.Getenv("SIGNALDOCK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("SIGNALDOCK_TEST_DATABASE_URL is required")
	}

	ctx := t.Context()

	pool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("NewPostgresPool() error = %v, want nil", err)
	}

	t.Cleanup(pool.Close)

	if err := database.EnsureSchema(ctx, pool); err != nil {
		t.Fatalf("EnsureSchema() error = %v, want nil", err)
	}

	if _, err := pool.Exec(ctx, `TRUNCATE TABLE sessions, users`); err != nil {
		t.Fatalf("TRUNCATE sessions, users error = %v, want nil", err)
	}

	return NewRepository(pool)
}

func createTestUser(t *testing.T, repository *Repository) uuid.UUID {
	t.Helper()

	id := uuid.NewV7()

	const query = `
		INSERT INTO users (id, username, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`

	if _, err := repository.pool.Exec(t.Context(), query, id, "test-user", "hashed-password", "viewer"); err != nil {
		t.Fatalf("insert test user error = %v, want nil", err)
	}

	return id
}
