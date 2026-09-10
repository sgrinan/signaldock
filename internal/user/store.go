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

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool: pool,
	}
}

func (s *Store) Insert(user User) error {
	const query = `
		INSERT INTO users (id, username, password_hash, role, disabled)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.pool.Exec(context.Background(), query, user.ID, user.Username, user.PasswordHash, user.Role, user.Disabled)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}

		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (s *Store) List() ([]User, error) {
	const query = `
		SELECT id::text, username, password_hash, role, disabled, created_at
		FROM users
		ORDER BY created_at, id
	`

	rows, err := s.pool.Query(context.Background(), query)
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

func (s *Store) ByUsername(username string) (User, error) {
	const query = `
		SELECT id::text, username, password_hash, role, disabled, created_at
		FROM users
		WHERE username = $1
	`

	var (
		user  User
		rawID string
	)

	err := s.pool.QueryRow(context.Background(), query, username).Scan(&rawID, &user.Username, &user.PasswordHash, &user.Role, &user.Disabled, &user.CreatedAt)

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

func (s *Store) ByID(id uuid.UUID) (User, error) {
	const query = `
		SELECT id::text, username, password_hash, role, disabled, created_at
		FROM users
		WHERE id = $1
	`

	var (
		user  User
		rawID string
	)

	err := s.pool.QueryRow(context.Background(), query, id).Scan(&rawID, &user.Username, &user.PasswordHash, &user.Role, &user.Disabled, &user.CreatedAt)

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

func (s *Store) SetDisabled(id uuid.UUID, disabled bool) error {
	const query = `
		UPDATE users
		SET disabled = $2
		WHERE id = $1
	`

	result, err := s.pool.Exec(context.Background(), query, id, disabled)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (s *Store) SetRole(id uuid.UUID, role Role) error {
	const query = `
		UPDATE users
		SET role = $2
		WHERE id = $1
	`

	result, err := s.pool.Exec(context.Background(), query, id, role)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}
