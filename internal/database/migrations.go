package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	const query = `
		CREATE TABLE IF NOT EXISTS endpoints (
			id UUID PRIMARY KEY,
			url TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if _, err := pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("create endpoints table: %w", err)
	}

	return nil
}
