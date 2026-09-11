package user

import (
	"context"
	"errors"
	"fmt"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists users in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository returns a Repository backed by pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// Insert persists a user.
func (r *Repository) Insert(ctx context.Context, user User) error {
	const query = `
		INSERT INTO users (id, username, password_hash, role, disabled)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.pool.Exec(ctx, query, user.ID, user.Username, user.PasswordHash, user.Role, user.Disabled)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}

		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// List returns all persisted users.
func (r *Repository) List(ctx context.Context) ([]User, error) {
	const query = `
		SELECT id::text, username, password_hash, role, disabled, created_at
		FROM users
		ORDER BY created_at, id
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)

	for rows.Next() {
		var (
			user User
			id   string
		)

		if err := rows.Scan(&id, &user.Username, &user.PasswordHash, &user.Role, &user.Disabled, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		user.ID, err = uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("parse user id: %w", err)
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

// ByUsername returns the user with the given username.
func (r *Repository) ByUsername(ctx context.Context, username string) (User, error) {
	const query = `
		SELECT id::text, username, password_hash, role, disabled, created_at
		FROM users
		WHERE username = $1
	`

	var (
		user  User
		rawID string
	)

	err := r.pool.QueryRow(ctx, query, username).Scan(&rawID, &user.Username, &user.PasswordHash, &user.Role, &user.Disabled, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}

	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}

	user.ID, err = uuid.Parse(rawID)
	if err != nil {
		return User{}, fmt.Errorf("parse user id: %w", err)
	}

	return user, nil
}

// ByID returns the user with the given ID.
func (r *Repository) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	const query = `
		SELECT id::text, username, password_hash, role, disabled, created_at
		FROM users
		WHERE id = $1
	`

	var (
		user  User
		rawID string
	)

	err := r.pool.QueryRow(ctx, query, id).Scan(&rawID, &user.Username, &user.PasswordHash, &user.Role, &user.Disabled, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}

	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}

	user.ID, err = uuid.Parse(rawID)
	if err != nil {
		return User{}, fmt.Errorf("parse user id: %w", err)
	}

	return user, nil
}

// UpdateAccess updates whether a user account is disabled.
func (r *Repository) UpdateAccess(ctx context.Context, id uuid.UUID, role Role, disabled bool) error {
	const query = `
		UPDATE users
		SET role = $2,
			disabled = $3
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, role, disabled)
	if err != nil {
		return fmt.Errorf("update user access: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// RemoveByID deletes the user with the given ID.
func (r *Repository) RemoveByID(ctx context.Context, id uuid.UUID) error {
	const query = `
		DELETE FROM users
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}
